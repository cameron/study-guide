package main

import (
	"os"

	"study-guide/src/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:]))
}
