package cli

import (
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

type doctorServeProcess struct {
	PID             int
	Command         string
	StartTime       time.Time
	CWD             string
	Executable      string
	ExecutableMTime time.Time
}

type doctorServeProcessSnapshot struct {
	Processes     []doctorServeProcess
	PathHaft      string
	PathHaftMTime time.Time
	Error         string
}

func collectDoctorServeProcessSnapshot() doctorServeProcessSnapshot {
	pathHaft, pathHaftMTime := doctorPathHaft()
	processes, err := doctorServeProcesses()
	if err != nil {
		return doctorServeProcessSnapshot{
			PathHaft:      pathHaft,
			PathHaftMTime: pathHaftMTime,
			Error:         err.Error(),
		}
	}

	return doctorServeProcessSnapshot{
		Processes:     processes,
		PathHaft:      pathHaft,
		PathHaftMTime: pathHaftMTime,
	}
}

func doctorPathHaft() (string, time.Time) {
	path, err := exec.LookPath("haft")
	if err != nil {
		return "", time.Time{}
	}

	absolutePath := doctorCleanPath(path)
	info, err := os.Stat(absolutePath)
	if err != nil {
		return absolutePath, time.Time{}
	}

	return absolutePath, info.ModTime()
}

func doctorServeProcesses() ([]doctorServeProcess, error) {
	if runtime.GOOS == "windows" {
		return nil, fmt.Errorf("serve process inspection is not supported on windows")
	}

	output, err := exec.Command("ps", "ax", "-o", "pid=,command=").Output()
	if err != nil {
		return nil, err
	}

	processes := make([]doctorServeProcess, 0)
	for _, line := range strings.Split(string(output), "\n") {
		process, ok := parseDoctorServeProcessLine(line)
		if !ok {
			continue
		}
		process = doctorServeProcessWithOpenFiles(process)
		processes = append(processes, process)
	}

	sort.Slice(processes, func(i int, j int) bool {
		return processes[i].PID < processes[j].PID
	})

	return processes, nil
}

func parseDoctorServeProcessLine(line string) (doctorServeProcess, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return doctorServeProcess{}, false
	}

	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return doctorServeProcess{}, false
	}

	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return doctorServeProcess{}, false
	}

	command := strings.TrimSpace(strings.TrimPrefix(trimmed, fields[0]))
	if !doctorCommandRunsHaftServe(command) {
		return doctorServeProcess{}, false
	}

	return doctorServeProcess{
		PID:     pid,
		Command: command,
	}, true
}

func doctorCommandRunsHaftServe(command string) bool {
	fields := strings.Fields(command)
	for index := 0; index+1 < len(fields); index++ {
		if filepath.Base(fields[index]) == "haft" && fields[index+1] == "serve" {
			return true
		}
	}
	return false
}

func doctorServeProcessWithOpenFiles(process doctorServeProcess) doctorServeProcess {
	process.StartTime = doctorProcessStartTime(process.PID)

	output, err := exec.Command("lsof", "-p", strconv.Itoa(process.PID), "-Fn").Output()
	if err != nil {
		return process
	}

	cwd, executable := doctorProcessCWDAndExecutable(string(output))
	process.CWD = cwd
	process.Executable = executable
	process.ExecutableMTime = doctorFileModTime(executable)

	return process
}

func doctorProcessStartTime(pid int) time.Time {
	output, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "lstart=").Output()
	if err != nil {
		return time.Time{}
	}

	startedAt, err := time.ParseInLocation(
		"Mon Jan _2 15:04:05 2006",
		strings.TrimSpace(string(output)),
		time.Local,
	)
	if err != nil {
		return time.Time{}
	}

	return startedAt
}

func doctorProcessCWDAndExecutable(output string) (string, string) {
	currentField := ""
	firstText := ""
	haftText := ""
	cwd := ""

	for _, line := range strings.Split(output, "\n") {
		if line == "" {
			continue
		}
		switch line[0] {
		case 'f':
			currentField = strings.TrimSpace(line[1:])
		case 'n':
			name := strings.TrimSpace(line[1:])
			if currentField == "cwd" {
				cwd = name
			}
			if currentField == "txt" && firstText == "" {
				firstText = name
			}
			if currentField == "txt" && filepath.Base(name) == "haft" && haftText == "" {
				haftText = name
			}
		}
	}

	if haftText != "" {
		return cwd, doctorCleanPath(haftText)
	}

	return cwd, doctorCleanPath(firstText)
}

func doctorFileModTime(path string) time.Time {
	if strings.TrimSpace(path) == "" {
		return time.Time{}
	}

	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}

	return info.ModTime()
}

func doctorServeProcessStatus(projectRoot string, snapshot doctorServeProcessSnapshot) (string, bool) {
	if snapshot.Error != "" {
		return "cannot inspect haft serve processes: " + snapshot.Error, false
	}

	current := doctorCurrentProjectServeProcesses(projectRoot, snapshot.Processes)
	if len(current) == 0 {
		return doctorNoCurrentServeMessage(snapshot), true
	}

	issues := doctorServeProcessIssues(current, snapshot)
	if len(issues) > 0 {
		return strings.Join(issues, "; ") + "; restart host/MCP after rebuilding the configured binary", false
	}

	return fmt.Sprintf(
		"%d current-project serve process(es): %s",
		len(current),
		doctorServeProcessList(current),
	), true
}

func doctorNoCurrentServeMessage(snapshot doctorServeProcessSnapshot) string {
	if len(snapshot.Processes) == 0 {
		return "none running"
	}

	return fmt.Sprintf(
		"none for current project; %d other serve process(es) running",
		len(snapshot.Processes),
	)
}

func doctorCurrentProjectServeProcesses(projectRoot string, processes []doctorServeProcess) []doctorServeProcess {
	root := doctorCleanPath(projectRoot)
	current := make([]doctorServeProcess, 0, len(processes))
	for _, process := range processes {
		if doctorCleanPath(process.CWD) != root {
			continue
		}
		current = append(current, process)
	}
	return current
}

func doctorServeProcessIssues(current []doctorServeProcess, snapshot doctorServeProcessSnapshot) []string {
	issues := make([]string, 0)
	for _, process := range current {
		if doctorServeExecutableDiffersFromPath(process, snapshot.PathHaft) {
			issues = append(issues, fmt.Sprintf(
				"serve executable differs from PATH haft: pid=%d executable=%s path_haft=%s",
				process.PID,
				doctorDisplayPath(process.Executable),
				doctorDisplayPath(snapshot.PathHaft),
			))
		}
		if doctorServeExecutableOlderThanPath(process, snapshot) {
			issues = append(issues, fmt.Sprintf(
				"serve executable older than PATH haft: pid=%d executable_mtime=%s path_haft_mtime=%s",
				process.PID,
				doctorDisplayTime(process.ExecutableMTime),
				doctorDisplayTime(snapshot.PathHaftMTime),
			))
		}
		if doctorServeProcessStartedBeforeRebuild(process) {
			issues = append(issues, fmt.Sprintf(
				"serve process predates executable rebuild: pid=%d started=%s executable_mtime=%s",
				process.PID,
				doctorDisplayTime(process.StartTime),
				doctorDisplayTime(process.ExecutableMTime),
			))
		}
	}

	return issues
}

func doctorServeExecutableDiffersFromPath(process doctorServeProcess, pathHaft string) bool {
	if strings.TrimSpace(process.Executable) == "" || strings.TrimSpace(pathHaft) == "" {
		return false
	}
	return doctorCleanPath(process.Executable) != doctorCleanPath(pathHaft)
}

func doctorServeExecutableOlderThanPath(process doctorServeProcess, snapshot doctorServeProcessSnapshot) bool {
	if process.ExecutableMTime.IsZero() || snapshot.PathHaftMTime.IsZero() {
		return false
	}
	return process.ExecutableMTime.Before(snapshot.PathHaftMTime)
}

func doctorServeProcessStartedBeforeRebuild(process doctorServeProcess) bool {
	if process.StartTime.IsZero() || process.ExecutableMTime.IsZero() {
		return false
	}
	return process.StartTime.Before(process.ExecutableMTime)
}

func doctorServeProcessList(processes []doctorServeProcess) string {
	parts := make([]string, 0, len(processes))
	for _, process := range processes {
		parts = append(parts, fmt.Sprintf(
			"pid=%d started=%s cwd=%s executable=%s mtime=%s",
			process.PID,
			doctorDisplayTime(process.StartTime),
			doctorDisplayPath(process.CWD),
			doctorDisplayPath(process.Executable),
			doctorDisplayTime(process.ExecutableMTime),
		))
	}
	return strings.Join(parts, ", ")
}

func doctorDisplayPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return "<unknown>"
	}
	return path
}

func doctorDisplayTime(value time.Time) string {
	if value.IsZero() {
		return "<unknown>"
	}
	return value.UTC().Format(time.RFC3339)
}

func doctorCleanPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}

	absolutePath, err := filepath.Abs(trimmed)
	if err != nil {
		return filepath.Clean(trimmed)
	}

	resolvedPath, err := filepath.EvalSymlinks(absolutePath)
	if err != nil {
		return filepath.Clean(absolutePath)
	}

	return filepath.Clean(resolvedPath)
}
