package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println("blackark-agent dev")
		return
	}
	fmt.Println("blackark-agent: runtime reconciliation is configured in the next MVP phase")
}
