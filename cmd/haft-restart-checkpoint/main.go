package main

import (
	"context"
	"os"

	"github.com/m0n0x41d/haft/internal/agenthostrestart"
)

func main() {
	code := agenthostrestart.RunCheckpointCommand(
		context.Background(),
		os.Args[1:],
		os.Stdout,
		os.Stderr,
	)
	os.Exit(code)
}
