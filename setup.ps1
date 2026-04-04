$ErrorActionPreference = "Stop"

$RootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$BinDir = Join-Path $RootDir "bin"
$GoBin = if ($env:GO_BIN) { $env:GO_BIN } else { "go" }

New-Item -ItemType Directory -Force -Path $BinDir | Out-Null

& $GoBin build -o (Join-Path $BinDir "codex-remote-relayd.exe") (Join-Path $RootDir "cmd/relayd")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $GoBin build -o (Join-Path $BinDir "codex-remote-wrapper.exe") (Join-Path $RootDir "cmd/relay-wrapper")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
& $GoBin build -o (Join-Path $BinDir "codex-remote-install.exe") (Join-Path $RootDir "cmd/relay-install")
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$installArgs = @($args)
if ($installArgs.Count -eq 0) {
  $installArgs = @("-interactive")
}

& (Join-Path $BinDir "codex-remote-install.exe") @installArgs
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
