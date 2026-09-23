// Command haft10 is the isolated candidate entrypoint for the shared API.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/app"
	"github.com/m0n0x41d/haft/internal/core/carrier"
	"github.com/m0n0x41d/haft/internal/core/host"
	"github.com/m0n0x41d/haft/internal/core/transport"
)

// Version is set by the isolated installer to the candidate commit SHA.
var Version = "development"

const help = `haft10 <operation> [action] [options]

Operations: remember recall context impact fpf source check change recover
  api --input FILE|-          Execute one complete haft.api/1 request
  serve                      Serve the single haft MCP tool on stdio
  init [--codex]              Install project-local instructions and Codex assets
  migrate --from-9x --dry-run ...  Stage an explicit source in a separate output root
  migrate queue|assist ...    Review or record a proposed staging repair
  version                    Print candidate version as JSON

Shared options:
  --root ROOT                Project root (default current directory)
  --source-root ROOT         Offline source checkout
  --source-repository TOKEN  Source repository identity
  --input FILE|-             One JSON request; '-' reads stdin
  --ref REF --query TEXT --limit N --strict

Use api --input for exact CLI/MCP request parity. Other operations fill omitted
format/operation/action fields; conflicting flags or input fields are rejected.
JSON results go to stdout; transport diagnostics go to stderr. Structural check
does not run tests. Check prepare and observe separate capture from execution.
`

func main() { os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
func run(ctx context.Context, args []string, in io.Reader, out, log io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(out, help)
		return 0
	}
	op := args[0]
	args = args[1:]
	if op == "migrate" {
		return runMigration(ctx, args, in, out, log)
	}
	if op == "version" || op == "--version" {
		if len(args) > 0 {
			return cliError(out, log, "invalid_arguments", fmt.Errorf("version takes no arguments"))
		}
		return emit(out, map[string]any{"format": "haft.version/1", "version": Version}, log)
	}
	action := ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		action = args[0]
		args = args[1:]
	}
	f := flag.NewFlagSet("haft10 "+op, flag.ContinueOnError)
	f.SetOutput(log)
	root := f.String("root", ".", "project root")
	sourceRoot := f.String("source-root", "", "offline source checkout")
	sourceRepository := f.String("source-repository", "", "source repository identity")
	input := f.String("input", "", "request JSON path or -")
	ref := f.String("ref", "", "exact reference")
	query := f.String("query", "", "search query")
	limit := f.Int("limit", 0, "maximum result count")
	strict := f.Bool("strict", false, "strict structural validation")
	codex := f.Bool("codex", false, "install project-local Codex assets")
	if err := f.Parse(args); err != nil {
		if err == flag.ErrHelp {
			fmt.Fprint(out, help)
			return 0
		}
		return cliError(out, log, "invalid_arguments", err)
	}
	if f.NArg() != 0 {
		return cliError(out, log, "invalid_arguments", fmt.Errorf("unexpected positional arguments"))
	}
	seen := map[string]bool{}
	f.Visit(func(v *flag.Flag) { seen[v.Name] = true })
	if seen["codex"] && op != "init" {
		return cliError(out, log, "invalid_arguments", fmt.Errorf("--codex requires init"))
	}
	absolute, err := filepath.Abs(*root)
	if err != nil {
		return cliError(out, log, "invalid_root", err)
	}
	s := app.Service{Root: absolute, SourceRoot: *sourceRoot, SourceRepository: *sourceRepository}
	if op == "serve" || op == "init" {
		if action != "" || *input != "" || seen["ref"] || seen["query"] || seen["limit"] || seen["strict"] {
			return cliError(out, log, "invalid_arguments", fmt.Errorf("%s does not accept request fields", op))
		}
		if op == "serve" {
			if err := (transport.Server{Service: s, Version: Version}).Serve(ctx, in, out); err != nil {
				fmt.Fprintln(log, err)
				return 1
			}
			return 0
		}
		binary, err := os.Executable()
		if err != nil {
			return cliError(out, log, "host_init", err)
		}
		binary, err = filepath.EvalSymlinks(binary)
		if err != nil {
			return cliError(out, log, "host_init", err)
		}
		result, err := host.Init(host.Config{Root: absolute, Binary: binary, SourceRoot: *sourceRoot, SourceRepository: *sourceRepository, Codex: *codex})
		if err != nil {
			return cliError(out, log, "host_init", err)
		}
		return emit(out, app.Result{Format: app.Format, Operation: "init", Kind: "initialized", Data: result, Diagnostics: []carrier.Diagnostic{}, Basis: map[string]string{"candidate_version": Version}, Coverage: "complete", Limits: []string{"Project-local delivery only; host execution must be qualified separately"}}, log)
	}
	var request app.Request
	fields := map[string]json.RawMessage{}
	if *input != "" {
		reader := in
		var file *os.File
		if *input != "-" {
			file, err = os.Open(*input)
			if err != nil {
				return cliError(out, log, "input_read", err)
			}
			defer file.Close()
			reader = file
		}
		raw, e := transport.ReadInput(reader)
		if e != nil {
			return cliError(out, log, "input_read", e)
		}
		request, err = transport.DecodeRequest(raw)
		if err == nil {
			err = json.Unmarshal(raw, &fields)
		}
		if err != nil {
			return cliError(out, log, "input_decode", err)
		}
	}
	if op == "api" {
		if *input == "" || action != "" || seen["ref"] || seen["query"] || seen["limit"] || seen["strict"] {
			return cliError(out, log, "invalid_arguments", fmt.Errorf("api requires --input and accepts no request overrides"))
		}
	} else {
		if request.Operation != "" && request.Operation != op {
			return cliError(out, log, "request_conflict", fmt.Errorf("operation differs from JSON input"))
		}
		request.Operation = op
		if request.Format == "" {
			request.Format = app.Format
		}
		if action != "" {
			if request.Action != "" && request.Action != action {
				return cliError(out, log, "request_conflict", fmt.Errorf("action differs from JSON input"))
			}
			request.Action = action
		}
		if seen["ref"] {
			if fields["ref"] != nil && request.Ref != *ref {
				return cliError(out, log, "request_conflict", fmt.Errorf("ref differs from JSON input"))
			}
			request.Ref = *ref
		}
		if seen["query"] {
			if fields["query"] != nil && request.Query != *query {
				return cliError(out, log, "request_conflict", fmt.Errorf("query differs from JSON input"))
			}
			request.Query = *query
		}
		if seen["limit"] {
			if fields["limit"] != nil && request.Limit != *limit {
				return cliError(out, log, "request_conflict", fmt.Errorf("limit differs from JSON input"))
			}
			request.Limit = *limit
		}
		if seen["strict"] {
			if fields["strict"] != nil && request.Strict != *strict {
				return cliError(out, log, "request_conflict", fmt.Errorf("strict differs from JSON input"))
			}
			request.Strict = *strict
		}
	}
	result := s.Execute(ctx, request)
	if exit := emit(out, result, log); exit != 0 {
		return exit
	}
	if result.Failed() {
		return 1
	}
	return 0
}
func emit(out io.Writer, value any, log io.Writer) int {
	if err := json.NewEncoder(out).Encode(value); err != nil {
		fmt.Fprintln(log, err)
		return 1
	}
	return 0
}
func cliError(out, log io.Writer, code string, err error) int {
	fmt.Fprintln(log, "haft10:", err)
	emit(out, app.Result{Format: app.Format, Kind: "invalid", Data: nil, Diagnostics: []carrier.Diagnostic{{Code: code, Message: err.Error(), Severity: "error"}}, Basis: map[string]string{}, Coverage: "unavailable", Limits: []string{}}, log)
	return 2
}
