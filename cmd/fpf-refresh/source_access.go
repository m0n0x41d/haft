package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/m0n0x41d/haft/internal/fpfrefresh"
)

func runSourceAccess(ctx context.Context, mode string, args []string, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet(mode, flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("repo", ".", "Haft repository containing the unchanged memory database")
	source := flags.String("source-repo", "", "upstream Git object repository (defaults to data/FPF; checkout never moved)")
	revision := flags.String("candidate-ref", "", "candidate to resolve once (check/apply default origin/main; verify uses archive pin and treats this as an expected revision)")
	noFetch := true
	remote := "origin"
	reportPath := "-"
	if mode != "source-verify" {
		flags.BoolVar(&noFetch, "no-fetch", false, "use already fetched objects without network or Git ref writes")
		flags.StringVar(&remote, "fetch-remote", "origin", "upstream remote to fetch before pinning the candidate")
		flags.StringVar(&reportPath, "report", "-", "optional source report copy (default '-' prints only to stdout; distinct from the integrated memory report)")
	}
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("source access accepts no positional arguments")
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(*root)
	if err != nil {
		return 1, err
	}
	if reportPath != "-" {
		reportPath, err = filepath.Abs(reportPath)
		if err != nil {
			return 1, err
		}
		if err := fpfrefresh.ValidateReportPath(layout, reportPath); err != nil {
			return 1, err
		}
		if reportPath == layout.Report {
			return 2, fmt.Errorf("source report must not replace the integrated memory report; use latest-source-report.json")
		}
	}
	sourcePath := *source
	if sourcePath == "" {
		sourcePath = layout.SourceRepository
	}
	request := fpfrefresh.SourceAccessRequest{
		Source:             fpfrefresh.GitSourceRequest{RepositoryPath: sourcePath, CandidateRef: *revision},
		MemoryDatabasePath: layout.Database,
		ArchivePath:        filepath.Join(layout.Root, fpfrefresh.SourceAccessArchiveRelativePath),
	}
	if !noFetch {
		request.Source.Fetch = &fpfrefresh.GitFetchRequest{Remote: remote}
	}
	if mode != "source-verify" && request.Source.CandidateRef == "" {
		request.Source.CandidateRef = fpfrefresh.DefaultCandidateRef
	}
	report, err := executeSourceAccess(ctx, mode, request)
	if err != nil {
		return 1, err
	}
	if err := writeSourceAccessWarning(stderr, report); err != nil {
		return 1, err
	}
	if reportPath != "-" {
		if err := writeSourceAccessReport(reportPath, report); err != nil {
			return 1, err
		}
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return 1, err
	}
	return 0, nil
}

func writeSourceAccessReport(path string, report fpfrefresh.SourceAccessReport) error {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0755); err != nil {
		return err
	}
	payload = append(payload, '\n')
	return os.WriteFile(path, payload, 0644)
}

func executeSourceAccess(ctx context.Context, mode string, request fpfrefresh.SourceAccessRequest) (fpfrefresh.SourceAccessReport, error) {
	if mode == "source-verify" {
		return fpfrefresh.VerifySourceAccess(ctx, request)
	}
	report, archive, err := fpfrefresh.PrepareSourceAccess(ctx, request)
	if err != nil {
		return fpfrefresh.SourceAccessReport{}, err
	}
	if mode == "source-check" {
		return report, nil
	}
	if err := fpfrefresh.ApplySourceAccess(request, report, archive); err != nil {
		return fpfrefresh.SourceAccessReport{}, err
	}
	// Verify exactly the prepared candidate. A second fetch or origin/main
	// lookup here could accidentally verify a different upstream commit.
	request.Source = fpfrefresh.GitSourceRequest{
		RepositoryPath: request.Source.RepositoryPath,
		CandidateRef:   report.SourceRevision,
	}
	return fpfrefresh.VerifySourceAccess(ctx, request)
}

func writeSourceAccessWarning(writer io.Writer, report fpfrefresh.SourceAccessReport) error {
	_, err := fmt.Fprintf(writer, "FPF SOURCE/MEMORY: source %s; memory remains %s; compiler %s; no memory successor activated.\n", report.SourceRevision, report.Basis.MemorySourceRevision, report.Basis.CandidateCompilation)
	if err != nil {
		return err
	}
	var diagnostics []struct {
		Code    string `json:"code"`
		UnitID  string `json:"unit_id"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(report.Basis.CompilerDiagnostics, &diagnostics); err != nil {
		return err
	}
	for _, diagnostic := range diagnostics {
		if _, err := fmt.Fprintf(writer, "%s[%s]: %s\n", diagnostic.Code, diagnostic.UnitID, diagnostic.Message); err != nil {
			return err
		}
	}
	return nil
}

func runSourceFetch(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	flags := flag.NewFlagSet("source-fetch", flag.ContinueOnError)
	flags.SetOutput(stderr)
	root := flags.String("repo", ".", "Haft repository containing the pinned source archive")
	source := flags.String("source-repo", "", "Git repository receiving the pinned objects (defaults to data/FPF)")
	remote := flags.String("fetch-remote", "origin", "remote supplying the archive's exact commit")
	if err := flags.Parse(args); err != nil {
		return 2, err
	}
	if flags.NArg() != 0 {
		return 2, fmt.Errorf("source-fetch accepts no positional arguments")
	}
	layout, err := fpfrefresh.ResolveRepositoryLayout(*root)
	if err != nil {
		return 1, err
	}
	sourcePath := *source
	if sourcePath == "" {
		sourcePath = layout.SourceRepository
	}
	archivePath := filepath.Join(layout.Root, fpfrefresh.SourceAccessArchiveRelativePath)
	revision, err := fpfrefresh.FetchSourceAccessObjects(ctx, archivePath, sourcePath, *remote)
	if err != nil {
		return 1, err
	}
	_, err = fmt.Fprintf(stdout, "Fetched pinned source objects %s; checkout unchanged.\n", revision)
	return 0, err
}
