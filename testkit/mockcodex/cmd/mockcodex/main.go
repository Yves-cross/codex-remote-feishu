package main

import (
	"bufio"
	"log"
	"os"

	"github.com/kxn/codex-remote-feishu/testkit/mockcodex"
)

func main() {
	engine := mockcodex.New()
	engine.SeedThread("thread-1", "/data/dl/droid", "修复登录流程")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		outputs, err := engine.HandleRemoteCommand(append(scanner.Bytes(), '\n'))
		if err != nil {
			log.Printf("mockcodex: %v", err)
			continue
		}
		for _, output := range outputs {
			if _, err := os.Stdout.Write(output); err != nil {
				log.Fatal(err)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
}
