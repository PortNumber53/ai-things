package main

import (
	"os"

	"ai-things/manager-go/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args))
}
