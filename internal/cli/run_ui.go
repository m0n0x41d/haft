package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// ANSI escape sequences for terminal formatting.
const (
	aBold    = "\033[1m"
	aDim     = "\033[2m"
	aReset   = "\033[0m"
	aRed     = "\033[31m"
	aGreen   = "\033[32m"
	aYellow  = "\033[33m"
	aBlue    = "\033[34m"
	aMagenta = "\033[35m"
	aCyan    = "\033[36m"
)

// runUI handles terminal output for haft run.
type runUI struct {
	startTime time.Time
	filesRead []string
	filesEdit []string
	cmdsRun   []string
	toolCalls int
}

func (u *runUI) bar() {
	fmt.Println(aCyan + strings.Repeat("━", 52) + aReset)
}

func (u *runUI) header(title string) {
	u.bar()
	fmt.Printf("  %s%s%s\n", aBold, title, aReset)
	u.bar()
}

func (u *runUI) meta(label, value string) {
	fmt.Printf("  %s%-14s%s %s\n", aDim, label, aReset, value)
}

func (u *runUI) phase(name string) {
	fmt.Printf("\n  %s⟳ %s%s\n", aCyan, name, aReset)
	fmt.Printf("  %s──────────────────────────%s\n", aDim, aReset)
}

func (u *runUI) ok(msg string) {
	fmt.Printf("  %s✓%s %s\n", aGreen, aReset, msg)
}

func (u *runUI) fail(msg string) {
	fmt.Printf("  %s✗%s %s\n", aRed, aReset, msg)
}

func (u *runUI) warn(msg string) {
	fmt.Printf("  %s⚠%s %s\n", aYellow, aReset, msg)
}

func (u *runUI) invariantResult(source, text string, pass bool) {
	icon := aGreen + "✓" + aReset
	if !pass {
		icon = aRed + "✗" + aReset
	}
	fmt.Printf("  %s %s[%s]%s %s\n", icon, aDim, source, aReset, text)
}

func (u *runUI) summary() {
	u.bar()
	elapsed := time.Since(u.startTime)
	fmt.Printf("  %sDuration: %ds%s\n", aDim, int(elapsed.Seconds()), aReset)
}

func spawnAgent(agent, prompt, projectRoot string) error {
	var cmd *exec.Cmd
	switch agent {
	case "codex":
		cmd = exec.Command("codex", "exec", "--full-auto", "-c", "mcp_servers={}", "-")
	case "claude":
		cmd = exec.Command("claude", "-p", prompt, "--allowedTools", "Edit,Write,Bash,Read,Glob,Grep")
	default:
		return fmt.Errorf("unknown agent: %s", agent)
	}

	cmd.Dir = projectRoot
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if agent == "codex" {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		if err := cmd.Start(); err != nil {
			return err
		}
		_, _ = stdin.Write([]byte(prompt))
		_ = stdin.Close()
		return cmd.Wait()
	}

	cmd.Stdin = os.Stdin
	return cmd.Run()
}

func newShellCmd(command, dir string) *exec.Cmd {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func uniqueStrings(ss []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
