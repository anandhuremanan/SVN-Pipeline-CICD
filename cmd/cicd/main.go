package main

import (
	"context"
	"fmt"
	"os"

	"github.com/anandhuremanan/cicd-engine/internal/api"
	"github.com/anandhuremanan/cicd-engine/internal/cli"
	"github.com/anandhuremanan/cicd-engine/internal/service"
)

func main() {
	pipeSvc := service.NewDefaultPipelineService(nil, nil, nil, nil, os.Stdout)
	apiServer := api.NewServer(pipeSvc, "")

	app := cli.NewApp(os.Stdout, os.Stderr, pipeSvc)
	app.ServerRunner = func(ctx context.Context, addr string) error {
		return apiServer.ListenAndServeContext(ctx, addr)
	}

	if err := app.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
