package main

import (
	"os"

	app "github.com/kxn/codex-remote-feishu/internal/app/upgradeshim"
)

func main() {
	os.Exit(app.RunMain(os.Args[1:]))
}
