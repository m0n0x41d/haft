package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	methodpkg "github.com/m0n0x41d/haft/internal/method"
)

var (
	methodCheckpointOpenJSON        bool
	methodCheckpointOpenTargetRef   string
	methodCheckpointOpenCheckRef    string
	methodCheckpointOpenDigest      string
	methodCheckpointOpenTTLMinutes  int
	methodCheckpointCloseJSON       bool
	methodCheckpointCloseOutcome    string
	methodCheckpointCloseObsRefs    []string
	methodCheckpointCloseDigest     string
	methodCheckpointCloseNextTarget string
	methodCheckpointTraceJSON       bool
)

var methodCheckpointCmd = &cobra.Command{
	Use:   "checkpoint",
	Short: "Manage MethodRun attention checkpoints",
	Long: `Open, close, and trace MethodRun checkpoints.

Checkpoints are attention/process telemetry only. They do not prove
correctness, create evidence truth, pass gates, or authorize work.`,
}

var methodCheckpointOpenCmd = &cobra.Command{
	Use:   "open RUN_ID",
	Short: "Open a MethodRun checkpoint",
	Args:  cobra.ExactArgs(1),
	RunE:  runMethodCheckpointOpen,
}

var methodCheckpointCloseCmd = &cobra.Command{
	Use:   "close CLOSE_TOKEN",
	Short: "Close a MethodRun checkpoint by token",
	Args:  cobra.ExactArgs(1),
	RunE:  runMethodCheckpointClose,
}

var methodCheckpointTraceCmd = &cobra.Command{
	Use:   "trace RUN_ID",
	Short: "Trace MethodRun checkpoint records",
	Args:  cobra.ExactArgs(1),
	RunE:  runMethodCheckpointTrace,
}

func init() {
	methodCheckpointOpenCmd.Flags().BoolVar(&methodCheckpointOpenJSON, "json", false, "print structured JSON output")
	methodCheckpointOpenCmd.Flags().StringVar(&methodCheckpointOpenTargetRef, "target-ref", "", "concrete target being checked")
	methodCheckpointOpenCmd.Flags().StringVar(&methodCheckpointOpenCheckRef, "check-ref", "", "check or method step reference")
	methodCheckpointOpenCmd.Flags().StringVar(&methodCheckpointOpenDigest, "target-digest", "", "digest of the target state at checkpoint open")
	methodCheckpointOpenCmd.Flags().IntVar(&methodCheckpointOpenTTLMinutes, "ttl-minutes", int(methodpkg.DefaultCheckpointTTL.Minutes()), "close-token lifetime in minutes")

	methodCheckpointCloseCmd.Flags().BoolVar(&methodCheckpointCloseJSON, "json", false, "print structured JSON output")
	methodCheckpointCloseCmd.Flags().StringVar(&methodCheckpointCloseOutcome, "outcome", "", "checkpoint outcome")
	methodCheckpointCloseCmd.Flags().StringArrayVar(&methodCheckpointCloseObsRefs, "observation-ref", nil, "observation reference; repeatable")
	methodCheckpointCloseCmd.Flags().StringVar(&methodCheckpointCloseDigest, "resulting-digest", "", "digest of the target state at checkpoint close")
	methodCheckpointCloseCmd.Flags().StringVar(&methodCheckpointCloseNextTarget, "next-target-ref", "", "next target reference, if work moved")

	methodCheckpointTraceCmd.Flags().BoolVar(&methodCheckpointTraceJSON, "json", false, "print structured JSON output")

	methodCheckpointCmd.AddCommand(methodCheckpointOpenCmd)
	methodCheckpointCmd.AddCommand(methodCheckpointCloseCmd)
	methodCheckpointCmd.AddCommand(methodCheckpointTraceCmd)
	methodCmd.AddCommand(methodCheckpointCmd)
}

func runMethodCheckpointOpen(cmd *cobra.Command, args []string) error {
	store, haftDir, closeFn, err := openMethodCommandStore()
	if err != nil {
		return err
	}
	defer closeFn()

	result, err := methodpkg.OpenCheckpoint(context.Background(), store, haftDir, methodpkg.CheckpointOpenInput{
		RunID:        args[0],
		TargetRef:    methodCheckpointOpenTargetRef,
		CheckRef:     methodCheckpointOpenCheckRef,
		TargetDigest: methodCheckpointOpenDigest,
		TTL:          time.Duration(methodCheckpointOpenTTLMinutes) * time.Minute,
	})
	if err != nil {
		return err
	}
	if methodCheckpointOpenJSON {
		return writeMethodCheckpointJSON(cmd.OutOrStdout(), result)
	}
	return writeMethodCheckpointOpenText(cmd.OutOrStdout(), result)
}

func runMethodCheckpointClose(cmd *cobra.Command, args []string) error {
	store, haftDir, closeFn, err := openMethodCommandStore()
	if err != nil {
		return err
	}
	defer closeFn()

	result, err := methodpkg.CloseCheckpoint(context.Background(), store, haftDir, methodpkg.CheckpointCloseInput{
		CloseToken:      args[0],
		Outcome:         methodCheckpointCloseOutcome,
		ObservationRefs: methodCheckpointCloseObsRefs,
		ResultingDigest: methodCheckpointCloseDigest,
		NextTargetRef:   methodCheckpointCloseNextTarget,
	})
	if err != nil {
		return err
	}
	if methodCheckpointCloseJSON {
		return writeMethodCheckpointJSON(cmd.OutOrStdout(), result)
	}
	return writeMethodCheckpointCloseText(cmd.OutOrStdout(), result)
}

func runMethodCheckpointTrace(cmd *cobra.Command, args []string) error {
	store, _, closeFn, err := openMethodCommandStore()
	if err != nil {
		return err
	}
	defer closeFn()

	report, err := methodpkg.TraceCheckpoints(context.Background(), store, args[0])
	if err != nil {
		return err
	}
	if methodCheckpointTraceJSON {
		return writeMethodCheckpointJSON(cmd.OutOrStdout(), report)
	}
	return writeMethodCheckpointTraceText(cmd.OutOrStdout(), report)
}

func openMethodCommandStore() (*artifact.Store, string, func(), error) {
	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, "", nil, err
	}
	store, closeFn, err := openArtifactStore(projectRoot)
	if err != nil {
		return nil, "", nil, err
	}
	return store, haftDirFor(projectRoot), closeFn, nil
}

func writeMethodCheckpointJSON(w io.Writer, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}

func writeMethodCheckpointOpenText(w io.Writer, result methodpkg.CheckpointResult) error {
	_, err := fmt.Fprintf(
		w,
		"Checkpoint opened: run=%s checkpoint=%s token=%s expires=%s\n",
		result.RunID,
		result.CheckpointID,
		result.CloseToken,
		result.Record.ExpiresAt,
	)
	return err
}

func writeMethodCheckpointCloseText(w io.Writer, result methodpkg.CheckpointResult) error {
	_, err := fmt.Fprintf(
		w,
		"Checkpoint closed: run=%s checkpoint=%s outcome=%s\n",
		result.RunID,
		result.CheckpointID,
		result.Record.Outcome,
	)
	return err
}

func writeMethodCheckpointTraceText(w io.Writer, report methodpkg.CheckpointTraceReport) error {
	if _, err := fmt.Fprintf(w, "Method checkpoint trace run=%s records=%d open=%d closed=%d expired=%d\n",
		report.RunID,
		report.Summary.Records,
		report.Summary.Open,
		report.Summary.Closed,
		report.Summary.Expired,
	); err != nil {
		return err
	}
	for _, state := range report.States {
		if _, err := fmt.Fprintf(w, "- %s %s target=%s check=%s\n",
			state.Status,
			state.CheckpointID,
			state.TargetRef,
			state.CheckRef,
		); err != nil {
			return err
		}
	}
	if len(report.States) == 0 {
		if _, err := fmt.Fprintln(w, "No checkpoints recorded."); err != nil {
			return err
		}
	}
	if !strings.Contains(report.AuthorityBoundary, "not_evidence") {
		_, err := fmt.Fprintln(w, "authority="+report.AuthorityBoundary)
		return err
	}
	return nil
}
