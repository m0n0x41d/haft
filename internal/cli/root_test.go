package cli

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestRootExecutePrintsOneErrorWithoutUsage(t *testing.T) {
	command := exec.Command(
		os.Args[0],
		"-test.run=^TestRootExecuteHelper$",
	)
	command.Env = append(
		os.Environ(),
		"HAFT_TEST_ROOT_EXECUTE_HELPER=1",
	)
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatal("unknown command unexpectedly succeeded")
	}
	text := string(output)
	if strings.Count(text, "Error:") != 1 {
		t.Fatalf("error count = %d, output:\n%s", strings.Count(text, "Error:"), text)
	}
	if strings.Contains(text, "Usage:") ||
		strings.Count(text, "unknown command") != 1 {
		t.Fatalf("root error rendering is noisy:\n%s", text)
	}
}

func TestRootExecuteHelper(t *testing.T) {
	if os.Getenv("HAFT_TEST_ROOT_EXECUTE_HELPER") != "1" {
		return
	}
	rootCmd.SetArgs([]string{"definitely-not-a-haft-command"})
	Execute()
}

func TestRootExecuteInitSyntaxErrorsPrintOnlyHelpHint(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		wantDetail string
	}{
		{
			name:       "positional arguments",
			mode:       "positional",
			wantDetail: `accepts no positional arguments: "unexpected-positional"`,
		},
		{
			name:       "unknown flag",
			mode:       "unknown-flag",
			wantDetail: "unknown flag: --definitely-unknown",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := executeInitErrorHelper(test.mode)
			if err == nil {
				t.Fatal("init syntax error unexpectedly succeeded")
			}
			text := string(output)
			if strings.Count(text, "Error:") != 1 {
				t.Fatalf(
					"error count = %d, output:\n%s",
					strings.Count(text, "Error:"),
					text,
				)
			}
			if strings.Count(text, test.wantDetail) != 1 {
				t.Fatalf(
					"syntax detail count = %d, output:\n%s",
					strings.Count(text, test.wantDetail),
					text,
				)
			}
			if strings.Count(
				text,
				"Run 'haft init --help' for help.",
			) != 1 {
				t.Fatalf("help hint is missing or repeated:\n%s", text)
			}
			if strings.Contains(text, "Usage:") {
				t.Fatalf("syntax error printed full usage:\n%s", text)
			}
		})
	}
}

func TestRootExecuteInitSemanticErrorHasNoSyntaxHelp(t *testing.T) {
	output, err := executeInitErrorHelper("semantic")
	if err == nil {
		t.Fatal("invalid init selection unexpectedly succeeded")
	}
	text := string(output)
	if strings.Count(text, "Error:") != 1 {
		t.Fatalf(
			"error count = %d, output:\n%s",
			strings.Count(text, "Error:"),
			text,
		)
	}
	if !strings.Contains(
		text,
		"--mcp-only requires an explicit host flag or --all",
	) {
		t.Fatalf("semantic error detail is missing:\n%s", text)
	}
	if strings.Contains(text, "Usage:") ||
		strings.Contains(text, "haft init --help") {
		t.Fatalf("semantic error rendering is noisy:\n%s", text)
	}
}

func executeInitErrorHelper(mode string) ([]byte, error) {
	command := exec.Command(
		os.Args[0],
		"-test.run=^TestRootExecuteInitErrorHelper$",
	)
	command.Env = append(
		os.Environ(),
		"HAFT_TEST_ROOT_EXECUTE_INIT_ERROR_HELPER="+mode,
	)
	return command.CombinedOutput()
}

func TestRootExecuteInitErrorHelper(t *testing.T) {
	argumentsByMode := map[string][]string{
		"positional": {
			"init",
			"unexpected-positional",
		},
		"unknown-flag": {
			"init",
			"--definitely-unknown",
		},
		"semantic": {
			"init",
			"--mcp-only",
		},
	}
	mode := os.Getenv("HAFT_TEST_ROOT_EXECUTE_INIT_ERROR_HELPER")
	arguments, found := argumentsByMode[mode]
	if !found {
		return
	}
	rootCmd.SetArgs(arguments)
	Execute()
}
