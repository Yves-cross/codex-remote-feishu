# Quick Start

## Option 1: One-line install on macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/kxn/codex-remote-feishu/master/install-release.sh | bash
```

This command will:

1. Detect your platform
2. Download the latest release package
3. Extract it under your local release cache
4. Start the packaged interactive installer

To pin a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/kxn/codex-remote-feishu/master/install-release.sh | bash -s -- --version v1.0.0
```

## Option 2: Download a release archive

1. Download the archive matching your platform from GitHub Releases
2. Extract it
3. Run:

macOS / Linux:

```bash
./setup.sh
```

Windows PowerShell:

```powershell
.\setup.ps1
```

## After installation

Start the relay service on Linux with:

```bash
./install.sh start
```

If you only need to restart the current relay chain, or recover from a stale daemon that is still alive without a PID file:

```bash
./install.sh restart
```

If you changed Go code and use `managed_shim`, refresh the installed wrapper binary and VS Code bundle entrypoint before testing:

```bash
./install.sh refresh
```

`restart` and `refresh` may interrupt an active VS Code Codex session because they proactively stop the current managed wrapper/app-server/daemon chain before starting again.

Before you test in Feishu:

- make sure the app has the bot message/event permissions from `deploy/feishu/README.md`
- if you want local `.md` links to become Feishu preview links, also grant `drive:drive`

Then in Feishu:

- send `/list`
- reply with the instance number to attach
- use `/threads` to switch thread if needed
- remote execution defaults to full access; if you need confirmation mode temporarily, send `/access confirm`
