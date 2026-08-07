package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	refreshToolRevisionPrefix = "fpf-refresh-inputs/v1:sha256:"
	refreshToolDigestDomain   = "haft.fpf-refresh.generator-input-closure/v1"
	refreshToolBuildContext   = "haft.fpf-refresh.canonical-go-list/v1\n" +
		"GOOS=linux\nGOARCH=amd64\nGOAMD64=v1\nCGO_ENABLED=1\n" +
		"GOEXPERIMENT=none\nGOFLAGS=\nGOWORK=off\nGOENV=off\n" +
		"GOTOOLCHAIN=auto\nGO111MODULE=on\n-mod=readonly\n-tags="
)

var refreshToolRuntimeInputs = []string{
	"go.mod",
	"go.sum",
	"scripts/fpf_query_token_gate.sh",
	"scripts/fpf_query_token_count.py",
	"scripts/fpf_query_token_count.requirements.txt",
}

var refreshToolCommandRoots = []string{
	"./cmd/fpf-refresh",
	"./cmd/indexer",
}

type refreshToolGoListError struct {
	Err string
}

type refreshToolGoListPackage struct {
	Dir          string
	ImportPath   string
	Standard     bool
	GoFiles      []string
	CgoFiles     []string
	CFiles       []string
	CXXFiles     []string
	MFiles       []string
	HFiles       []string
	FFiles       []string
	SFiles       []string
	SwigFiles    []string
	SwigCXXFiles []string
	SysoFiles    []string
	EmbedFiles   []string
	Incomplete   bool
	Error        *refreshToolGoListError
	DepsErrors   []refreshToolGoListError
}

type refreshToolInput struct {
	logicalName string
	path        string
	size        int64
	digest      [sha256.Size]byte
}

func exactRefreshToolRevision(ctx context.Context, root string) (string, error) {
	inputs, err := listRefreshToolInputs(ctx, root)
	if err != nil {
		return "", err
	}
	return digestRefreshToolInputs(inputs), nil
}

func listRefreshToolInputs(
	ctx context.Context,
	root string,
) (map[string]refreshToolInput, error) {
	command := exec.CommandContext(
		ctx,
		"go",
		"list",
		"-mod=readonly",
		"-tags=",
		"-deps",
		"-json=Dir,ImportPath,Standard,GoFiles,CgoFiles,CFiles,CXXFiles,MFiles,HFiles,FFiles,SFiles,SwigFiles,SwigCXXFiles,SysoFiles,EmbedFiles,Incomplete,Error,DepsErrors",
		refreshToolCommandRoots[0],
		refreshToolCommandRoots[1],
	)
	command.Dir = root
	command.Env = canonicalRefreshToolGoListEnvironment(os.Environ())
	output, err := command.Output()
	if err != nil {
		detail := ""
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			detail = strings.TrimSpace(string(exitErr.Stderr))
		}
		if detail != "" {
			return nil, fmt.Errorf("list canonical refresh-tool build inputs: %w: %s", err, detail)
		}
		return nil, fmt.Errorf("list canonical refresh-tool build inputs: %w", err)
	}

	inputs := make(map[string]refreshToolInput)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for {
		var pkg refreshToolGoListPackage
		if err := decoder.Decode(&pkg); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, fmt.Errorf("decode canonical refresh-tool package metadata: %w", err)
		}
		if pkg.Incomplete || pkg.Error != nil || len(pkg.DepsErrors) != 0 {
			return nil, fmt.Errorf(
				"canonical refresh-tool package %q is incomplete: %s",
				pkg.ImportPath,
				refreshToolPackageErrorDetail(pkg),
			)
		}
		if pkg.Standard {
			continue
		}
		if pkg.ImportPath == "" || pkg.Dir == "" {
			return nil, fmt.Errorf("canonical refresh-tool package metadata is missing identity or directory")
		}
		for _, name := range refreshToolPackageBuildFiles(pkg) {
			clean, err := canonicalRefreshToolInputName(name)
			if err != nil {
				return nil, fmt.Errorf("package %s build input: %w", pkg.ImportPath, err)
			}
			logicalName := "go/" + pkg.ImportPath + "/" + filepath.ToSlash(clean)
			if err := addRefreshToolInput(
				inputs,
				logicalName,
				filepath.Join(pkg.Dir, clean),
			); err != nil {
				return nil, err
			}
		}
	}

	for _, relative := range refreshToolRuntimeInputs {
		clean, err := canonicalRefreshToolInputName(filepath.FromSlash(relative))
		if err != nil {
			return nil, fmt.Errorf("runtime input: %w", err)
		}
		if err := addRefreshToolInput(
			inputs,
			"runtime/"+filepath.ToSlash(clean),
			filepath.Join(root, clean),
		); err != nil {
			return nil, err
		}
	}
	return inputs, nil
}

func refreshToolPackageBuildFiles(pkg refreshToolGoListPackage) []string {
	fields := [][]string{
		pkg.GoFiles,
		pkg.CgoFiles,
		pkg.CFiles,
		pkg.CXXFiles,
		pkg.MFiles,
		pkg.HFiles,
		pkg.FFiles,
		pkg.SFiles,
		pkg.SwigFiles,
		pkg.SwigCXXFiles,
		pkg.SysoFiles,
		pkg.EmbedFiles,
	}
	var files []string
	for _, field := range fields {
		files = append(files, field...)
	}
	return files
}

func canonicalRefreshToolInputName(name string) (string, error) {
	if name == "" || filepath.IsAbs(name) {
		return "", fmt.Errorf("input name %q must be non-empty and relative", name)
	}
	clean := filepath.Clean(name)
	if clean == "." || clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("input name %q escapes its package or repository", name)
	}
	return clean, nil
}

func addRefreshToolInput(
	inputs map[string]refreshToolInput,
	logicalName string,
	path string,
) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read refresh-tool input %q: %w", logicalName, err)
	}
	candidate := refreshToolInput{
		logicalName: logicalName,
		path:        path,
		size:        int64(len(content)),
		digest:      sha256.Sum256(content),
	}
	previous, exists := inputs[logicalName]
	if !exists {
		inputs[logicalName] = candidate
		return nil
	}
	if previous.size == candidate.size && previous.digest == candidate.digest {
		previousContent, readErr := os.ReadFile(previous.path)
		if readErr != nil {
			return fmt.Errorf("reread duplicate refresh-tool input %q: %w", logicalName, readErr)
		}
		if bytes.Equal(previousContent, content) {
			return nil
		}
	}
	return fmt.Errorf(
		"duplicate logical refresh-tool input %q resolves to different bytes",
		logicalName,
	)
}

func digestRefreshToolInputs(inputs map[string]refreshToolInput) string {
	digest := sha256.New()
	writeRefreshToolDigestRecord(digest, "domain", refreshToolDigestDomain, nil)
	writeRefreshToolDigestRecord(
		digest,
		"canonical-build-context",
		refreshToolBuildContext,
		nil,
	)
	for _, root := range refreshToolCommandRoots {
		writeRefreshToolDigestRecord(digest, "command-root", root, nil)
	}
	logicalNames := make([]string, 0, len(inputs))
	for logicalName := range inputs {
		logicalNames = append(logicalNames, logicalName)
	}
	sort.Strings(logicalNames)
	for _, logicalName := range logicalNames {
		input := inputs[logicalName]
		writeRefreshToolDigestRecord(
			digest,
			"content-input",
			input.logicalName,
			input.digest[:],
		)
	}
	return refreshToolRevisionPrefix + hex.EncodeToString(digest.Sum(nil))
}

func writeRefreshToolDigestRecord(
	digest hash.Hash,
	kind string,
	logicalName string,
	payload []byte,
) {
	for _, field := range [][]byte{[]byte(kind), []byte(logicalName), payload} {
		var length [8]byte
		binary.BigEndian.PutUint64(length[:], uint64(len(field)))
		_, _ = digest.Write(length[:])
		_, _ = digest.Write(field)
	}
}

func canonicalRefreshToolGoListEnvironment(environ []string) []string {
	canonical := map[string]string{
		"CGO_ENABLED":  "1",
		"GO111MODULE":  "on",
		"GOAMD64":      "v1",
		"GOARCH":       "amd64",
		"GOENV":        "off",
		"GOEXPERIMENT": "none",
		"GOFLAGS":      "",
		"GOOS":         "linux",
		"GOTOOLCHAIN":  "auto",
		"GOWORK":       "off",
	}
	result := make([]string, 0, len(environ)+len(canonical))
	for _, entry := range environ {
		name, _, found := strings.Cut(entry, "=")
		if found {
			if _, overridden := canonical[name]; overridden {
				continue
			}
		}
		result = append(result, entry)
	}
	names := make([]string, 0, len(canonical))
	for name := range canonical {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result = append(result, name+"="+canonical[name])
	}
	return result
}

func refreshToolPackageErrorDetail(pkg refreshToolGoListPackage) string {
	details := make([]string, 0, 1+len(pkg.DepsErrors))
	if pkg.Error != nil && strings.TrimSpace(pkg.Error.Err) != "" {
		details = append(details, strings.TrimSpace(pkg.Error.Err))
	}
	for _, dependencyErr := range pkg.DepsErrors {
		if detail := strings.TrimSpace(dependencyErr.Err); detail != "" {
			details = append(details, detail)
		}
	}
	if len(details) == 0 {
		return "go list reported incomplete metadata"
	}
	return strings.Join(details, "; ")
}

func verifyRefreshToolRevision(
	ctx context.Context,
	root string,
	expected string,
) error {
	observed, err := exactRefreshToolRevision(ctx, root)
	if err != nil {
		return err
	}
	if observed != expected {
		return fmt.Errorf(
			"refresh implementation changed during candidate evaluation: started %s, observed %s",
			expected,
			observed,
		)
	}
	return nil
}
