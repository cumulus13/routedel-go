# routedel

Delete, add, list, and monitor IPv4 routes with parallel `ntfy` (HTTP) and Growl (GNTP 1.0) notifications.

## Overview

`routedel` is a Go port of the `routedel` CLI tool for reading, modifying, and monitoring IPv4 routing tables on Linux (using `ip` or `route`), while sending notifications concurrently to one or more `ntfy` endpoints and Growl/GNTP servers. It includes Rich-matching ANSI terminal colors (`NO_COLOR` / `FORCE_COLOR` aware) and platform-specific configuration paths across Linux, macOS, and Windows.

## Project Structure

- `cmd/routedel/main.go` — CLI entry point.
- `internal/routedel/version.go` — Application name and version (`1.0.4`).
- `internal/routedel/color.go` — ANSI terminal color styling matching Rich console and `richcolorlog`.
- `internal/routedel/cli.go` — Top-level CLI argument parsing and command dispatch.
- `internal/routedel/config.go` — Platform-aware configuration file lookup, validation, masking, and `routedel config` (`init`, `path`, `show`, `add`, `remove`, `validate`, `edit`, `test`) subcommands.
- `internal/routedel/routes.go` — Route reading, `route -n` / `ip -4 route show` parsing, command construction, and route add/delete operations.
- `internal/routedel/display.go` — Colorized route table rendering, route diffing, and real-time monitoring loop.
- `internal/routedel/notify.go` — Parallel `ntfy` HTTP POST and Growl GNTP 1.0 (with optional `SHA256` authentication) notifier.
- `docs/USAGE.md` — Detailed command-line and configuration reference.

## Build & Run

```bash
go mod tidy
go build -o routedel ./cmd/routedel
./routedel --help
```

## License

MIT License. Author: Hadi Cahyadi (`cumulus13@gmail.com`).

## 👤 Author
        
[Hadi Cahyadi](mailto:cumulus13@gmail.com)
    

[![Buy Me a Coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/cumulus13)

[![Donate via Ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/cumulus13)
 
[Support me on Patreon](https://www.patreon.com/cumulus13)