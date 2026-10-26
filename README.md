<p align="center">
  <img src="build/appicon.png" width="128" alt="Termimus logo">
</p>

<h1 align="center">Termimus</h1>

<p align="center">A lightweight, open-source SSH client for managing many servers.</p>

Termimus keeps your servers in one place, organised by group, and opens each
one in its own terminal tab with a single click. It is a small native Windows
app (one ~12 MB executable) built with Go and [Wails](https://wails.io), using the
WebView2 runtime that ships with Windows instead of bundling a browser.

## Features

- **Host manager**: add, edit, and delete hosts; organise them into groups; search by name, address, or user.
- **Password or SSH key login**: OpenSSH keys (ed25519, RSA, ECDSA), with or without a passphrase.
- **Encrypted credentials**: passwords and passphrases are encrypted with Windows DPAPI, so they can only be decrypted by your Windows account. They are never sent to the UI.
- **Tabbed terminals** powered by [xterm.js](https://xtermjs.org), with clickable links, 5,000 lines of scrollback, and keep-alive.
- **Host key verification**: you confirm a server's fingerprint the first time you connect; if the key changes later, the connection is refused.
- **Clean tab handling**: typing `exit` closes the tab; a dropped connection keeps the tab open with a **Reconnect** button.

Keyboard shortcuts: `Ctrl+Tab` / `Ctrl+Shift+Tab` switch tabs, `Ctrl+W` closes the current tab.

## Install

Download the latest `termimus-amd64-installer.exe` (or the portable `termimus.exe`)
from the [Releases](../../releases) page.

Requirements: Windows 10 or 11 (64-bit). The installer sets up the WebView2 runtime if it is missing.

> The builds are not code-signed yet, so Windows SmartScreen may warn on first launch.
> Choose **More info → Run anyway**, or build from source.

## Where your data lives

| File | Contents |
| --- | --- |
| `%APPDATA%\termimus\hosts.json` | Host list; secrets are DPAPI-encrypted |
| `%APPDATA%\termimus\known_hosts` | Fingerprints of servers you trusted |

Uninstalling does not delete this folder, so your hosts survive a reinstall.

## Build from source

Prerequisites: [Go](https://go.dev/dl/) 1.25+, [Node.js](https://nodejs.org) 20+, and the Wails CLI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@latest
```

Then:

```sh
wails dev                                      # run with live reload
wails build -trimpath -ldflags "-s -w"         # portable exe → build/bin/termimus.exe
wails build -nsis -trimpath -ldflags "-s -w"   # also build the installer (needs NSIS)
go test ./...                                  # run backend tests
```

The installer step needs [NSIS](https://nsis.sourceforge.io) on your `PATH`
(`winget install NSIS.NSIS`).

### Project layout

| Path | Purpose |
| --- | --- |
| `main.go`, `app.go` | Window setup and the methods exposed to the frontend |
| `ssh.go` | SSH sessions, authentication, host key verification |
| `store.go` | Host list persistence |
| `secret_windows.go` | DPAPI encryption for stored credentials |
| `frontend/src/` | Svelte 5 + TypeScript UI |

## Roadmap

- [ ] Import hosts from `~/.ssh/config`
- [ ] SFTP file browser
- [ ] Snippets that run one command on several servers
- [ ] Port forwarding
- [ ] Split panes
- [ ] macOS and Linux builds (needs a Keychain / libsecret backend)

## Contributing

Issues and pull requests are welcome. Before opening a PR, please run
`go test ./...` and `npm run check` in `frontend/`.

## License

[MIT](LICENSE) © 2026 rafliandi13
