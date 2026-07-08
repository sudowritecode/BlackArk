package main

import (
	"fmt"
	"os"

	"github.com/sudowritecode/BlackArk/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "blackark:", err)
		os.Exit(1)
	}
}
