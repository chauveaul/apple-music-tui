package main

import (
	"fmt"
	"os"

	"main/tui"
)

func main() {
	if err := tui.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}
