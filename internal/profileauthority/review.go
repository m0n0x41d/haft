package profileauthority

import (
	"fmt"
	"slices"
	"strings"
)

type ReviewCard struct {
	text       string
	nonEffects []string
}

func NewReviewCard(content AuthorizationContent) (ReviewCard, error) {
	if !content.valid() {
		return ReviewCard{}, fmt.Errorf(
			"profile authorization review requires canonical content",
		)
	}
	root := content.state.projectRoot.String()
	validFrom := formatTime(content.state.authorizationValidity.From())
	validUntil := formatTime(content.state.authorizationValidity.Until())
	nonEffects := []string{
		"does not itself declare or change the project profile",
		"does not perform the future onboarding Work",
		"does not modify specifications, code, or project files",
		"does not authorize another action, project, subject, method, or session",
	}
	lines := []string{
		"Project profile authorization review",
		"Affected project: " + root,
		"Change: institute one single-use MAY permission for the pre-bound ProfileAuthor to perform the reviewed profile-declaration method.",
		"Validity: " + validFrom + " through " + validUntil + ".",
		"Consequences: the later profile-admission gate may consume this permission once, only inside its bound method, evidence, and Work windows.",
		"Non-effects:",
		"- " + nonEffects[0],
		"- " + nonEffects[1],
		"- " + nonEffects[2],
		"- " + nonEffects[3],
		"Cancel/reversibility: enter anything except the exact phrase to cancel; after capture, the SpeechAct remains historical even if the permission effect fails.",
		"Exact phrase: " + AuthorizationPhrase(),
	}
	text := strings.Join(lines, "\n")
	return ReviewCard{text: text, nonEffects: nonEffects}, nil
}

func (card ReviewCard) valid() bool {
	return card.text != "" &&
		len(card.nonEffects) == 4 &&
		strings.Contains(card.text, AuthorizationPhrase()) &&
		!strings.Contains(card.text, "sha256:")
}

func (card ReviewCard) Text() (string, bool) {
	return card.text, card.valid()
}

func (card ReviewCard) NonEffects() ([]string, bool) {
	if !card.valid() {
		return nil, false
	}
	return slices.Clone(card.nonEffects), true
}
