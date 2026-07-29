package authority

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-isatty"
)

const authorityIssueTTYPath = "/dev/tty"

const authorityIssueIdentityBoundary = "This confirmation is read from your active terminal to prevent accidental or model-only approval. It does not verify your legal identity."

const authorityTerminalSessionDigestDomain = "haft.authority.observed-terminal-session/v1"
const authorityTerminalObservationDigestDomain = "haft.authority.terminal-session-observation-material/v1"

type authorityIssueTerminal interface {
	io.Reader
	io.Writer
	Close() error
	Interactive() bool
	ObservedSession() (string, error)
}

type authorityIssueFileTerminal struct {
	file *os.File
}

// CaptureVerifiedSpeechAct is the generic manual source mint. It records only
// terminal capture, context-policy-derived authorizer assignment, and actual
// SpeechAct Work. Higher-level protocols institute their own typed effects.
func CaptureVerifiedSpeechAct(
	ctx context.Context,
	prepared PreparedManualSpeechAct,
) (VerifiedSpeechActSource, error) {
	if ctx == nil {
		return VerifiedSpeechActSource{}, fmt.Errorf("SpeechAct capture requires a context")
	}
	terminal, err := openAuthorityIssueTerminal()
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	defer terminal.Close()
	return captureVerifiedSpeechAct(
		ctx,
		prepared,
		terminal,
		time.Now,
		rand.Reader,
	)
}

func captureVerifiedSpeechAct(
	ctx context.Context,
	prepared PreparedManualSpeechAct,
	terminal authorityIssueTerminal,
	now func() time.Time,
	nonce io.Reader,
) (VerifiedSpeechActSource, error) {
	if ctx == nil {
		return VerifiedSpeechActSource{}, fmt.Errorf("SpeechAct capture requires a context")
	}
	if err := ctx.Err(); err != nil {
		return VerifiedSpeechActSource{}, err
	}
	if !prepared.valid() {
		return VerifiedSpeechActSource{}, fmt.Errorf("manual SpeechAct capture requires exact prepared source material")
	}
	if terminal == nil || !terminal.Interactive() {
		return VerifiedSpeechActSource{}, fmt.Errorf("manual SpeechAct requires an interactive %s", authorityIssueTTYPath)
	}
	if now == nil || nonce == nil {
		return VerifiedSpeechActSource{}, fmt.Errorf("manual SpeechAct runtime is unavailable")
	}
	startedAt := canonicalAuthorityTime(now())
	if startedAt.IsZero() {
		return VerifiedSpeechActSource{}, fmt.Errorf("manual SpeechAct start observation is unavailable")
	}
	if err := writeSpeechActIntentReview(terminal, prepared); err != nil {
		return VerifiedSpeechActSource{}, err
	}
	state := prepared.state
	want, err := state.intent.state.utteranceRule.expected(
		state.reviewDigest,
		state.intent.state.reviewSubjectDig,
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	got, err := readAuthorityIssueAcceptance(terminal)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	if got != want {
		return VerifiedSpeechActSource{}, fmt.Errorf("SpeechAct rejected: input did not exactly match %q", want)
	}
	exactUtteranceObservedAt, err := nextAuthorityWorkObservation(
		now,
		startedAt,
		"exact utterance",
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	observation, err := observeAuthorityTerminalSession(
		terminal,
		exactUtteranceObservedAt,
		nonce,
	)
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	endedAt, err := nextAuthorityWorkObservation(now, exactUtteranceObservedAt, "SpeechAct end")
	if err != nil {
		return VerifiedSpeechActSource{}, err
	}
	return newVerifiedSpeechActSource(
		state.intent,
		state.reviewText,
		state.reviewDigest,
		got,
		startedAt,
		exactUtteranceObservedAt,
		endedAt,
		observation,
	)
}

func writeSpeechActIntentReview(
	writer io.Writer,
	prepared PreparedManualSpeechAct,
) error {
	if !prepared.valid() {
		return fmt.Errorf("prepared manual SpeechAct is invalid")
	}
	lines := []string{
		prepared.state.reviewText,
		"",
		authorityIssueIdentityBoundary,
		"To confirm this exact action, type:",
		canonicalSpeechActReviewPhrase(prepared),
		"> ",
	}
	presentation := strings.Join(lines, "\n")
	_, err := io.WriteString(writer, presentation)
	if err != nil {
		return fmt.Errorf("display SpeechAct review: %w", err)
	}
	return nil
}

func canonicalSpeechActReviewPhrase(
	prepared PreparedManualSpeechAct,
) string {
	if !prepared.valid() {
		return ""
	}
	phrase, err := prepared.state.intent.state.utteranceRule.expected(
		prepared.state.reviewDigest,
		prepared.state.intent.state.reviewSubjectDig,
	)
	if err != nil {
		return ""
	}
	return phrase
}

func (terminal authorityIssueFileTerminal) Read(value []byte) (int, error) {
	return terminal.file.Read(value)
}

func (terminal authorityIssueFileTerminal) Write(value []byte) (int, error) {
	return terminal.file.Write(value)
}

func (terminal authorityIssueFileTerminal) Close() error {
	return terminal.file.Close()
}

func (terminal authorityIssueFileTerminal) Interactive() bool {
	fd := terminal.file.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

func (terminal authorityIssueFileTerminal) ObservedSession() (string, error) {
	info, err := terminal.file.Stat()
	if err != nil {
		return "", fmt.Errorf("inspect controlling terminal session: %w", err)
	}
	return fmt.Sprintf(
		"path=%s;mode=%s;pid=%d;ppid=%d",
		terminal.file.Name(),
		info.Mode().String(),
		os.Getpid(),
		os.Getppid(),
	), nil
}

// CaptureVerifiedAuthorityAct is the only production mint for a manual
// authority SpeechAct. The input is pre-act intent; the terminal capture,
// authorizer RoleAssignment, performed Work, permission, presentation, and
// resolution are all derived after the exact /dev/tty utterance.
func CaptureVerifiedAuthorityAct(
	ctx context.Context,
	intent PreparedAuthorityIntent,
	reviewText string,
	exactPhraseDigest Digest,
) (VerifiedAuthorityAct, error) {
	if ctx == nil {
		return VerifiedAuthorityAct{}, fmt.Errorf("authority act capture requires a context")
	}
	terminal, err := openAuthorityIssueTerminal()
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	defer terminal.Close()
	return captureVerifiedAuthorityAct(
		ctx,
		intent,
		reviewText,
		exactPhraseDigest,
		terminal,
		time.Now,
		rand.Reader,
	)
}

func captureVerifiedAuthorityAct(
	ctx context.Context,
	intent PreparedAuthorityIntent,
	reviewText string,
	exactPhraseDigest Digest,
	terminal authorityIssueTerminal,
	now func() time.Time,
	nonce io.Reader,
) (VerifiedAuthorityAct, error) {
	if ctx == nil {
		return VerifiedAuthorityAct{}, fmt.Errorf("authority act capture requires a context")
	}
	if !intent.valid() {
		return VerifiedAuthorityAct{}, fmt.Errorf("prepared profile authority intent is invalid")
	}
	prepared, err := PrepareManualSpeechAct(intent.state.sourceIntent, reviewText)
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	reviewDigest, _ := prepared.ReviewDigest()
	if reviewDigest != exactPhraseDigest {
		return VerifiedAuthorityAct{}, fmt.Errorf("authority act phrase digest does not bind the exact prepared intent review")
	}
	source, err := captureVerifiedSpeechAct(
		ctx,
		prepared,
		terminal,
		now,
		nonce,
	)
	if err != nil {
		return VerifiedAuthorityAct{}, err
	}
	return completeVerifiedAuthorityAct(intent, source)
}

func nextAuthorityWorkObservation(
	now func() time.Time,
	previous time.Time,
	boundary string,
) (time.Time, error) {
	observed := canonicalAuthorityTime(now())
	if observed.After(previous) {
		return observed, nil
	}
	return time.Time{}, fmt.Errorf(
		"%s clock did not advance after the preceding actual operation",
		boundary,
	)
}

func observeAuthorityTerminalSession(
	terminal authorityIssueTerminal,
	capturedAt time.Time,
	nonce io.Reader,
) (terminalSessionObservation, error) {
	observed, err := terminal.ObservedSession()
	if err != nil {
		return terminalSessionObservation{}, err
	}
	if observed == "" || strings.TrimSpace(observed) != observed {
		return terminalSessionObservation{}, fmt.Errorf("controlling terminal session observation is invalid")
	}
	nonceBytes := make([]byte, 16)
	_, err = io.ReadFull(nonce, nonceBytes)
	if err != nil {
		return terminalSessionObservation{}, fmt.Errorf("generate terminal-session observation nonce: %w", err)
	}
	return newTerminalSessionObservation(
		observed,
		fmt.Sprintf("%x", nonceBytes),
		capturedAt,
	)
}

func newTerminalSessionObservation(
	material string,
	nonce string,
	capturedAt time.Time,
) (terminalSessionObservation, error) {
	canonicalCapturedAt := canonicalAuthorityTime(capturedAt)
	validMaterial := material != "" && strings.TrimSpace(material) == material
	validNonce := len(nonce) == 32 && !strings.ContainsFunc(nonce, func(value rune) bool {
		return !strings.ContainsRune("0123456789abcdef", value)
	})
	if !validMaterial || !validNonce || canonicalCapturedAt.IsZero() {
		return terminalSessionObservation{}, fmt.Errorf("terminal-session observation material is invalid")
	}
	observationWriter := newAuthorityDigestWriter(authorityTerminalObservationDigestDomain)
	observationWriter.add(material)
	observationDigest := observationWriter.digest()
	identityWriter := newAuthorityDigestWriter(authorityTerminalSessionDigestDomain)
	identityWriter.add(observationDigest.String())
	identityWriter.add(formatAuthorityTime(canonicalCapturedAt))
	identityWriter.add(nonce)
	identity := strings.TrimPrefix(identityWriter.digest().String(), "sha256:")[:32]
	holder, err := NewSystemRef("system:local-terminal-session:" + identity)
	if err != nil {
		return terminalSessionObservation{}, err
	}
	assignment, err := NewRoleAssignmentRef("role-assignment:terminal-authorizer:" + identity)
	if err != nil {
		return terminalSessionObservation{}, err
	}
	return terminalSessionObservation{
		material:        material,
		nonce:           nonce,
		digest:          observationDigest,
		holderSystemRef: holder,
		assignmentRef:   assignment,
	}, nil
}

func openAuthorityIssueTerminal() (authorityIssueTerminal, error) {
	file, err := os.OpenFile(authorityIssueTTYPath, os.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open controlling terminal %s: %w", authorityIssueTTYPath, err)
	}
	return authorityIssueFileTerminal{file: file}, nil
}

func readAuthorityIssueAcceptance(reader io.Reader) (string, error) {
	buffered := bufio.NewReader(reader)
	line, err := buffered.ReadString('\n')
	if errors.Is(err, io.EOF) {
		return "", fmt.Errorf(
			"read authority issue acceptance from %s: terminal input ended before a complete newline-terminated SpeechAct",
			authorityIssueTTYPath,
		)
	}
	if err != nil {
		return "", fmt.Errorf("read authority issue acceptance from %s: %w", authorityIssueTTYPath, err)
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}
