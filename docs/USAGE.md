# routedel Usage Guide

## Top-Level Commands

- `routedel -l` / `routedel --list` — List current IPv4 routes with ANSI colors.
- `routedel -a -g <GW> -i <IFACE> <CIDR>` — Add a route (or use `-d` for default route `0.0.0.0/0`, optional `--met <METRIC>`).
- `routedel -m <METRIC>` — Remove route(s) matching `<METRIC>` (prompts when multiple routes share the metric).
- `routedel <8 or 9 route tokens>` / `routedel c` — Delete a route from arguments or clipboard (`c`).
- `routedel <4 tokens: X GW METRIC DEV>` — Delete a default route (`0.0.0.0/0`).
- `routedel --monitor [--refresh SECONDS]` — Monitor routing table changes and send notifications.
- `routedel --test-notify` — Send a test notification to configured targets and exit.

## Color Control

ANSI colors automatically follow TTY detection and standard environment variables:
- `NO_COLOR=1` or `TERM=dumb` disables ANSI colors.
- `FORCE_COLOR=1` forces ANSI colors even when piped.

## Notification Flags

- `--ntfy URL` — Repeatable `ntfy` topic URL (e.g., `https://ntfy.sh/topic`).
- `--growl HOST[:PORT]` — Repeatable Growl/GNTP server (default port `23053`).
- `--config FILE` — Path to notification config JSON file.
- `--no-notify` — Disable all notifications.

## Config Subcommands (`routedel config`)

Platform-aware lookup order (first existing file wins):
1. `--config`
2. `$ROUTEDEL_CONFIG`
3. Platform user config directory:
   - **Linux / BSD**: `$XDG_CONFIG_HOME/routedel/config.json` or `~/.config/routedel/config.json`
   - **macOS**: `~/Library/Application Support/routedel/config.json` (or `$XDG_CONFIG_HOME/routedel/config.json` if set)
   - **Windows**: `%APPDATA%\routedel\config.json`
4. Cross-platform XDG fallback (`~/.config/routedel/config.json`, when distinct)
5. Legacy user file (`~/.routedel.json`)
6. Platform system-wide config:
   - **Linux / BSD**: `/etc/routedel/config.json`
   - **macOS**: `/Library/Application Support/routedel/config.json`
   - **Windows**: `%ProgramData%\routedel\config.json`

Subcommands:
- `routedel config path [--all]`
- `routedel config show [--json] [--reveal]` (aliases: `list`, `ls`)
- `routedel config init [--force] [--example]`
- `routedel config add ntfy|growl TARGET [--topic TOPIC] [--token TOKEN] [--user USER] [--password PASS] [--port PORT] [--timeout SEC]`
- `routedel config remove|rm ntfy|growl TARGET`
- `routedel config validate`
- `routedel config edit`
- `routedel config test`

## 👤 Author
        
[Hadi Cahyadi](mailto:cumulus13@gmail.com)
    

[![Buy Me a Coffee](https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png)](https://www.buymeacoffee.com/cumulus13)

[![Donate via Ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/cumulus13)
 
[Support me on Patreon](https://www.patreon.com/cumulus13)