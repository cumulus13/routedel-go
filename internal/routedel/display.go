package routedel

// File: internal/routedel/display.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: Rich-styled ANSI display and live monitor.
// License: MIT

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"
)

type RouteKey struct {
	Destination string
	Gateway     string
	Genmask     string
	Metric      string
	Iface       string
}

func MakeRouteKey(e Route) RouteKey {
	return RouteKey{
		Destination: e["Destination"],
		Gateway:     e["Gateway"],
		Genmask:     e["Genmask"],
		Metric:      e["Metric"],
		Iface:       e["Iface"],
	}
}

func FormatRouteKey(k RouteKey) string {
	pfx, _ := PrefixLen(k.Genmask)
	return fmt.Sprintf("%s/%d via %s dev %s metric %s", k.Destination, pfx, k.Gateway, k.Iface, k.Metric)
}

func DiffRoutes(prev map[RouteKey]Route, cur map[RouteKey]Route) ([]string, bool) {
	var added []RouteKey
	var removed []RouteKey
	for k := range cur {
		if _, exists := prev[k]; !exists {
			added = append(added, k)
		}
	}
	for k := range prev {
		if _, exists := cur[k]; !exists {
			removed = append(removed, k)
		}
	}
	if len(added) == 0 && len(removed) == 0 {
		return nil, false
	}
	sort.Slice(added, func(i, j int) bool {
		return FormatRouteKey(added[i]) < FormatRouteKey(added[j])
	})
	sort.Slice(removed, func(i, j int) bool {
		return FormatRouteKey(removed[i]) < FormatRouteKey(removed[j])
	})

	var lines []string
	for _, k := range added {
		lines = append(lines, "+ "+FormatRouteKey(k))
	}
	for _, k := range removed {
		lines = append(lines, "- "+FormatRouteKey(k))
	}
	if len(lines) > 10 {
		moreCount := len(lines) - 10
		lines = append(lines[:10], fmt.Sprintf("... +%d more", moreCount))
	}

	dflt := false
	for _, k := range added {
		if k.Destination == "0.0.0.0" && k.Genmask == "0.0.0.0" {
			dflt = true
		}
	}
	for _, k := range removed {
		if k.Destination == "0.0.0.0" && k.Genmask == "0.0.0.0" {
			dflt = true
		}
	}
	return lines, dflt
}

func RenderRouteTable(routes Routes, title string) string {
	var b strings.Builder
	b.WriteString(styleStdout(ansiBoldMagenta, title) + "\n")
	b.WriteString(fmt.Sprintf("%s %s %s %s %s %s %s %s %s\n",
		styleStdout(ansiCyan, fmt.Sprintf("%-5s", "Idx")),
		styleStdout(ansiBoldCyan, fmt.Sprintf("%-15s", "Destination")),
		styleStdout(ansiBoldYellow, fmt.Sprintf("%-15s", "Gateway")),
		styleStdout(ansiBoldBlue, fmt.Sprintf("%-15s", "Genmask")),
		styleStdout(ansiBoldMagenta, fmt.Sprintf("%-7s", "Flags")),
		styleStdout(ansiBoldGreen, fmt.Sprintf("%-8s", "Metric")),
		styleStdout(ansiBoldRed, fmt.Sprintf("%-5s", "Ref")),
		styleStdout(ansiBoldWhite, fmt.Sprintf("%-5s", "Use")),
		styleStdout(ansiBoldCyan, fmt.Sprintf("%-8s", "Iface")),
	))
	keys := make([]int, 0, len(routes))
	for k := range routes {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, idx := range keys {
		e := routes[idx]
		idxColor := ansiCyan
		if IsDefault(e) {
			idxColor = ansiBoldRed
		}
		b.WriteString(fmt.Sprintf("%s %s %s %s %s %s %s %s %s\n",
			styleStdout(idxColor, fmt.Sprintf("%-5d", idx)),
			styleStdout(ansiBoldCyan, fmt.Sprintf("%-15s", e["Destination"])),
			styleStdout(ansiBoldYellow, fmt.Sprintf("%-15s", e["Gateway"])),
			styleStdout(ansiBoldBlue, fmt.Sprintf("%-15s", e["Genmask"])),
			styleStdout(ansiBoldMagenta, fmt.Sprintf("%-7s", e["Flags"])),
			styleStdout(ansiBoldGreen, fmt.Sprintf("%-8s", e["Metric"])),
			styleStdout(ansiBoldRed, fmt.Sprintf("%-5s", e["Ref"])),
			styleStdout(ansiBoldWhite, fmt.Sprintf("%-5s", e["Use"])),
			styleStdout(ansiBoldCyan, fmt.Sprintf("%-8s", e["Iface"])),
		))
	}
	return strings.TrimRight(b.String(), "\n")
}

func RenderMonitorLayout(routes Routes, lastUpdate string, interval float64, nTargets int) string {
	ifaceSet := make(map[string]bool)
	defaultCount := 0
	for _, e := range routes {
		if e["Iface"] != "" {
			ifaceSet[e["Iface"]] = true
		}
		if IsDefault(e) {
			defaultCount++
		}
	}
	ifaces := make([]string, 0, len(ifaceSet))
	for k := range ifaceSet {
		ifaces = append(ifaces, k)
	}
	sort.Strings(ifaces)

	var b strings.Builder
	b.WriteString(styleStdout(ansiBoldMagenta, "Route Monitor") +
		styleStdout(ansiDim, fmt.Sprintf(" | %s | refresh %.6gs | notify targets: %d | Ctrl+C to exit", lastUpdate, interval, nTargets)) + "\n")
	b.WriteString(styleStdout(ansiBoldBlue, "--------------------------------------------------------------------------------") + "\n")
	b.WriteString(RenderRouteTable(routes, fmt.Sprintf("Routing Table (%d routes)", len(routes))))
	b.WriteString("\n" + styleStdout(ansiDim, "--------------------------------------------------------------------------------") + "\n")
	b.WriteString(styleStdout(ansiBoldWhite, fmt.Sprintf("Routes: %d", len(routes))) +
		styleStdout(ansiBoldRed, fmt.Sprintf(" | Default: %d", defaultCount)) +
		styleStdout(ansiCyan, fmt.Sprintf(" | Interfaces: %s", strings.Join(ifaces, ", "))))
	return b.String()
}

func MonitorRoutes(interval float64, notifier *Notifier) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	prev := make(map[RouteKey]Route)
	for _, e := range GetRoutes() {
		prev[MakeRouteKey(e)] = e
	}
	notifier.Send(fmt.Sprintf("Monitor started (%d routes)", len(prev)), "info", false, "")

	defer func() {
		notifier.Send("Monitor stopped", "info", true, "")
		fmt.Println(styleStdout(ansiGreen, "Route monitoring stopped."))
	}()

	dur := time.Duration(interval * float64(time.Second))
	if dur <= 0 {
		dur = 2 * time.Second
	}

	for {
		if ctx.Err() != nil {
			return
		}
		routes := GetRoutes()
		cur := make(map[RouteKey]Route)
		for _, e := range routes {
			cur[MakeRouteKey(e)] = e
		}
		lines, dflt := DiffRoutes(prev, cur)
		if len(lines) > 0 {
			level := "info"
			if dflt {
				level = "warning"
			}
			notifier.Send("Routing table changed:\n"+strings.Join(lines, "\n"), level, false, "")
			prev = cur
		}
		fmt.Print("\x1b[2J\x1b[H")
		fmt.Println(RenderMonitorLayout(routes, time.Now().Format("15:04:05"), interval, len(notifier.Targets)))

		select {
		case <-ctx.Done():
			return
		case <-time.After(dur):
		}
	}
}
