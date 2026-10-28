package main

import (
	"context"
	"os"

	"github.com/codewithcheese/toggle/internal/toggle"
)

func main() {
	os.Exit(toggle.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr))
}
