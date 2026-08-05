package main

import (
	"context"
	"fmt"
	"os"

	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/engine"
)

func main() {
	eng := engine.NewEngine(os.Stdout, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	app := cli.NewApp(os.Stdout, os.Stderr, eng)

	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
