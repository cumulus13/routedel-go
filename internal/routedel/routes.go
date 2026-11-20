package routedel

// File: internal/routedel/routes.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: Route table reading, parsing and modification (Linux `ip` / `route`).
// License: MIT

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

type Route map[string]string

type Routes map[int]Route

type IndexedRoute struct {
	Index int
	Route Route
}

var RouteCols = []string{"Destination", "Gateway", "Genmask", "Flags", "Metric", "Ref", "Use", "Iface"}

func Netmask(prefix int) (string, error) {
	if prefix < 0 || prefix > 32 {
		return "", fmt.Errorf("invalid prefix length: %d", prefix)
	}
	mask := net.CIDRMask(prefix, 32)
	return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3]), nil
}

func PrefixLen(maskStr string) (int, error) {
	ip := net.ParseIP(strings.TrimSpace(maskStr)).To4()
	if ip == nil {
		return 0, fmt.Errorf("invalid netmask: %q", maskStr)
	}
	ones, bits := net.IPv4Mask(ip[0], ip[1], ip[2], ip[3]).Size()
	if bits != 32 {
		return 0, fmt.Errorf("invalid IPv4 netmask: %q", maskStr)
	}
	return ones, nil
}

func IsDefault(r Route) bool {
	return r["Destination"] == "0.0.0.0" && r["Genmask"] == "0.0.0.0"
}

func ParseRouteTable(text string) Routes {
	var lines []string
	for _, ln := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed != "" {
			lines = append(lines, ln)
		}
	}
	hdrIdx := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Destination") && strings.Contains(ln, "Gateway") {
			hdrIdx = i
			break
		}
	}
	var bodyLines []string
	if hdrIdx >= 0 {
		bodyLines = lines[hdrIdx+1:]
	} else {
		bodyLines = lines
	}
	out := make(Routes)
	for _, ln := range bodyLines {
		cleaned := strings.TrimSpace(ln)
		if strings.HasPrefix(cleaned, "#") {
			cleaned = strings.TrimSpace(strings.TrimPrefix(cleaned, "#"))
		}
		parts := strings.Fields(cleaned)
		if len(parts) == len(RouteCols)+1 {
			allDigits := len(parts[0]) > 0
			for _, ch := range parts[0] {
				if ch < '0' || ch > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				parts = parts[1:]
			}
		}
		if len(parts) != len(RouteCols) {
			continue
		}
		r := make(Route, len(RouteCols))
		for i, col := range RouteCols {
			r[col] = parts[i]
		}
		out[len(out)] = r
	}
	return out
}

func ParseIPRoute(text string) Routes {
	out := make(Routes)
	for _, ln := range strings.Split(text, "\n") {
		p := strings.Fields(ln)
		if len(p) == 0 {
			continue
		}
		switch p[0] {
		case "blackhole", "unreachable", "prohibit", "throw":
			continue
		}
		dest := p[0]
		kv := make(map[string]string)
		for i := 0; i < len(p)-1; i++ {
			if p[i] == "via" || p[i] == "dev" || p[i] == "metric" {
				kv[p[i]] = p[i+1]
			}
		}
		var destIP, mask string
		if dest == "default" {
			destIP = "0.0.0.0"
			mask = "0.0.0.0"
		} else if strings.Contains(dest, "/") {
			parts := strings.SplitN(dest, "/", 2)
			pre, err := strconv.Atoi(parts[1])
			if err != nil {
				continue
			}
			m, err := Netmask(pre)
			if err != nil {
				continue
			}
			destIP = parts[0]
			mask = m
		} else {
			destIP = dest
			mask = "255.255.255.255"
		}
		if net.ParseIP(destIP) == nil {
			continue
		}
		gw := "0.0.0.0"
		flags := "U"
		if v, ok := kv["via"]; ok {
			gw = v
			flags = "UG"
		}
		metric := "0"
		if mVal, ok := kv["metric"]; ok {
			metric = mVal
		}
		iface := ""
		if devVal, ok := kv["dev"]; ok {
			iface = devVal
		}
		out[len(out)] = Route{
			"Destination": destIP,
			"Gateway":     gw,
			"Genmask":     mask,
			"Flags":       flags,
			"Metric":      metric,
			"Ref":         "0",
			"Use":         "0",
			"Iface":       iface,
		}
	}
	return out
}

func GetRoutes() Routes {
	if _, err := exec.LookPath("ip"); err == nil {
		cmd := exec.Command("ip", "-4", "route", "show")
		out, err := cmd.Output()
		if err == nil {
			return ParseIPRoute(string(out))
		}
		logError("Failed to read routes: %s", err.Error())
		return Routes{}
	}
	if _, err := exec.LookPath("route"); err == nil {
		cmd := exec.Command("route", "-n")
		out, err := cmd.Output()
		if err == nil {
			return ParseRouteTable(string(out))
		}
		logError("Failed to read routes: %s", err.Error())
		return Routes{}
	}
	return Routes{}
}

func FindByMetric(data Routes, metric string) []IndexedRoute {
	var matches []IndexedRoute
	for k, e := range data {
		if e["Metric"] == metric {
			matches = append(matches, IndexedRoute{Index: k, Route: e})
		}
	}
	return matches
}

func runCommand(args []string) (bool, string) {
	if len(args) == 0 {
		return false, ""
	}
	cmd := exec.Command(args[0], args[1:]...)
	combined, err := cmd.CombinedOutput()
	msg := strings.TrimSpace(string(combined))
	return err == nil, msg
}

func BuildCmd(action string, dest string, mask string, gw string, iface string, metric string) []string {
	hasGW := gw != "" && gw != "0.0.0.0"
	hasMetric := metric != "" && metric != "0"
	isDefault := dest == "0.0.0.0" && mask == "0.0.0.0"
	pfx, _ := PrefixLen(mask)

	if _, err := exec.LookPath("ip"); err == nil {
		target := fmt.Sprintf("%s/%d", dest, pfx)
		if isDefault {
			target = "default"
		}
		cmd := []string{"ip", "route", action, target}
		if hasGW {
			cmd = append(cmd, "via", gw)
		}
		cmd = append(cmd, "dev", iface)
		if hasMetric {
			cmd = append(cmd, "metric", metric)
		}
		return cmd
	}

	cmd := []string{"route", action}
	if isDefault {
		cmd = append(cmd, "default")
	} else {
		cmd = append(cmd, "-net", dest, "netmask", mask)
	}
	if hasGW {
		cmd = append(cmd, "gw", gw)
	}
	if hasMetric {
		cmd = append(cmd, "metric", metric)
	}
	cmd = append(cmd, "dev", iface)
	return cmd
}

func RouteAction(action string, dest string, mask string, gw string, iface string, metric string, notifier *Notifier) bool {
	pfx, _ := PrefixLen(mask)
	metDisp := metric
	if metDisp == "" {
		metDisp = "0"
	}
	desc := fmt.Sprintf("%s/%d via %s dev %s metric %s", dest, pfx, gw, iface, metDisp)
	cmd := BuildCmd(action, dest, mask, gw, iface, metric)
	logWarning("%s: %s", action, strings.Join(cmd, " "))
	ok, msg := runCommand(cmd)
	verb := "deleted"
	if action == "add" {
		verb = "added"
	}
	if ok {
		logInfo("Route %s: %s", verb, desc)
		notifier.Send(fmt.Sprintf("Route %s: %s", verb, desc), "info", true, "")
	} else {
		logError("Failed to %s route: %s", action, msg)
		notifier.Send(fmt.Sprintf("FAILED to %s route: %s\n%s", action, desc, msg), "error", true, "")
	}
	return ok
}

func DeleteEntry(e Route, notifier *Notifier) bool {
	if e["Iface"] == "" {
		logError("Entry missing Iface, skipping: %v", e)
		return false
	}
	return RouteAction("del", e["Destination"], e["Genmask"], e["Gateway"], e["Iface"], e["Metric"], notifier)
}

func DeleteDefault(gw string, metric string, iface string, notifier *Notifier) bool {
	return RouteAction("del", "0.0.0.0", "0.0.0.0", gw, iface, metric, notifier)
}
