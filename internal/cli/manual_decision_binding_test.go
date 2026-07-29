package cli

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/project"
)

type fakeManualDecisionBindingSession struct {
	bindInput    artifact.DecideInput
	bindResult   manualDecisionBindingOutcome
	bindErr      error
	resumeRef    string
	resumeResult manualDecisionBindingOutcome
	resumeErr    error
	closed       bool
}

func (session *fakeManualDecisionBindingSession) Bind(
	_ context.Context,
	input artifact.DecideInput,
) (manualDecisionBindingOutcome, error) {
	session.bindInput = input
	return session.bindResult, session.bindErr
}

func (session *fakeManualDecisionBindingSession) Resume(
	_ context.Context,
	decisionRef string,
) (manualDecisionBindingOutcome, error) {
	session.resumeRef = decisionRef
	return session.resumeResult, session.resumeErr
}

func (session *fakeManualDecisionBindingSession) Close() error {
	session.closed = true
	return nil
}

func stubManualDecisionBindingSession(
	t *testing.T,
	session manualDecisionBindingSession,
) {
	t.Helper()
	previous := openManualDecisionBindingSession
	openManualDecisionBindingSession = func(
		context.Context,
		string,
	) (manualDecisionBindingSession, error) {
		return session, nil
	}
	t.Cleanup(func() {
		openManualDecisionBindingSession = previous
	})
}

func TestManualDecisionArtifactCreateUsesBindingServiceAndReadableResult(t *testing.T) {
	root := newArtifactCLITestProject(t)
	writeDecisionBindingModeForTest(
		t,
		root,
		project.DecisionBindingModeStrictCLISpeechAct,
	)
	session := &fakeManualDecisionBindingSession{
		bindResult: manualDecisionBindingOutcome{
			DecisionRef: "dec-20260715-readable-a1b2c3d4",
			Title:       "Use the typed decision service",
			FilePath:    root + "/.haft/decisions/typed-decision.md",
		},
	}
	stubManualDecisionBindingSession(t, session)
	input := []byte(`{
		"selected_title":"Use the typed decision service",
		"problem_statement":"The public CLI must not bypass the human decision act.",
		"why_selected":"It preserves the authority boundary.",
		"selection_policy":"Require a literal manual SpeechAct.",
		"counterargument":"It adds one explicit interaction.",
		"weakest_link":"The controlling terminal can be unavailable."
	}`)
	command := &cobra.Command{}
	output := bytes.Buffer{}
	command.SetOut(&output)
	previousJSON := artifactCreateJSON
	artifactCreateJSON = false
	t.Cleanup(func() { artifactCreateJSON = previousJSON })

	if err := runDecisionArtifactCreate(command, input); err != nil {
		t.Fatalf("runDecisionArtifactCreate: %v", err)
	}
	if session.bindInput.SelectedTitle != "Use the typed decision service" {
		t.Fatalf("binding input title = %q", session.bindInput.SelectedTitle)
	}
	for _, want := range []string{
		"dec-20260715-readable-a1b2c3d4",
		"Use the typed decision service",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("result omitted %q:\n%s", want, output.String())
		}
	}
	if !session.closed {
		t.Fatal("manual decision session was not closed")
	}
}

func TestManualDecisionFailureNamesReadableResumeCommand(t *testing.T) {
	result := manualDecisionBindingOutcome{
		DecisionRef: "dec-20260715-resume-a1b2c3d4",
		Title:       "Preserve the durable human act",
	}
	err := resumableDecisionBindingError(
		result,
		errors.New("checked post-source/pre-effect guard rejected the ledger"),
	)
	for _, want := range []string{
		"dec-20260715-resume-a1b2c3d4",
		"Preserve the durable human act",
		"was not instituted",
		"haft artifact resume-decision dec-20260715-resume-a1b2c3d4",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error omitted %q: %v", want, err)
		}
	}
	for _, forbidden := range []string{"digest", "nonce", "sha256:"} {
		if strings.Contains(strings.ToLower(err.Error()), forbidden) {
			t.Fatalf("ordinary resume error exposed %q: %v", forbidden, err)
		}
	}
}

func TestPostEffectVerificationFailureDoesNotClaimDecisionWasAbsent(t *testing.T) {
	result := manualDecisionBindingOutcome{
		DecisionRef:      "dec-20260715-verified-late-a1b2c3d4",
		Title:            "Keep institutional fact distinct from verification",
		EffectInstituted: true,
	}
	err := resumableDecisionBindingError(
		result,
		errors.New("checked ledger topology changed after commit"),
	)
	if !strings.Contains(err.Error(), "was instituted") ||
		strings.Contains(err.Error(), "was not instituted") {
		t.Fatalf("post-effect error confused institution with verification: %v", err)
	}
}

func TestArtifactResumeDecisionUsesOnlyDecisionID(t *testing.T) {
	root := newArtifactCLITestProject(t)
	writeDecisionBindingModeForTest(
		t,
		root,
		project.DecisionBindingModeStrictCLISpeechAct,
	)
	session := &fakeManualDecisionBindingSession{
		resumeResult: manualDecisionBindingOutcome{
			DecisionRef: "dec-20260715-resume-b2c3d4e5",
			Title:       "Resume by readable decision identity",
			ExactReplay: true,
		},
	}
	stubManualDecisionBindingSession(t, session)
	command := &cobra.Command{}
	output := bytes.Buffer{}
	command.SetOut(&output)
	previousJSON := artifactResumeJSON
	artifactResumeJSON = false
	t.Cleanup(func() { artifactResumeJSON = previousJSON })

	err := runArtifactResumeDecision(
		command,
		[]string{"dec-20260715-resume-b2c3d4e5"},
	)
	if err != nil {
		t.Fatalf("runArtifactResumeDecision: %v", err)
	}
	if session.resumeRef != "dec-20260715-resume-b2c3d4e5" {
		t.Fatalf("resume ref = %q", session.resumeRef)
	}
	if !strings.Contains(output.String(), "Resume by readable decision identity") {
		t.Fatalf("resume output omitted title:\n%s", output.String())
	}
}

func TestArtifactResumeDecisionRejectsExplicitHDecideMode(t *testing.T) {
	_ = newArtifactCLITestProject(t)
	command := &cobra.Command{}

	err := runArtifactResumeDecision(
		command,
		[]string{"dec-20260715-no-pending-speech-act-a1b2c3d4"},
	)
	if err == nil {
		t.Fatal("resume-decision accepted explicit_h_decide mode")
	}
	for _, want := range []string{
		"strict_cli_speech_act",
		"explicit_h_decide",
		"does not create the separate durable SpeechAct",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("resume error omitted %q: %v", want, err)
		}
	}
}
