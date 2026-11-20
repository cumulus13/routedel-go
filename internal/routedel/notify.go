package routedel

// File: internal/routedel/notify.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: ntfy (HTTP) and Growl (GNTP 1.0) notifications. Multiple servers, sent in parallel.
// License: MIT

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type NotifyTarget struct {
	Kind  string
	Ntfy  NtfyConfig
	Growl GrowlConfig
}

type Notifier struct {
	Targets []NotifyTarget
}

func getHostname() string {
	h, err := os.Hostname()
	if err != nil {
		return ""
	}
	return h
}

func toASCIIReplace(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func ntfySend(cfg NtfyConfig, title string, message string, level string) error {
	u := strings.TrimRight(cfg.URL, "/")
	if cfg.Topic != "" {
		u = fmt.Sprintf("%s/%s", u, cfg.Topic)
	}
	priorityMap := map[string]string{
		"info":    "3",
		"warning": "4",
		"error":   "5",
	}
	priority, ok := priorityMap[level]
	if !ok {
		priority = "3"
	}
	tagsMap := map[string]string{
		"info":    "information_source",
		"warning": "warning",
		"error":   "rotating_light",
	}
	tags := tagsMap[level]

	timeoutSec := 5.0
	if cfg.Timeout != nil {
		timeoutSec = *cfg.Timeout
	}
	client := &http.Client{
		Timeout: time.Duration(timeoutSec * float64(time.Second)),
	}
	req, err := http.NewRequest(http.MethodPost, u, bytes.NewBufferString(message))
	if err != nil {
		return err
	}
	req.Header.Set("Title", toASCIIReplace(title))
	req.Header.Set("Priority", priority)
	if tags != "" {
		req.Header.Set("Tags", tags)
	}
	if cfg.Token != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.Token)
	} else if cfg.User != "" {
		cred := base64.StdEncoding.EncodeToString([]byte(cfg.User + ":" + cfg.Password))
		req.Header.Set("Authorization", "Basic "+cred)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}

func gntpHeader(msgType string, password string) (string, error) {
	if password == "" {
		return fmt.Sprintf("GNTP/1.0 %s NONE\r\n", msgType), nil
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	passBytes := []byte(password)
	combined := make([]byte, 0, len(passBytes)+len(salt))
	combined = append(combined, passBytes...)
	combined = append(combined, salt...)
	keySum := sha256.Sum256(combined)
	hashSum := sha256.Sum256(keySum[:])
	keyHash := strings.ToUpper(hex.EncodeToString(hashSum[:]))
	saltHex := strings.ToUpper(hex.EncodeToString(salt))
	return fmt.Sprintf("GNTP/1.0 %s NONE SHA256:%s.%s\r\n", msgType, keyHash, saltHex), nil
}

func gntpRoundtrip(host string, port int, payload string, timeout time.Duration) error {
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if _, err := conn.Write([]byte(payload)); err != nil {
		return err
	}
	var buf bytes.Buffer
	tmp := make([]byte, 4096)
	for !bytes.Contains(buf.Bytes(), []byte("\r\n\r\n")) {
		n, rErr := conn.Read(tmp)
		if n > 0 {
			buf.Write(tmp[:n])
		}
		if rErr != nil {
			break
		}
	}
	text := buf.String()
	firstLine := strings.SplitN(text, "\r\n", 2)[0]
	if !strings.Contains(firstLine, "-OK") {
		cleaned := strings.TrimSpace(strings.ReplaceAll(text, "\r\n", " | "))
		if cleaned == "" {
			cleaned = "empty response"
		}
		return fmt.Errorf("%s", cleaned)
	}
	return nil
}

func gntpSingleLine(s string) string {
	var b strings.Builder
	prevBreak := false
	for _, r := range s {
		if r == '\r' || r == '\n' {
			if !prevBreak {
				b.WriteString("; ")
				prevBreak = true
			}
		} else {
			b.WriteRune(r)
			prevBreak = false
		}
	}
	return b.String()
}

func gntpSend(cfg GrowlConfig, title string, message string, level string) error {
	host := cfg.Host
	port := cfg.Port
	if port == 0 {
		port = DefaultGNTPPort
	}
	pw := cfg.Password
	timeoutSec := 5.0
	if cfg.Timeout != nil {
		timeoutSec = *cfg.Timeout
	}
	timeout := time.Duration(timeoutSec * float64(time.Second))

	regHdr, err := gntpHeader("REGISTER", pw)
	if err != nil {
		return err
	}
	reg := regHdr +
		fmt.Sprintf("Application-Name: %s\r\nNotifications-Count: 1\r\n\r\n", AppName) +
		"Notification-Name: route-event\r\nNotification-Display-Name: Route Event\r\n" +
		"Notification-Enabled: True\r\n\r\n"
	if err := gntpRoundtrip(host, port, reg, timeout); err != nil {
		return err
	}

	prioMap := map[string]int{
		"info":    0,
		"warning": 1,
		"error":   2,
	}
	prio, ok := prioMap[level]
	if !ok {
		prio = 0
	}
	noteHdr, err := gntpHeader("NOTIFY", pw)
	if err != nil {
		return err
	}
	note := noteHdr +
		fmt.Sprintf("Application-Name: %s\r\nNotification-Name: route-event\r\n", AppName) +
		fmt.Sprintf("Notification-Title: %s\r\nNotification-Text: %s\r\n", gntpSingleLine(title), gntpSingleLine(message)) +
		fmt.Sprintf("Notification-Priority: %d\r\n\r\n", prio)
	return gntpRoundtrip(host, port, note, timeout)
}

func (n *Notifier) sendOne(target NotifyTarget, title string, msg string, level string) bool {
	var label string
	var err error
	if target.Kind == "ntfy" {
		label = target.Ntfy.URL
		err = ntfySend(target.Ntfy, title, msg, level)
	} else {
		port := target.Growl.Port
		if port == 0 {
			port = DefaultGNTPPort
		}
		label = fmt.Sprintf("%s:%d", target.Growl.Host, port)
		err = gntpSend(target.Growl, title, msg, level)
	}
	if err == nil {
		logInfo("[%s] sent -> %s", target.Kind, label)
		return true
	}
	logError("[%s] failed -> %s: %s", target.Kind, label, err.Error())
	return false
}

func (n *Notifier) Send(message string, level string, wait bool, title string) int {
	if n == nil || len(n.Targets) == 0 {
		return 0
	}
	if title == "" {
		title = fmt.Sprintf("%s@%s", AppName, getHostname())
	}
	var mu sync.Mutex
	completed := 0
	failedCount := 0
	var wg sync.WaitGroup

	for _, t := range n.Targets {
		wg.Add(1)
		go func(tgt NotifyTarget) {
			defer wg.Done()
			ok := n.sendOne(tgt, title, message, level)
			mu.Lock()
			completed++
			if !ok {
				failedCount++
			}
			mu.Unlock()
		}(t)
	}

	if !wait {
		return 0
	}

	doneCh := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case <-doneCh:
		return failedCount
	case <-time.After(15 * time.Second):
		mu.Lock()
		unfinished := len(n.Targets) - completed
		totalFailed := failedCount + unfinished
		mu.Unlock()
		return totalFailed
	}
}

func BuildNotifier(noNotify bool, configPath string, ntfyURLs []string, growlHosts []string) (*Notifier, error) {
	if noNotify {
		return &Notifier{Targets: []NotifyTarget{}}, nil
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	var targets []NotifyTarget
	for _, c := range cfg.Ntfy {
		targets = append(targets, NotifyTarget{Kind: "ntfy", Ntfy: c})
	}
	for _, c := range cfg.Growl {
		targets = append(targets, NotifyTarget{Kind: "growl", Growl: c})
	}
	for _, u := range ntfyURLs {
		base, topic, err := SplitNtfyURL(u)
		if err != nil {
			return nil, err
		}
		if topic == "" {
			return nil, fmt.Errorf("--ntfy needs a topic: https://host/TOPIC (got %q)", u)
		}
		targets = append(targets, NotifyTarget{
			Kind: "ntfy",
			Ntfy: NtfyConfig{URL: base, Topic: topic},
		})
	}
	for _, g := range growlHosts {
		host, port, err := ParseGrowl(g)
		if err != nil {
			return nil, err
		}
		targets = append(targets, NotifyTarget{
			Kind:  "growl",
			Growl: GrowlConfig{Host: host, Port: port},
		})
	}
	return &Notifier{Targets: targets}, nil
}
