//go:build !windows

package embedding

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	sidecarStateActive       = "active"
	sidecarStateProcessOnly  = "process_only"
	sidecarStateStaleSocket  = "stale_socket"
	sidecarStateStaleLock    = "stale_lock"
	sidecarStateStdioProcess = "stdio_process"
)

type sidecarProcessStatus struct {
	PID             int
	PPID            int
	RSSKB           int64
	VSZKB           int64
	Command         string
	ServeSocket     string
	Model           string
	CacheDir        string
	RequestedDim    int
	DimLabel        string
	IdleTimeoutSecs int
}

func LoadSidecarStatus(ctx context.Context, options SidecarStatusOptions) (SidecarStatusReport, error) {
	socketDir := sharedSidecarStatusDir()
	processes, processErr := loadSidecarProcesses(ctx)

	report := buildSidecarStatus(socketDir, processes)
	report.SharedEnabled = sharedSidecarEnabled()
	report.SocketDir = socketDir
	report.Binary, report.BinaryFound = locateSidecar()

	if processErr != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("process scan failed: %v", processErr))
	}
	if options.IncludeFootprint {
		addSidecarFootprints(ctx, report.Entries)
	}
	return report, nil
}

func sharedSidecarStatusDir() string {
	override := strings.TrimSpace(os.Getenv(sharedSocketDirEnv))
	if override != "" {
		return override
	}
	parent := filepath.Join("/tmp", fmt.Sprintf("haft-%d", os.Getuid()))
	return filepath.Join(parent, "embed")
}

func buildSidecarStatus(socketDir string, processes []sidecarProcessStatus) SidecarStatusReport {
	entries := map[string]*SidecarStatusEntry{}
	warnings := []string{}

	fileEntries, fileWarnings := sidecarSocketFileEntries(socketDir)
	warnings = append(warnings, fileWarnings...)
	for key, entry := range fileEntries {
		entries[key] = entry
	}

	for _, process := range processes {
		if process.ServeSocket == "" {
			key := fmt.Sprintf("stdio-%d", process.PID)
			entry := &SidecarStatusEntry{Key: key, State: sidecarStateStdioProcess}
			applySidecarProcess(entry, process)
			entries[key] = entry
			continue
		}

		key := sidecarKeyFromSocket(process.ServeSocket)
		entry := entries[key]
		if entry == nil {
			entry = &SidecarStatusEntry{Key: key, SocketPath: process.ServeSocket}
			entry.SocketExists = pathExists(process.ServeSocket)
			entries[key] = entry
		}
		if entry.SocketPath == "" {
			entry.SocketPath = process.ServeSocket
			entry.SocketExists = pathExists(process.ServeSocket)
		}
		applySidecarProcess(entry, process)
	}

	out := make([]SidecarStatusEntry, 0, len(entries))
	for _, entry := range entries {
		entry.State = sidecarEntryState(entry)
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		left := sidecarStatusSortKey(out[i])
		right := sidecarStatusSortKey(out[j])
		return left < right
	})

	return SidecarStatusReport{Entries: out, Warnings: warnings}
}

func sidecarSocketFileEntries(socketDir string) (map[string]*SidecarStatusEntry, []string) {
	entries := map[string]*SidecarStatusEntry{}
	dirEntries, err := os.ReadDir(socketDir)
	if err != nil {
		if os.IsNotExist(err) {
			return entries, nil
		}
		return entries, []string{fmt.Sprintf("read socket dir %s: %v", socketDir, err)}
	}

	for _, dirEntry := range dirEntries {
		name := dirEntry.Name()
		key, kind, ok := sidecarSocketArtifact(name)
		if !ok {
			continue
		}
		entry := entries[key]
		if entry == nil {
			entry = &SidecarStatusEntry{Key: key}
			entries[key] = entry
		}
		path := filepath.Join(socketDir, name)
		switch kind {
		case "sock":
			entry.SocketPath = path
			entry.SocketExists = true
		case "lock":
			entry.LockPath = path
			entry.LockExists = true
		}
	}

	return entries, nil
}

func sidecarSocketArtifact(name string) (string, string, bool) {
	if strings.HasPrefix(name, "embed-") && strings.HasSuffix(name, ".sock") {
		key := strings.TrimSuffix(name, ".sock")
		return key, "sock", true
	}
	if strings.HasPrefix(name, "embed-") && strings.HasSuffix(name, ".lock") {
		key := strings.TrimSuffix(name, ".lock")
		return key, "lock", true
	}
	return "", "", false
}

func sidecarKeyFromSocket(socketPath string) string {
	base := filepath.Base(socketPath)
	key := strings.TrimSuffix(base, ".sock")
	if key != "" {
		return key
	}
	return socketPath
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func applySidecarProcess(entry *SidecarStatusEntry, process sidecarProcessStatus) {
	entry.PID = process.PID
	entry.PPID = process.PPID
	entry.Model = process.Model
	entry.CacheDir = process.CacheDir
	entry.RequestedDim = process.RequestedDim
	entry.DimLabel = process.DimLabel
	entry.IdleTimeoutSecs = process.IdleTimeoutSecs
	entry.RSSKB = process.RSSKB
	entry.VSZKB = process.VSZKB
	entry.Command = process.Command
}

func sidecarEntryState(entry *SidecarStatusEntry) string {
	if entry.PID > 0 && entry.SocketExists {
		return sidecarStateActive
	}
	if entry.PID > 0 && entry.SocketPath == "" {
		return sidecarStateStdioProcess
	}
	if entry.PID > 0 {
		return sidecarStateProcessOnly
	}
	if entry.SocketExists {
		return sidecarStateStaleSocket
	}
	return sidecarStateStaleLock
}

func sidecarStatusSortKey(entry SidecarStatusEntry) string {
	stateRank := map[string]string{
		sidecarStateActive:       "0",
		sidecarStateProcessOnly:  "1",
		sidecarStateStdioProcess: "2",
		sidecarStateStaleSocket:  "3",
		sidecarStateStaleLock:    "4",
	}
	rank := stateRank[entry.State]
	if rank == "" {
		rank = "9"
	}
	return rank + ":" + entry.Key
}

func loadSidecarProcesses(ctx context.Context) ([]sidecarProcessStatus, error) {
	cmd := exec.CommandContext(ctx, "ps", "-axo", "pid=,ppid=,rss=,vsz=,command=")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return parseSidecarProcesses(string(output)), nil
}

func parseSidecarProcesses(output string) []sidecarProcessStatus {
	processes := []sidecarProcessStatus{}
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		process, ok := parseSidecarProcessLine(line)
		if ok {
			processes = append(processes, process)
		}
	}
	return processes
}

func parseSidecarProcessLine(line string) (sidecarProcessStatus, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return sidecarProcessStatus{}, false
	}
	executable := fields[4]
	if filepath.Base(executable) != sidecarBinaryName {
		return sidecarProcessStatus{}, false
	}

	pid, pidErr := strconv.Atoi(fields[0])
	ppid, ppidErr := strconv.Atoi(fields[1])
	rssKB, rssErr := strconv.ParseInt(fields[2], 10, 64)
	vszKB, vszErr := strconv.ParseInt(fields[3], 10, 64)
	if pidErr != nil || ppidErr != nil || rssErr != nil || vszErr != nil {
		return sidecarProcessStatus{}, false
	}

	command := strings.Join(fields[4:], " ")
	process := sidecarProcessStatus{
		PID:      pid,
		PPID:     ppid,
		RSSKB:    rssKB,
		VSZKB:    vszKB,
		Command:  command,
		DimLabel: "native",
	}
	applySidecarArgs(&process, fields[5:])
	return process, true
}

func applySidecarArgs(process *sidecarProcessStatus, args []string) {
	for index := 0; index < len(args); index++ {
		name := args[index]
		value, ok := sidecarFlagValue(args, index)
		if !ok {
			continue
		}
		switch name {
		case "--model":
			process.Model = value
		case "--cache-dir":
			process.CacheDir = value
		case "--dim":
			dim, err := strconv.Atoi(value)
			if err == nil {
				process.RequestedDim = dim
				process.DimLabel = value
			}
		case "--serve-socket":
			process.ServeSocket = value
		case "--idle-timeout-secs":
			seconds, err := strconv.Atoi(value)
			if err == nil {
				process.IdleTimeoutSecs = seconds
			}
		}
	}
}

func sidecarFlagValue(args []string, index int) (string, bool) {
	if index+1 >= len(args) {
		return "", false
	}
	value := strings.TrimSpace(args[index+1])
	return value, value != ""
}

func addSidecarFootprints(ctx context.Context, entries []SidecarStatusEntry) {
	for index := range entries {
		if entries[index].PID == 0 {
			continue
		}
		footprint, err := loadSidecarFootprintMB(ctx, entries[index].PID)
		if err != nil {
			entries[index].FootprintError = err.Error()
			continue
		}
		entries[index].FootprintMB = footprint
	}
}

func loadSidecarFootprintMB(ctx context.Context, pid int) (float64, error) {
	if runtime.GOOS != "darwin" {
		return 0, fmt.Errorf("physical footprint probe is only implemented on darwin")
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "vmmap", "-summary", strconv.Itoa(pid))
	output, err := cmd.CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(output))
		if text == "" {
			return 0, err
		}
		return 0, fmt.Errorf("%v: %s", err, text)
	}
	footprint, ok := parseVMMapPhysicalFootprintMB(string(output))
	if !ok {
		return 0, fmt.Errorf("vmmap physical footprint not found")
	}
	return footprint, nil
}

func parseVMMapPhysicalFootprintMB(output string) (float64, bool) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if !strings.Contains(line, "Physical footprint:") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		value := fields[len(fields)-1]
		megabytes, ok := parseMemoryToMB(value)
		if ok {
			return megabytes, true
		}
	}
	return 0, false
}

func parseMemoryToMB(value string) (float64, bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, false
	}
	unit := trimmed[len(trimmed)-1]
	number := strings.TrimSuffix(trimmed, string(unit))
	amount, err := strconv.ParseFloat(number, 64)
	if err != nil {
		return 0, false
	}
	switch unit {
	case 'K':
		return amount / 1024, true
	case 'M':
		return amount, true
	case 'G':
		return amount * 1024, true
	default:
		plain, err := strconv.ParseFloat(trimmed, 64)
		if err != nil {
			return 0, false
		}
		return plain / (1024 * 1024), true
	}
}
