package main

import (
	"fmt"
	"os"

	"github.com/ssemakov/lkmcu/labs/audio/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "\naudiolab:", err)
		os.Exit(1)
	}
}
