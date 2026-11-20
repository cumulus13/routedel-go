package routedel

// File: internal/routedel/cli.go
// Author: Hadi Cahyadi <cumulus13@gmail.com>
// Date: 2026-10-26
// Description: Command line interface.
// License: MIT

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
)

func logInfo(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", styleStderr(ansiBoldGreen, "[INFO]"), msg)
}

func logWarning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", styleStderr(ansiBoldYellow, "[WARNING]"), styleStderr(ansiYellow, msg))
}

func logError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "%s %s\n", styleStderr(ansiBoldRed, "[ERROR]"), styleStderr(ansiRed, msg))
}

func printHelp() {
	fmt.Println(styleStdout(ansiBoldMagenta, fmt.Sprintf("usage: %s [-h] [-V] [-m METRIC] [-l] [-a] [-d] [-g GATEWAY] [-i INTERFACE] [--met MET] [--monitor] [--refresh REFRESH] [--ntfy URL] [--growl HOST[:PORT]] [--config CONFIG] [--no-notify] [--test-notify] [data ...]", AppName)))
	fmt.Println()
	fmt.Println(styleStdout(ansiBoldCyan, "positional arguments:"))
	fmt.Printf("  %s                  Route string, or \"c\" to read from clipboard (see also: routedel config -h)\n", styleStdout(ansiBoldYellow, "data"))
	fmt.Println()
	fmt.Println(styleStdout(ansiBoldCyan, "options:"))
	fmt.Printf("  %s            show this help message and exit\n", styleStdout(ansiBoldGreen, "-h, --help"))
	fmt.Printf("  %s         show program's version number and exit\n", styleStdout(ansiBoldGreen, "-V, --version"))
	fmt.Printf("  %s   Remove route(s) by metric\n", styleStdout(ansiBoldGreen, "-m, --metric METRIC"))
	fmt.Printf("  %s            List current routes\n", styleStdout(ansiBoldGreen, "-l, --list"))
	fmt.Printf("  %s             Add route (needs -g and -i; DATA[0]=CIDR unless -d)\n", styleStdout(ansiBoldGreen, "-a, --add"))
	fmt.Printf("  %s         Add as default route\n", styleStdout(ansiBoldGreen, "-d, --default"))
	fmt.Printf("  %s Gateway IP for adding route\n", styleStdout(ansiBoldGreen, "-g, --gateway GATEWAY"))
	fmt.Printf("  %s\n", styleStdout(ansiBoldGreen, "-i, --interface INTERFACE"))
	fmt.Println("                        Interface name for adding route")
	fmt.Printf("  %s             Metric for adding route\n", styleStdout(ansiBoldGreen, "--met MET"))
	fmt.Printf("  %s             Monitor routing table in real-time\n", styleStdout(ansiBoldGreen, "--monitor"))
	fmt.Printf("  %s     Monitor refresh seconds (default: 2.0)\n", styleStdout(ansiBoldGreen, "--refresh REFRESH"))
	fmt.Printf("  %s            ntfy topic URL, e.g. https://ntfy.sh/topic (repeatable)\n", styleStdout(ansiBoldGreen, "--ntfy URL"))
	fmt.Printf("  %s   Growl/GNTP server (repeatable)\n", styleStdout(ansiBoldGreen, "--growl HOST[:PORT]"))
	fmt.Printf("  %s       Notification config JSON file\n", styleStdout(ansiBoldGreen, "--config CONFIG"))
	fmt.Printf("  %s           Disable all notifications\n", styleStdout(ansiBoldGreen, "--no-notify"))
	fmt.Printf("  %s         Send a test notification and exit\n", styleStdout(ansiBoldGreen, "--test-notify"))
}

type parsedCLI struct {
	version    bool
	help       bool
	metric     string
	hasMetric  bool
	list       bool
	add        bool
	defRoute   bool
	gateway    string
	interfaceN string
	met        int
	monitor    bool
	refresh    float64
	ntfy       []string
	growl      []string
	config     string
	noNotify   bool
	testNotify bool
	data       []string
}

func parseCLIArgs(argv []string) (parsedCLI, error) {
	cfg := parsedCLI{
		met:     0,
		refresh: 2.0,
	}
	i := 0
	for i < len(argv) {
		arg := argv[i]
		if arg == "--" {
			cfg.data = append(cfg.data, argv[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			cfg.data = append(cfg.data, arg)
			i++
			continue
		}

		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			eqIdx := strings.IndexByte(arg, '=')
			name := arg[:eqIdx]
			val := arg[eqIdx+1:]
			switch name {
			case "--metric":
				cfg.metric = val
				cfg.hasMetric = true
			case "--gateway":
				cfg.gateway = val
			case "--interface":
				cfg.interfaceN = val
			case "--met":
				m, err := strconv.Atoi(val)
				if err != nil {
					return cfg, fmt.Errorf("invalid --met value %q: %w", val, err)
				}
				cfg.met = m
			case "--refresh":
				r, err := strconv.ParseFloat(val, 64)
				if err != nil {
					return cfg, fmt.Errorf("invalid --refresh value %q: %w", val, err)
				}
				cfg.refresh = r
			case "--ntfy":
				cfg.ntfy = append(cfg.ntfy, val)
			case "--growl":
				cfg.growl = append(cfg.growl, val)
			case "--config":
				cfg.config = val
			default:
				return cfg, fmt.Errorf("unrecognized option: %s", arg)
			}
			i++
			continue
		}

		switch arg {
		case "-h", "--help":
			cfg.help = true
		case "-V", "--version":
			cfg.version = true
		case "-m", "--metric":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			cfg.metric = argv[i+1]
			cfg.hasMetric = true
			i++
		case "-l", "--list":
			cfg.list = true
		case "-a", "--add":
			cfg.add = true
		case "-d", "--default":
			cfg.defRoute = true
		case "-g", "--gateway":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			cfg.gateway = argv[i+1]
			i++
		case "-i", "--interface":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			cfg.interfaceN = argv[i+1]
			i++
		case "--met":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			m, err := strconv.Atoi(argv[i+1])
			if err != nil {
				return cfg, fmt.Errorf("invalid --met value %q: %w", argv[i+1], err)
			}
			cfg.met = m
			i++
		case "--monitor":
			cfg.monitor = true
		case "--refresh":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			r, err := strconv.ParseFloat(argv[i+1], 64)
			if err != nil {
				return cfg, fmt.Errorf("invalid --refresh value %q: %w", argv[i+1], err)
			}
			cfg.refresh = r
			i++
		case "--ntfy":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			cfg.ntfy = append(cfg.ntfy, argv[i+1])
			i++
		case "--growl":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			cfg.growl = append(cfg.growl, argv[i+1])
			i++
		case "--config":
			if i+1 >= len(argv) {
				return cfg, fmt.Errorf("option %s requires an argument", arg)
			}
			cfg.config = argv[i+1]
			i++
		case "--no-notify":
			cfg.noNotify = true
		case "--test-notify":
			cfg.testNotify = true
		default:
			return cfg, fmt.Errorf("unrecognized option: %s", arg)
		}
		i++
	}
	return cfg, nil
}

func readClipboard() (string, error) {
	var tools [][]string
	switch runtime.GOOS {
	case "windows":
		tools = [][]string{
			{"powershell", "-NoProfile", "-Command", "Get-Clipboard"},
			{"pwsh", "-NoProfile", "-Command", "Get-Clipboard"},
		}
	case "darwin":
		tools = [][]string{
			{"pbpaste"},
		}
	default:
		tools = [][]string{
			{"xclip", "-o", "-selection", "clipboard"},
			{"xsel", "-ob"},
			{"wl-paste"},
		}
	}
	for _, t := range tools {
		if _, err := exec.LookPath(t[0]); err == nil {
			out, err := exec.Command(t[0], t[1:]...).Output()
			if err == nil {
				return string(out), nil
			}
		}
	}
	return "", fmt.Errorf("clipboard tool not available")
}

func parseCIDRToNetMask(cidr string) (string, string, error) {
	if !strings.Contains(cidr, "/") {
		ip := net.ParseIP(cidr).To4()
		if ip == nil {
			return "", "", fmt.Errorf("invalid IPv4 address: %q", cidr)
		}
		return ip.String(), "255.255.255.255", nil
	}
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", "", err
	}
	ip4 := ipNet.IP.To4()
	if ip4 == nil {
		return "", "", fmt.Errorf("not an IPv4 network: %q", cidr)
	}
	mask := ipNet.Mask
	if len(mask) != 4 {
		return "", "", fmt.Errorf("invalid IPv4 mask for %q", cidr)
	}
	maskStr := fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	return ip4.String(), maskStr, nil
}

func Main(argv []string) int {
	if len(argv) > 0 && argv[0] == "config" {
		return RunConfig(argv[1:])
	}

	if len(argv) == 0 {
		printHelp()
		return 0
	}

	args, err := parseCLIArgs(argv)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}

	if args.help {
		printHelp()
		return 0
	}

	if args.version {
		fmt.Printf("%s %s\n", styleStdout(ansiBoldCyan, AppName), styleStdout(ansiBoldGreen, Version))
		return 0
	}

	notifier, err := BuildNotifier(args.noNotify, args.config, args.ntfy, args.growl)
	if err != nil {
		logError("%s", err.Error())
		return 1
	}

	if args.testNotify {
		if len(notifier.Targets) == 0 {
			logError("No notification targets configured.")
			return 1
		}
		failed := notifier.Send("Test notification", "info", true, "")
		if failed > 0 {
			logError("%d of %d target(s) failed", failed, len(notifier.Targets))
			return 1
		}
		return 0
	}

	if args.monitor {
		MonitorRoutes(args.refresh, notifier)
		return 0
	}

	if args.list {
		fmt.Println(RenderRouteTable(GetRoutes(), "Current Routes"))
		return 0
	}

	if (args.add || len(args.data) > 0 || args.hasMetric) && runtime.GOOS != "windows" && os.Geteuid() != 0 {
		logWarning("Not running as root; route changes will likely fail.")
	}

	if args.add {
		if args.gateway == "" || args.interfaceN == "" {
			logError("--add requires -g GATEWAY and -i INTERFACE")
			return 1
		}
		var dest, mask string
		if args.defRoute {
			dest = "0.0.0.0"
			mask = "0.0.0.0"
		} else if len(args.data) > 0 {
			d, m, cErr := parseCIDRToNetMask(args.data[0])
			if cErr != nil {
				logError("Invalid network '%s': %s", args.data[0], cErr.Error())
				return 1
			}
			dest = d
			mask = m
		} else {
			logError("Provide DATA (CIDR) or use -d for default route")
			return 1
		}
		if RouteAction("add", dest, mask, args.gateway, args.interfaceN, strconv.Itoa(args.met), notifier) {
			return 0
		}
		return 1
	}

	if len(args.data) > 0 {
		var tokens []string
		if len(args.data) == 1 && args.data[0] == "c" {
			pasted, cbErr := readClipboard()
			if cbErr != nil {
				logError("Clipboard support missing: %s", cbErr.Error())
				return 1
			}
			tokens = strings.Fields(pasted)
		} else {
			tokens = args.data
		}

		if len(tokens) == 8 || len(tokens) == 9 {
			parsed := ParseRouteTable(strings.Join(tokens, " "))
			e, ok := parsed[0]
			if !ok {
				logError("Could not parse route row")
				return 1
			}
			if DeleteEntry(e, notifier) {
				return 0
			}
			return 1
		}
		if len(tokens) == 4 {
			if DeleteDefault(tokens[1], tokens[2], tokens[3], notifier) {
				return 0
			}
			return 1
		}
		logError("Unsupported data format (%d tokens)", len(tokens))
		return 1
	}

	if args.hasMetric {
		matches := FindByMetric(GetRoutes(), args.metric)
		if len(matches) == 0 {
			logError("No route found with metric %s", args.metric)
			return 1
		}
		toDelete := matches
		if len(matches) > 1 {
			logWarning("Multiple routes found with metric %s:", args.metric)
			for _, m := range matches {
				fmt.Printf("  [%s] %v\n", styleStdout(ansiBoldCyan, strconv.Itoa(m.Index)), m.Route)
			}
			fmt.Print("Index to delete, 'a' = all, 'c' = cancel: ")
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			q := strings.ToLower(strings.TrimSpace(line))
			if q == "c" {
				logInfo("Operation cancelled.")
				return 0
			}
			if q == "a" {
			} else if idxVal, err := strconv.Atoi(q); err == nil {
				var filtered []IndexedRoute
				for _, m := range matches {
					if m.Index == idxVal {
						filtered = append(filtered, m)
					}
				}
				if len(filtered) == 0 {
					logError("Invalid input, operation cancelled.")
					return 1
				}
				toDelete = filtered
			} else {
				logError("Invalid input, operation cancelled.")
				return 1
			}
		}
		allOK := true
		for _, m := range toDelete {
			if !DeleteEntry(m.Route, notifier) {
				allOK = false
			}
		}
		if allOK {
			return 0
		}
		return 1
	}

	logError("Invalid arguments. Provide route details, -m METRIC, -a, -l or --monitor.")
	printHelp()
	return 1
}
