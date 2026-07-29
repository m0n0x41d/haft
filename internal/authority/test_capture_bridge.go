package authority

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// CaptureVerifiedAuthorityActForTestFixture lets cross-package integration
// fixtures traverse the real sealed capture core without exposing a production
// mint. testing.TB has a package-private method, so non-test callers cannot
// implement or obtain the required capability.
func CaptureVerifiedAuthorityActForTestFixture(
	t testing.TB,
	intent PreparedAuthorityIntent,
	reviewText string,
	reviewDigest Digest,
	startedAt time.Time,
	exactUtteranceObservedAt time.Time,
	endedAt time.Time,
) (VerifiedAuthorityAct, error) {
	t.Helper()
	if err := requireAuthorityTestFixtureRuntime(testing.Testing()); err != nil {
		return VerifiedAuthorityAct{}, err
	}
	phrase, err := intent.state.sourceIntent.state.utteranceRule.expected(
		reviewDigest,
		intent.state.sourceIntent.state.reviewSubjectDig,
	)
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	terminal := &authorityFixtureTerminal{
		input: strings.NewReader(phrase + "\n"),
	}
	intentDigest, _ := intent.Digest()
	nonceMaterial := sha256.Sum256([]byte(intentDigest.String()))
	clock := authorityFixtureClock(
		[]time.Time{startedAt, exactUtteranceObservedAt, endedAt},
	)
	return captureVerifiedAuthorityAct(
		context.Background(),
		intent,
		reviewText,
		reviewDigest,
		terminal,
		clock,
		bytes.NewReader(nonceMaterial[:16]),
	)
}

// CaptureVerifiedSpeechActForTestFixture is the generic-source sibling used
// by cross-package integration tests for non-profile institutional effects.
// The testing.TB capability and runtime guard keep this mint unavailable to
// production callers.
func CaptureVerifiedSpeechActForTestFixture(
	t testing.TB,
	prepared PreparedManualSpeechAct,
	startedAt time.Time,
	exactUtteranceObservedAt time.Time,
	endedAt time.Time,
) (VerifiedSpeechActSource, error) {
	t.Helper()
	if err := requireAuthorityTestFixtureRuntime(testing.Testing()); err != nil {
		return VerifiedSpeechActSource{}, err
	}
	if !prepared.valid() {
		return VerifiedSpeechActSource{}, fmt.Errorf("manual SpeechAct fixture requires exact prepared source material")
	}
	terminal := &authorityFixtureTerminal{
		input: strings.NewReader(canonicalSpeechActReviewPhrase(prepared) + "\n"),
	}
	intentDigest, _ := prepared.state.intent.Digest()
	nonceMaterial := sha256.Sum256([]byte(intentDigest.String()))
	clock := authorityFixtureClock(
		[]time.Time{startedAt, exactUtteranceObservedAt, endedAt},
	)
	return captureVerifiedSpeechAct(
		context.Background(),
		prepared,
		terminal,
		clock,
		bytes.NewReader(nonceMaterial[:16]),
	)
}

func requireAuthorityTestFixtureRuntime(isTesting bool) error {
	if isTesting {
		return nil
	}
	return fmt.Errorf("deterministic authority fixture capture is unavailable outside go test")
}

type authorityFixtureTerminal struct {
	input *strings.Reader
}

func (terminal *authorityFixtureTerminal) Read(value []byte) (int, error) {
	return terminal.input.Read(value)
}

func (*authorityFixtureTerminal) Write(value []byte) (int, error) {
	return len(value), nil
}

func (*authorityFixtureTerminal) Close() error { return nil }

func (*authorityFixtureTerminal) Interactive() bool { return true }

func (*authorityFixtureTerminal) ObservedSession() (string, error) {
	return "test-fixture-controlling-terminal-session", nil
}

func authorityFixtureClock(observations []time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if index >= len(observations) {
			return time.Time{}
		}
		observed := observations[index]
		index++
		return observed
	}
}

var _ authorityIssueTerminal = (*authorityFixtureTerminal)(nil)
var _ io.Reader = (*authorityFixtureTerminal)(nil)
