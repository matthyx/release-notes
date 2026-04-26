// Command release-notes prints release-notes markdown for a kubescape-operator
// helm chart tag. Usage: release-notes <chart-tag>.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/matthyx/release-notes/internal/app"
	"github.com/matthyx/release-notes/internal/gh"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "Usage: release-notes <chart-tag>")
		fmt.Fprintln(os.Stderr, "  Example: GITHUB_TOKEN=... release-notes kubescape-operator-1.30.5")
		os.Exit(2)
	}
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "error: GITHUB_TOKEN environment variable is required")
		os.Exit(1)
	}
	client := gh.New(token)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := app.Run(ctx, os.Stdout, client, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
