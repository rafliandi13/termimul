# Contributing to Termimul

Thanks for your interest in improving Termimul. Bug reports, feature ideas,
documentation fixes and code are all welcome.

By contributing, you agree that your work is released under the project's
[MIT License](LICENSE).

## Reporting a bug

Search the [existing issues](https://github.com/rafliandi13/termimul/issues)
first. If yours is new, include:

- Your OS and version, and the Termimul version (or commit).
- The steps to reproduce it.
- What you expected, and what happened instead.
- Any error message or screenshot.

Remove server addresses, usernames, keys and passwords from anything you post.

**Security problems:** please don't open a public issue. Report them privately
through [GitHub security advisories](https://github.com/rafliandi13/termimul/security/advisories/new).

## Suggesting a feature

Open an issue that describes the problem you want solved and how you imagine it
working. For anything large, wait for a reply before you start coding, so your
time isn't spent on something that may not fit the project.

## Setting up

You need [Go](https://go.dev/dl/) 1.25+, [Node.js](https://nodejs.org) 20+ and the
Wails CLI:

```sh
go install github.com/wailsapp/wails/v2/cmd/wails@v2.17.0
```

On Linux, also install `build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev`.

Then fork the repository and run the app with live reload:

```sh
git clone https://github.com/<your-username>/termimul.git
cd termimul
wails dev
```

If the window stays blank and the log says `wails.localhost` is not an allowed
host, allow it and start again:

```sh
WAILS_DEV_ALLOWED_HOSTS=wails.localhost wails dev           # bash, zsh
$env:WAILS_DEV_ALLOWED_HOSTS="wails.localhost"; wails dev   # PowerShell
```

## How the code is organised

The [project layout](README.md#project-layout) lists every package. Three rules
keep it that way:

- `internal/services` is the only layer the frontend calls, and the only one that
  imports Wails. Dialogs and events belong there. Everything else in `internal/`
  stays free of Wails, so it can be tested on its own.
- Secrets stay in the backend. Passwords and private keys are encrypted through
  `internal/secret` and never sent to the UI; the frontend only sees flags such as
  `hasPassword`.
- `frontend/wailsjs/` is generated. Don't edit it by hand. After you add or change
  a service method or a type it uses, regenerate it and commit the result:

  ```sh
  wails generate module
  ```

## Code style

- **Go:** format with `gofmt` and check with `go vet ./...`.
- **Frontend:** Svelte 5 with runes and TypeScript. `npm run check` must pass.
- **Comments:** in English, and only where they explain why.
- **User-facing text:** the app's UI and backend error messages are in Indonesian.
  Write new ones in Indonesian too. If you can't, say so in the pull request and
  a maintainer will help.
- Match the code around your change, and keep changes focused on one thing.

## Tests

Add tests for backend changes. `internal/sshtest` provides throwaway SSH, SFTP,
exec and port-forwarding servers, so tests never need a real server.

Before you open a pull request, run:

```sh
go vet ./...
go test ./...
cd frontend && npm run check && npm run build
```

`main.go` embeds `frontend/dist`, so build the frontend at least once before
running the Go commands on a fresh clone.

If your change affects the UI, try it in `wails dev` as well.

## Submitting a pull request

1. Create a branch from `main`, for example `feature/split-panes` or `fix/sftp-rename`.
2. Write commit messages in the imperative mood, with a short first line:
   `Add port forwarding`, `Fix copy and paste in the terminal`.
3. Update `README.md` if you add a feature or change how something works.
4. Open the pull request against `main`. Explain what changed and how you tested
   it, attach a screenshot for UI changes, and link the issue with `Closes #123`.

GitHub Actions runs the checks and builds on every pull request, and they must
pass before merging. Your pull request may get review comments asking for changes;
that's a normal part of the process.
