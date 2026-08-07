package agenthostrestart

import (
	"context"
	"flag"
	"fmt"
	"io"
)

// RunCommand is the hidden acceptance command surface used by a detached
// launchd job. It is intentionally not registered in Haft's public CLI.
func RunCommand(
	ctx context.Context,
	args []string,
	stdout io.Writer,
	stderr io.Writer,
) int {
	flags := flag.NewFlagSet("haft-restart-supervisor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	projectRoot := flags.String("project-root", "", "absolute initialized Haft project root")
	taskExecutable := flags.String("task-executable", "", "absolute task executable path")
	quitTimeout := flags.Duration("quit-timeout", 0, "bounded graceful quit/start timeout")
	allowTerm := flags.Bool("allow-term", false, "explicitly allow SIGTERM after graceful quit stalls")
	if err := flags.Parse(args); err != nil {
		return 2
	}
	if *projectRoot == "" || *taskExecutable == "" || *quitTimeout <= 0 {
		_, _ = fmt.Fprintln(stderr, "--project-root, --task-executable, and positive --quit-timeout are required")
		return 2
	}
	store, err := NewStore(*projectRoot)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	checkpoint, err := store.Load()
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	log, err := store.OpenSupervisorLog(checkpoint)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	defer log.Close()
	terminationPolicy := applicationTerminationPolicyForAllowTerm(*allowTerm)
	effects, err := NewCommandEffects(*taskExecutable, terminationPolicy)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	supervisor, err := NewSupervisor(store, effects, log, *quitTimeout)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	supervisor.logEvent("termination_policy", terminationPolicy.String())
	result, err := supervisor.Run(ctx)
	if err != nil {
		supervisor.logEvent("supervisor_failed", err.Error())
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "restart_id=%s state=%s\n", result.RestartID(), result.State().String())
	return 0
}
