package routedel

// File: internal/routedel/config.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: Platform-aware config file handling and the `routedel config ...` subcommands.
// License: MIT

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	DefaultGNTPPort = 23053
	MaskValue       = "********"
)

type NtfyConfig struct {
	URL      string   `json:"url"`
	Topic    string   `json:"topic,omitempty"`
	Token    string   `json:"token,omitempty"`
	User     string   `json:"user,omitempty"`
	Password string   `json:"password,omitempty"`
	Timeout  *float64 `json:"timeout,omitempty"`
}

type GrowlConfig struct {
	Host     string   `json:"host"`
	Port     int      `json:"port,omitempty"`
	Password string   `json:"password,omitempty"`
	Timeout  *float64 `json:"timeout,omitempty"`
}

type ConfigData struct {
	Ntfy  []NtfyConfig  `json:"ntfy"`
	Growl []GrowlConfig `json:"growl"`
}

type ConfigCandidate struct {
	Label string
	Path  string
}

type ConfigIssue struct {
	Level   string
	Message string
}

var allowedTopKeys = map[string]bool{
	"ntfy":  true,
	"growl": true,
}

var allowedNtfyKeys = map[string]bool{
	"url":      true,
	"topic":    true,
	"token":    true,
	"user":     true,
	"password": true,
	"timeout":  true,
}

var allowedGrowlKeys = map[string]bool{
	"host":     true,
	"port":     true,
	"password": true,
	"timeout":  true,
}

func emptyConfig() ConfigData {
	return ConfigData{
		Ntfy:  []NtfyConfig{},
		Growl: []GrowlConfig{},
	}
}

func exampleConfig() ConfigData {
	port := DefaultGNTPPort
	return ConfigData{
		Ntfy: []NtfyConfig{
			{URL: "https://ntfy.sh", Topic: "my-routes", Token: "tk_xxx"},
		},
		Growl: []GrowlConfig{
			{Host: "192.168.1.10", Port: port, Password: "secret"},
		},
	}
}

func expandHome(p string) string {
	if p == "~" {
		h, err := os.UserHomeDir()
		if err == nil {
			return h
		}
		return p
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		h, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(h, p[2:])
		}
	}
	return p
}

// DefaultConfigPath returns the platform-native user config file path:
//   - Windows: %APPDATA%\routedel\config.json (fallback: ~\AppData\Roaming\routedel\config.json)
//   - macOS:   ~/Library/Application Support/routedel/config.json (fallback if XDG_CONFIG_HOME is set)
//   - Linux/BSD: $XDG_CONFIG_HOME/routedel/config.json or ~/.config/routedel/config.json
func DefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			return filepath.Join(appData, AppName, "config.json")
		}
		if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
			return filepath.Join(cfgDir, AppName, "config.json")
		}
		return filepath.Join(home, "AppData", "Roaming", AppName, "config.json")
	case "darwin":
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, AppName, "config.json")
		}
		if cfgDir, err := os.UserConfigDir(); err == nil && cfgDir != "" {
			return filepath.Join(cfgDir, AppName, "config.json")
		}
		return filepath.Join(home, "Library", "Application Support", AppName, "config.json")
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, AppName, "config.json")
		}
		return filepath.Join(home, ".config", AppName, "config.json")
	}
}

// SystemConfigPath returns the platform-native system-wide config file path.
func SystemConfigPath() string {
	switch runtime.GOOS {
	case "windows":
		if progData := os.Getenv("ProgramData"); progData != "" {
			return filepath.Join(progData, AppName, "config.json")
		}
		return filepath.Join(`C:\ProgramData`, AppName, "config.json")
	case "darwin":
		return filepath.Join("/Library", "Application Support", AppName, "config.json")
	default:
		return filepath.Join("/etc", AppName, "config.json")
	}
}

// ConfigCandidates returns the platform-aware lookup order; the first existing file wins.
func ConfigCandidates(explicit string) []ConfigCandidate {
	var out []ConfigCandidate
	if explicit != "" {
		out = append(out, ConfigCandidate{Label: "--config", Path: expandHome(explicit)})
	}
	if env := os.Getenv("ROUTEDEL_CONFIG"); env != "" {
		out = append(out, ConfigCandidate{Label: "$ROUTEDEL_CONFIG", Path: expandHome(env)})
	}
	defPath := DefaultConfigPath()
	out = append(out, ConfigCandidate{Label: "user (" + runtime.GOOS + ")", Path: defPath})

	home, _ := os.UserHomeDir()
	// On Windows/macOS, also check ~/.config/routedel/config.json if distinct from DefaultConfigPath
	xdgFallback := filepath.Join(home, ".config", AppName, "config.json")
	if xdgFallback != defPath {
		out = append(out, ConfigCandidate{Label: "xdg-user", Path: xdgFallback})
	}
	out = append(out, ConfigCandidate{Label: "legacy", Path: filepath.Join(home, ".routedel.json")})
	out = append(out, ConfigCandidate{Label: "system (" + runtime.GOOS + ")", Path: SystemConfigPath()})
	return out
}

func isRegularFile(p string) bool {
	info, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func ActiveConfigPath(explicit string) string {
	for _, c := range ConfigCandidates(explicit) {
		if isRegularFile(c.Path) {
			return c.Path
		}
	}
	return ""
}

func TargetConfigPath(explicit string) string {
	if explicit != "" {
		return expandHome(explicit)
	}
	if env := os.Getenv("ROUTEDEL_CONFIG"); env != "" {
		return expandHome(env)
	}
	if act := ActiveConfigPath(explicit); act != "" {
		return act
	}
	return DefaultConfigPath()
}

func ReadConfigFile(p string) (ConfigData, error) {
	raw, err := os.ReadFile(p)
	if err != nil {
		return ConfigData{}, fmt.Errorf("Bad config %s: %w", p, err)
	}
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil {
		return ConfigData{}, fmt.Errorf("Bad config %s: %w", p, err)
	}
	if top == nil {
		return ConfigData{}, fmt.Errorf("Bad config %s: top level must be a JSON object", p)
	}
	var cfg ConfigData
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return ConfigData{}, fmt.Errorf("Bad config %s: %w", p, err)
	}
	if cfg.Ntfy == nil {
		cfg.Ntfy = []NtfyConfig{}
	}
	if cfg.Growl == nil {
		cfg.Growl = []GrowlConfig{}
	}
	return cfg, nil
}

func LoadConfig(explicit string) (ConfigData, error) {
	if explicit != "" {
		exp := expandHome(explicit)
		if !isRegularFile(exp) {
			return ConfigData{}, fmt.Errorf("Config file not found: %s", explicit)
		}
	}
	p := ActiveConfigPath(explicit)
	if p == "" {
		return emptyConfig(), nil
	}
	return ReadConfigFile(p)
}

func SaveConfig(p string, data ConfigData) error {
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	encoded = append(encoded, '\n')
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, encoded, 0o600); err != nil {
		return err
	}
	_ = os.Chmod(tmp, 0o600)
	return os.Rename(tmp, p)
}

func SplitNtfyURL(u string) (string, string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(u), "/")
	sp, err := url.Parse(trimmed)
	if err != nil || (sp.Scheme != "http" && sp.Scheme != "https") || sp.Host == "" {
		return "", "", fmt.Errorf("invalid ntfy URL: %q", u)
	}
	path := strings.TrimRight(sp.Path, "/")
	if path == "" {
		return fmt.Sprintf("%s://%s", sp.Scheme, sp.Host), "", nil
	}
	lastSlash := strings.LastIndex(path, "/")
	var basePath, topic string
	if lastSlash < 0 {
		basePath = ""
		topic = path
	} else {
		basePath = path[:lastSlash]
		topic = path[lastSlash+1:]
	}
	return fmt.Sprintf("%s://%s%s", sp.Scheme, sp.Host, basePath), topic, nil
}

func ParseGrowl(s string) (string, int, error) {
	trimmed := strings.TrimSpace(s)
	lastColon := strings.LastIndex(trimmed, ":")
	if lastColon < 0 {
		if trimmed == "" {
			return "", 0, errors.New("empty Growl host")
		}
		return trimmed, DefaultGNTPPort, nil
	}
	host := trimmed[:lastColon]
	portStr := trimmed[lastColon+1:]
	if host == "" || portStr == "" {
		return "", 0, fmt.Errorf("invalid Growl host[:port]: %q", s)
	}
	for _, ch := range portStr {
		if ch < '0' || ch > '9' {
			return "", 0, fmt.Errorf("invalid Growl host[:port]: %q", s)
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port >= 65536 {
		return "", 0, fmt.Errorf("invalid Growl host[:port]: %q", s)
	}
	return host, port, nil
}

func MaskConfig(data ConfigData) ConfigData {
	out := ConfigData{
		Ntfy:  make([]NtfyConfig, len(data.Ntfy)),
		Growl: make([]GrowlConfig, len(data.Growl)),
	}
	for i, c := range data.Ntfy {
		if c.Token != "" {
			c.Token = MaskValue
		}
		if c.Password != "" {
			c.Password = MaskValue
		}
		out.Ntfy[i] = c
	}
	for i, c := range data.Growl {
		if c.Password != "" {
			c.Password = MaskValue
		}
		out.Growl[i] = c
	}
	return out
}

func DescribeNtfy(c NtfyConfig) string {
	base := strings.TrimRight(c.URL, "/")
	if c.Topic != "" {
		return base + "/" + c.Topic
	}
	return c.URL
}

func DescribeGrowl(c GrowlConfig) string {
	port := c.Port
	if port == 0 {
		port = DefaultGNTPPort
	}
	return fmt.Sprintf("%s:%d", c.Host, port)
}

func AuthOfNtfy(c NtfyConfig) string {
	if c.Token != "" {
		return "token"
	}
	if c.User != "" {
		return "basic"
	}
	return "none"
}

func AuthOfGrowl(c GrowlConfig) string {
	if c.Password != "" {
		return "password"
	}
	return "none"
}

func ValidateConfigRaw(raw []byte) []ConfigIssue {
	var top map[string]any
	if err := json.Unmarshal(raw, &top); err != nil || top == nil {
		return []ConfigIssue{{Level: "error", Message: "top level must be a JSON object"}}
	}
	var issues []ConfigIssue
	for k := range top {
		if !allowedTopKeys[k] {
			issues = append(issues, ConfigIssue{Level: "warning", Message: fmt.Sprintf("unknown key '%s'", k)})
		}
	}

	if ntfyRaw, exists := top["ntfy"]; exists {
		items, ok := ntfyRaw.([]any)
		if !ok {
			issues = append(issues, ConfigIssue{Level: "error", Message: "'ntfy' must be a list"})
		} else {
			for i, item := range items {
				w := fmt.Sprintf("ntfy[%d]", i+1)
				c, ok := item.(map[string]any)
				if !ok {
					issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: must be an object", w)})
					continue
				}
				v, hasURL := c["url"]
				vStr, isStr := v.(string)
				if !hasURL || !isStr || strings.TrimSpace(vStr) == "" {
					issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: missing 'url'", w)})
				} else if !strings.HasPrefix(vStr, "http://") && !strings.HasPrefix(vStr, "https://") {
					issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: url must start with http:// or https://", w)})
				}
				for k := range c {
					if !allowedNtfyKeys[k] {
						issues = append(issues, ConfigIssue{Level: "warning", Message: fmt.Sprintf("%s: unknown key '%s'", w, k)})
					}
				}
				for _, k := range []string{"topic", "token", "user", "password"} {
					if val, present := c[k]; present {
						if _, ok := val.(string); !ok {
							issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: '%s' must be a string", w, k)})
						}
					}
				}
				if tVal, present := c["timeout"]; present {
					num, ok := tVal.(float64)
					if !ok || num <= 0 {
						issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: 'timeout' must be a number > 0", w)})
					}
				}
				tok, _ := c["token"].(string)
				usr, _ := c["user"].(string)
				if tok != "" && usr != "" {
					issues = append(issues, ConfigIssue{Level: "warning", Message: fmt.Sprintf("%s: both token and user set; token is used", w)})
				}
			}
		}
	}

	if growlRaw, exists := top["growl"]; exists {
		items, ok := growlRaw.([]any)
		if !ok {
			issues = append(issues, ConfigIssue{Level: "error", Message: "'growl' must be a list"})
		} else {
			for i, item := range items {
				w := fmt.Sprintf("growl[%d]", i+1)
				c, ok := item.(map[string]any)
				if !ok {
					issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: must be an object", w)})
					continue
				}
				v, hasHost := c["host"]
				vStr, isStr := v.(string)
				if !hasHost || !isStr || strings.TrimSpace(vStr) == "" {
					issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: missing 'host'", w)})
				}
				for k := range c {
					if !allowedGrowlKeys[k] {
						issues = append(issues, ConfigIssue{Level: "warning", Message: fmt.Sprintf("%s: unknown key '%s'", w, k)})
					}
				}
				if pw, present := c["password"]; present {
					if _, ok := pw.(string); !ok {
						issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: 'password' must be a string", w)})
					}
				}
				if pVal, present := c["port"]; present {
					_, isBool := pVal.(bool)
					num, isNum := pVal.(float64)
					if isBool || !isNum || num <= 0 || num >= 65536 || float64(int(num)) != num {
						issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: 'port' must be an integer 1-65535", w)})
					}
				}
				if tVal, present := c["timeout"]; present {
					num, ok := tVal.(float64)
					if !ok || num <= 0 {
						issues = append(issues, ConfigIssue{Level: "error", Message: fmt.Sprintf("%s: 'timeout' must be a number > 0", w)})
					}
				}
			}
		}
	}

	return issues
}

func needConfigFile(explicit string) string {
	if explicit != "" {
		exp := expandHome(explicit)
		if !isRegularFile(exp) {
			logError("Config file not found: %s", explicit)
			return ""
		}
	}
	p := ActiveConfigPath(explicit)
	if p == "" {
		logError("No config file found. Create one with: routedel config init")
	}
	return p
}

func cmdConfigPath(args []string, explicit string) int {
	fs := flag.NewFlagSet("path", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	all := fs.Bool("all", false, "Show the full lookup order")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *all {
		act := ActiveConfigPath(explicit)
		fmt.Println(styleStdout(ansiBoldMagenta, fmt.Sprintf("Config lookup order (platform: %s/%s)", runtime.GOOS, runtime.GOARCH)))
		fmt.Println(styleStdout(ansiBoldWhite, fmt.Sprintf("%-4s %-18s %-50s %-8s %-6s", "#", "Source", "Path", "Exists", "Active")))
		for i, c := range ConfigCandidates(explicit) {
			exists := styleStdout(ansiDim, "no")
			if isRegularFile(c.Path) {
				exists = styleStdout(ansiBoldGreen, "yes")
			}
			activeMark := ""
			rowColor := ansiWhite
			if c.Path == act && act != "" {
				activeMark = styleStdout(ansiBoldCyan, "<-")
				rowColor = ansiBoldCyan
			}
			fmt.Printf("%s %s %s %s %s\n",
				styleStdout(ansiCyan, fmt.Sprintf("%-4d", i+1)),
				styleStdout(rowColor, fmt.Sprintf("%-18s", c.Label)),
				styleStdout(rowColor, fmt.Sprintf("%-50s", c.Path)),
				exists,
				activeMark,
			)
		}
		return 0
	}
	p := ActiveConfigPath(explicit)
	if p == "" {
		p = TargetConfigPath(explicit)
	}
	fmt.Println(styleStdout(ansiBoldCyan, p))
	if !isRegularFile(p) {
		fmt.Fprintln(os.Stderr, styleStderr(ansiDim, "(not found; create it with: routedel config init)"))
	}
	return 0
}

func cmdConfigShow(args []string, explicit string) int {
	fs := flag.NewFlagSet("show", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	asJSON := fs.Bool("json", false, "Print the JSON instead of a table")
	reveal := fs.Bool("reveal", false, "With --json: do not mask tokens/passwords")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	p := needConfigFile(explicit)
	if p == "" {
		return 1
	}
	data, err := ReadConfigFile(p)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}
	if *asJSON {
		outData := data
		if !*reveal {
			outData = MaskConfig(data)
		}
		b, err := json.MarshalIndent(outData, "", "  ")
		if err != nil {
			logError("%s", err.Error())
			return 1
		}
		fmt.Println(string(b))
		return 0
	}
	fmt.Printf("%s %s (%s)\n", styleStdout(ansiBoldWhite, "Config:"), styleStdout(ansiCyan, p), runtime.GOOS)
	fmt.Println(styleStdout(ansiBoldMagenta, "Notification targets"))
	fmt.Printf("%s %s %s %s %s\n",
		styleStdout(ansiBoldCyan, fmt.Sprintf("%-8s", "Kind")),
		styleStdout(ansiCyan, fmt.Sprintf("%-4s", "#")),
		styleStdout(ansiBoldYellow, fmt.Sprintf("%-35s", "Target")),
		styleStdout(ansiGreen, fmt.Sprintf("%-10s", "Auth")),
		styleStdout(ansiWhite, fmt.Sprintf("%-8s", "Timeout")),
	)
	n := 0
	for i, c := range data.Ntfy {
		timeoutStr := "5"
		if c.Timeout != nil {
			timeoutStr = strconv.FormatFloat(*c.Timeout, 'f', -1, 64)
		}
		fmt.Printf("%s %s %s %s %s\n",
			styleStdout(ansiBoldCyan, fmt.Sprintf("%-8s", "ntfy")),
			styleStdout(ansiCyan, fmt.Sprintf("%-4d", i+1)),
			styleStdout(ansiBoldYellow, fmt.Sprintf("%-35s", DescribeNtfy(c))),
			styleStdout(ansiGreen, fmt.Sprintf("%-10s", AuthOfNtfy(c))),
			styleStdout(ansiWhite, fmt.Sprintf("%-8s", timeoutStr)),
		)
		n++
	}
	for i, c := range data.Growl {
		timeoutStr := "5"
		if c.Timeout != nil {
			timeoutStr = strconv.FormatFloat(*c.Timeout, 'f', -1, 64)
		}
		fmt.Printf("%s %s %s %s %s\n",
			styleStdout(ansiBoldCyan, fmt.Sprintf("%-8s", "growl")),
			styleStdout(ansiCyan, fmt.Sprintf("%-4d", i+1)),
			styleStdout(ansiBoldYellow, fmt.Sprintf("%-35s", DescribeGrowl(c))),
			styleStdout(ansiGreen, fmt.Sprintf("%-10s", AuthOfGrowl(c))),
			styleStdout(ansiWhite, fmt.Sprintf("%-8s", timeoutStr)),
		)
		n++
	}
	if n == 0 {
		fmt.Println(styleStdout(ansiDim, "No targets configured."))
	}
	raw, _ := os.ReadFile(p)
	errs := 0
	for _, iss := range ValidateConfigRaw(raw) {
		if iss.Level == "error" {
			errs++
		}
	}
	if errs > 0 {
		fmt.Printf("%s - run: routedel config validate\n",
			styleStdout(ansiBoldRed, fmt.Sprintf("%d problem(s)", errs)))
	}
	return 0
}

func cmdConfigInit(args []string, explicit string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	force := fs.Bool("force", false, "Overwrite an existing file")
	example := fs.Bool("example", false, "Write example entries instead of empty lists")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	p := TargetConfigPath(explicit)
	if _, err := os.Stat(p); err == nil && !*force {
		logError("%s already exists (use --force to overwrite)", p)
		return 1
	}
	var data ConfigData
	if *example {
		data = exampleConfig()
	} else {
		data = emptyConfig()
	}
	if err := SaveConfig(p, data); err != nil {
		logError("%s", err.Error())
		return 1
	}
	logInfo("Created %s", p)
	fmt.Printf("Next: %s  |  %s\n",
		styleStdout(ansiBoldWhite, "routedel config add ntfy https://ntfy.sh/TOPIC"),
		styleStdout(ansiBoldWhite, "routedel config add growl HOST[:PORT]"),
	)
	return 0
}

func cmdConfigAdd(args []string, explicit string) int {
	fs := flag.NewFlagSet("add", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	topicFlag := fs.String("topic", "", "ntfy topic (if not part of the URL)")
	tokenFlag := fs.String("token", "", "ntfy bearer token")
	userFlag := fs.String("user", "", "ntfy basic-auth user")
	passwordFlag := fs.String("password", "", "ntfy basic-auth password / Growl password")
	portFlag := fs.Int("port", 0, "Growl port (default 23053)")
	timeoutFlag := fs.Float64("timeout", 0, "Per-target timeout in seconds (default 5)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	pos := fs.Args()
	if len(pos) < 2 {
		logError("add requires KIND and TARGET")
		return 1
	}
	kind := pos[0]
	target := pos[1]
	if kind != "ntfy" && kind != "growl" {
		logError("invalid kind %q: must be ntfy or growl", kind)
		return 1
	}

	p := TargetConfigPath(explicit)
	var data ConfigData
	if isRegularFile(p) {
		var err error
		data, err = ReadConfigFile(p)
		if err != nil {
			logError("%s", err.Error())
			return 1
		}
	} else {
		data = emptyConfig()
	}

	var timeoutPtr *float64
	if *timeoutFlag != 0 {
		t := *timeoutFlag
		timeoutPtr = &t
	}

	if kind == "ntfy" {
		base, urlTopic, err := SplitNtfyURL(target)
		if err != nil {
			logError("%s", err.Error())
			return 1
		}
		topic := *topicFlag
		if topic == "" {
			topic = urlTopic
		}
		if topic == "" {
			logError("ntfy needs a topic: https://host/TOPIC or --topic TOPIC")
			return 1
		}
		entry := NtfyConfig{
			URL:      base,
			Topic:    topic,
			Token:    *tokenFlag,
			User:     *userFlag,
			Password: *passwordFlag,
			Timeout:  timeoutPtr,
		}
		desc := DescribeNtfy(entry)
		for _, c := range data.Ntfy {
			if DescribeNtfy(c) == desc {
				logError("Already configured: %s", desc)
				return 1
			}
		}
		data.Ntfy = append(data.Ntfy, entry)
		if err := SaveConfig(p, data); err != nil {
			logError("%s", err.Error())
			return 1
		}
		logInfo("Added ntfy target %s -> %s", desc, p)
		return 0
	}

	host, parsedPort, err := ParseGrowl(target)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}
	port := *portFlag
	if port == 0 {
		port = parsedPort
	}
	entry := GrowlConfig{
		Host:     host,
		Password: *passwordFlag,
		Timeout:  timeoutPtr,
	}
	if port != DefaultGNTPPort {
		entry.Port = port
	}
	desc := DescribeGrowl(entry)
	for _, c := range data.Growl {
		if DescribeGrowl(c) == desc {
			logError("Already configured: %s", desc)
			return 1
		}
	}
	data.Growl = append(data.Growl, entry)
	if err := SaveConfig(p, data); err != nil {
		logError("%s", err.Error())
		return 1
	}
	logInfo("Added growl target %s -> %s", desc, p)
	return 0
}

func cmdConfigRemove(args []string, explicit string) int {
	fs := flag.NewFlagSet("remove", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	pos := fs.Args()
	if len(pos) < 2 {
		logError("remove requires KIND and TARGET")
		return 1
	}
	kind := pos[0]
	target := pos[1]
	if kind != "ntfy" && kind != "growl" {
		logError("invalid kind %q: must be ntfy or growl", kind)
		return 1
	}

	p := needConfigFile(explicit)
	if p == "" {
		return 1
	}
	data, err := ReadConfigFile(p)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}

	if kind == "ntfy" {
		if len(data.Ntfy) == 0 {
			logError("No ntfy targets configured")
			return 1
		}
		idx, err := resolveRemoveIndex(len(data.Ntfy), target, func(i int) string {
			return DescribeNtfy(data.Ntfy[i])
		})
		if err != nil {
			logError("%s", err.Error())
			return 1
		}
		removedDesc := DescribeNtfy(data.Ntfy[idx])
		data.Ntfy = append(data.Ntfy[:idx], data.Ntfy[idx+1:]...)
		if err := SaveConfig(p, data); err != nil {
			logError("%s", err.Error())
			return 1
		}
		logInfo("Removed ntfy target %s", removedDesc)
		return 0
	}

	if len(data.Growl) == 0 {
		logError("No growl targets configured")
		return 1
	}
	idx, err := resolveRemoveIndex(len(data.Growl), target, func(i int) string {
		return DescribeGrowl(data.Growl[i])
	})
	if err != nil {
		logError("%s", err.Error())
		return 1
	}
	removedDesc := DescribeGrowl(data.Growl[idx])
	data.Growl = append(data.Growl[:idx], data.Growl[idx+1:]...)
	if err := SaveConfig(p, data); err != nil {
		logError("%s", err.Error())
		return 1
	}
	logInfo("Removed growl target %s", removedDesc)
	return 0
}

func resolveRemoveIndex(count int, target string, descFn func(int) string) (int, error) {
	isDigits := len(target) > 0
	for _, ch := range target {
		if ch < '0' || ch > '9' {
			isDigits = false
			break
		}
	}
	if isDigits {
		n, err := strconv.Atoi(target)
		if err != nil || n < 1 || n > count {
			return -1, fmt.Errorf("No target #%s (have %d)", target, count)
		}
		return n - 1, nil
	}
	var hits []int
	for i := 0; i < count; i++ {
		if strings.Contains(descFn(i), target) {
			hits = append(hits, i)
		}
	}
	if len(hits) == 0 {
		return -1, fmt.Errorf("No target matches %q", target)
	}
	if len(hits) > 1 {
		return -1, fmt.Errorf("%q is ambiguous (%d matches); use the # from `routedel config show`", target, len(hits))
	}
	return hits[0], nil
}

func cmdConfigValidate(explicit string) int {
	p := needConfigFile(explicit)
	if p == "" {
		return 1
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		logError("Bad config %s: %s", p, err.Error())
		return 1
	}
	data, err := ReadConfigFile(p)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}
	issues := ValidateConfigRaw(raw)
	hasErr := false
	for _, iss := range issues {
		lvColor := ansiYellow
		if iss.Level == "error" {
			lvColor = ansiRed
			hasErr = true
		}
		fmt.Printf("%s: %s\n", styleStdout(lvColor, iss.Level), iss.Message)
	}
	if hasErr {
		return 1
	}
	n := len(data.Ntfy) + len(data.Growl)
	fmt.Printf("%s: %s (%d target(s))\n", styleStdout(ansiGreen, "OK"), p, n)
	return 0
}

func findEditor() string {
	if ed := os.Getenv("VISUAL"); ed != "" {
		return ed
	}
	if ed := os.Getenv("EDITOR"); ed != "" {
		return ed
	}
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = []string{"notepad", "code", "nano", "vim", "vi"}
	} else {
		candidates = []string{"nano", "vim", "vi"}
	}
	for _, name := range candidates {
		if _, err := exec.LookPath(name); err == nil {
			return name
		}
	}
	return ""
}

func splitCommandLine(cmdStr string) []string {
	var args []string
	var cur strings.Builder
	var quote rune
	escaped := false
	for _, r := range cmdStr {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != '\'' && runtime.GOOS != "windows" {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if cur.Len() > 0 {
				args = append(args, cur.String())
				cur.Reset()
			}
			continue
		}
		cur.WriteRune(r)
	}
	if cur.Len() > 0 {
		args = append(args, cur.String())
	}
	return args
}

func cmdConfigEdit(explicit string) int {
	p := TargetConfigPath(explicit)
	if !isRegularFile(p) {
		if err := SaveConfig(p, emptyConfig()); err != nil {
			logError("%s", err.Error())
			return 1
		}
		logInfo("Created %s", p)
	}
	ed := findEditor()
	if ed == "" {
		logError("No editor found; set $EDITOR (file: %s)", p)
		return 1
	}
	parts := splitCommandLine(ed)
	parts = append(parts, p)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	rc := 0
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			rc = exitErr.ExitCode()
		} else {
			rc = 1
		}
	}
	var errs []string
	raw, err := os.ReadFile(p)
	if err != nil {
		errs = append(errs, fmt.Sprintf("invalid JSON: %s", err.Error()))
	} else {
		var check any
		if jErr := json.Unmarshal(raw, &check); jErr != nil {
			errs = append(errs, fmt.Sprintf("invalid JSON: %s", jErr.Error()))
		} else {
			for _, iss := range ValidateConfigRaw(raw) {
				if iss.Level == "error" {
					errs = append(errs, iss.Message)
				}
			}
		}
	}
	for _, m := range errs {
		logError("%s: %s", p, m)
	}
	if rc != 0 {
		return rc
	}
	if len(errs) > 0 {
		return 1
	}
	return 0
}

func cmdConfigTest(explicit string) int {
	n, err := BuildNotifier(false, explicit, nil, nil)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}
	if len(n.Targets) == 0 {
		logError("No notification targets configured.")
		return 1
	}
	failed := n.Send("Test notification", "info", true, "")
	if failed > 0 {
		logError("%d of %d target(s) failed", failed, len(n.Targets))
		return 1
	}
	return 0
}

func printConfigHelp() {
	fmt.Println(styleStdout(ansiBoldMagenta, "usage: routedel config [-c FILE] COMMAND [ARGS]"))
	fmt.Println()
	fmt.Println("Manage the routedel notification config.")
	fmt.Println()
	fmt.Println(styleStdout(ansiBoldCyan, "Commands:"))
	fmt.Printf("  %s      Print the config file path in use (--all)\n", styleStdout(ansiBoldYellow, "path"))
	fmt.Printf("  %s      Show configured targets (secrets hidden) [--json] [--reveal]\n", styleStdout(ansiBoldYellow, "show"))
	fmt.Printf("  %s      Create a new config file [--force] [--example]\n", styleStdout(ansiBoldYellow, "init"))
	fmt.Printf("  %s       Add an ntfy or Growl target\n", styleStdout(ansiBoldYellow, "add"))
	fmt.Printf("  %s    Remove a target by # or by text match (alias: rm)\n", styleStdout(ansiBoldYellow, "remove"))
	fmt.Printf("  %s  Check the config file for problems\n", styleStdout(ansiBoldYellow, "validate"))
	fmt.Printf("  %s      Open the config in $VISUAL/$EDITOR, then validate\n", styleStdout(ansiBoldYellow, "edit"))
	fmt.Printf("  %s      Send a test notification to every configured target\n", styleStdout(ansiBoldYellow, "test"))
}

func RunConfig(argv []string) int {
	var explicit string
	var rest []string
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "-c" || arg == "--config" {
			if i+1 < len(argv) {
				explicit = argv[i+1]
				i++
			}
		} else if strings.HasPrefix(arg, "-c=") {
			explicit = strings.TrimPrefix(arg, "-c=")
		} else if strings.HasPrefix(arg, "--config=") {
			explicit = strings.TrimPrefix(arg, "--config=")
		} else {
			rest = append(rest, arg)
		}
	}
	if len(rest) == 0 {
		printConfigHelp()
		return 0
	}
	sub := rest[0]
	subArgs := rest[1:]
	switch sub {
	case "-h", "--help", "help":
		printConfigHelp()
		return 0
	case "path":
		return cmdConfigPath(subArgs, explicit)
	case "show", "list", "ls":
		return cmdConfigShow(subArgs, explicit)
	case "init":
		return cmdConfigInit(subArgs, explicit)
	case "add":
		return cmdConfigAdd(subArgs, explicit)
	case "remove", "rm":
		return cmdConfigRemove(subArgs, explicit)
	case "validate":
		return cmdConfigValidate(explicit)
	case "edit":
		return cmdConfigEdit(explicit)
	case "test":
		return cmdConfigTest(explicit)
	default:
		printConfigHelp()
		return 0
	}
}
