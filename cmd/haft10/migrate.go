package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/app"
	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/migrate"
	"github.com/m0n0x41d/haft/internal/core/transport"
)

func runMigration(ctx context.Context, args []string, in io.Reader, out, log io.Writer) int {
	action := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}
	f := flag.NewFlagSet("haft10 migrate", flag.ContinueOnError)
	f.SetOutput(log)
	input := f.String("input", "", "explicit migration request JSON or -")
	db := f.String("database", "", "source database path")
	carriers := f.String("carrier-root", "", "source carrier directory")
	output := f.String("output-root", "", "separate staging project root")
	created := f.String("created-at", "", "explicit RFC3339 timestamp")
	id := f.String("request-id", "", "stable idempotency key")
	dry := f.Bool("dry-run", false, "stage only, required")
	from9 := f.Bool("from-9x", false, "read the bounded v9 source decoder")
	rows := f.Int("max-rows", 0, "capture row limit")
	size := f.Int64("max-bytes", 0, "capture byte limit")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return cliError(out, log, "invalid_arguments", err)
	}
	if f.NArg() != 0 {
		return cliError(out, log, "invalid_arguments", fmt.Errorf("unexpected positional arguments"))
	}
	seen := map[string]bool{}
	f.Visit(func(v *flag.Flag) { seen[v.Name] = true })
	if seen["from-9x"] && !*from9 {
		return cliError(out, log, "invalid_arguments", fmt.Errorf("B1 supports only --from-9x"))
	}
	var raw []byte
	if *input != "" {
		reader := in
		if *input != "-" {
			file, err := os.Open(*input)
			if err != nil {
				return cliError(out, log, "input_read", err)
			}
			defer file.Close()
			reader = file
		}
		var err error
		raw, err = transport.ReadInput(reader)
		if err != nil {
			return cliError(out, log, "input_read", err)
		}
	}
	result := app.Result{Format: app.Format, Operation: "migrate", Diagnostics: []carrier.Diagnostic{}, Basis: map[string]string{}, Coverage: "bounded", Limits: []string{"Separate staging only; source records retain their original status and limitations"}}
	switch action {
	case "assist":
		if *input == "" || len(seen) != 1 {
			return cliError(out, log, "invalid_arguments", fmt.Errorf("migrate assist requires --input only"))
		}
		var q migrate.AssistanceRequest
		if err := transport.Decode(raw, &q); err != nil {
			return cliError(out, log, "input_decode", err)
		}
		r, err := migrate.Assist(ctx, q)
		if err != nil {
			return cliError(out, log, "migration_assist", err)
		}
		result.Kind = r.Kind
		result.Data = r
	case "queue":
		if *output == "" || len(seen) != 1 {
			return cliError(out, log, "invalid_arguments", fmt.Errorf("migrate queue requires --output-root only"))
		}
		r, err := migrate.Queue(ctx, *output)
		if err != nil {
			return cliError(out, log, "migration_queue", err)
		}
		result.Kind = "results"
		result.Data = r
	case "":
		var q migrate.Request
		if *input != "" {
			allowed := 1
			if seen["from-9x"] {
				allowed++
			}
			if len(seen) != allowed {
				return cliError(out, log, "invalid_arguments", fmt.Errorf("migration JSON does not accept flag overrides"))
			}
			if err := transport.Decode(raw, &q); err != nil {
				return cliError(out, log, "input_decode", err)
			}
		} else {
			q = migrate.Request{DatabasePath: *db, CarrierRoot: *carriers, OutputRoot: *output, CreatedAt: *created, RequestID: *id, DryRun: *dry, MaxRows: *rows, MaxBytes: *size}
		}
		r, err := migrate.Run(ctx, q)
		result.Kind = r.Kind
		result.Data = r
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, carrier.Diagnostic{Code: "migration_failed", Message: err.Error(), Severity: "error"})
			fmt.Fprintln(log, "haft10:", err)
			emit(out, result, log)
			return 1
		}
		if r.Report.SourceDigest != "" {
			result.Basis["migration_source"] = r.Report.SourceDigest
		}
	default:
		return cliError(out, log, "invalid_arguments", fmt.Errorf("migration action is queue, assist or omitted"))
	}
	if code := emit(out, result, log); code != 0 {
		return code
	}
	if result.Failed() {
		return 1
	}
	return 0
}
