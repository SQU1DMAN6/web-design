package main

import (
	"fmt"
	"os"
	_ "ftr/cmd/all" // Import all commands
	"ftr/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}