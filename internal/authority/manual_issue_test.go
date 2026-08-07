package authority

import (
	"bytes"
	"strings"
	"testing"
)

type authorityIssueTerminalFixture struct {
	input       *strings.Reader
	output      bytes.Buffer
	interactive bool
	closed      bool
	events      *[]string
}

func newAuthorityIssueTerminalFixture(
	input string,
	interactive bool,
) *authorityIssueTerminalFixture {
	return &authorityIssueTerminalFixture{
		input:       strings.NewReader(input),
		interactive: interactive,
	}
}

func (terminal *authorityIssueTerminalFixture) Read(value []byte) (int, error) {
	terminal.record("tty:read")
	return terminal.input.Read(value)
}

func (terminal *authorityIssueTerminalFixture) Write(value []byte) (int, error) {
	terminal.record("tty:write")
	return terminal.output.Write(value)
}

func (terminal *authorityIssueTerminalFixture) Close() error {
	terminal.closed = true
	return nil
}

func (terminal *authorityIssueTerminalFixture) Interactive() bool {
	return terminal.interactive
}

func (terminal *authorityIssueTerminalFixture) ObservedSession() (string, error) {
	terminal.record("tty:observe-session")
	return "test-controlling-terminal-session", nil
}

func (terminal *authorityIssueTerminalFixture) record(event string) {
	if terminal.events != nil {
		*terminal.events = append(*terminal.events, event)
	}
}

func TestReadAuthorityIssueAcceptanceRequiresCompleteNewlineTerminatedAct(
	t *testing.T,
) {
	accepted, err := readAuthorityIssueAcceptance(
		strings.NewReader("AUTHORIZE THIS REVIEWED TYPEENV SELECTION\n"),
	)
	if err != nil {
		t.Fatalf("read complete acceptance: %v", err)
	}
	if accepted != "AUTHORIZE THIS REVIEWED TYPEENV SELECTION" {
		t.Fatalf("accepted phrase = %q", accepted)
	}

	accepted, err = readAuthorityIssueAcceptance(
		strings.NewReader("AUTHORIZE THIS REVIEWED TYPEENV SELECTION"),
	)
	if err == nil || accepted != "" {
		t.Fatalf(
			"unterminated acceptance = (%q, %v), want fail-closed",
			accepted,
			err,
		)
	}
}
