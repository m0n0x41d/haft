package operatorrequest

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// Provenance is the observable host-side routing basis carried into a kernel
// effect. It does not claim that the kernel independently observed a mental
// intent or established a U.SpeechAct occurrence.
type Provenance string

const HostRoutedOperatorRequest Provenance = "host_routed_operator_request"

// Effect names the institutional effect selected by the host. Keeping this
// algebra closed prevents one operator request from being replayed as another
// kind of authority.
type Effect string

const (
	DecisionBinding          Effect = "decision.bind"
	ProfileDeclaration       Effect = "profile.declare"
	ProjectTypeEnvHeadSelect Effect = "project_typeenv_head.select"
)

// Request is the pure, immutable authority-provenance value passed from a host
// adapter to an effect-specific recorder. It carries no conversational text
// and makes no claim about hidden operator intent.
type Request struct {
	effect        Effect
	subjectRef    string
	payloadDigest string
	requestDigest string
}

func New(effect Effect, subjectRef string, payload []byte) (Request, error) {
	if !validEffect(effect) {
		return Request{}, fmt.Errorf("operator request effect %q is unsupported", effect)
	}
	subject := strings.TrimSpace(subjectRef)
	if subject == "" {
		return Request{}, fmt.Errorf("operator request subject is required")
	}
	if len(payload) == 0 {
		return Request{}, fmt.Errorf("operator request payload is required")
	}
	hash := sha256.Sum256(payload)
	payloadDigest := "sha256:" + hex.EncodeToString(hash[:])
	requestDigest := deriveRequestDigest(effect, subject, payloadDigest)
	return Request{
		effect:        effect,
		subjectRef:    subject,
		payloadDigest: payloadDigest,
		requestDigest: requestDigest,
	}, nil
}

// FromCoordinates reconstructs a request from durable coordinates without
// requiring the original payload bytes. The digest is recomputed, so a stored
// row cannot relabel another effect, subject, or payload as the same request.
func FromCoordinates(
	effect Effect,
	subjectRef string,
	payloadDigest string,
	requestDigest string,
) (Request, error) {
	if !validEffect(effect) {
		return Request{}, fmt.Errorf("operator request effect %q is unsupported", effect)
	}
	subject := strings.TrimSpace(subjectRef)
	if subject == "" || subject != subjectRef {
		return Request{}, fmt.Errorf("operator request subject is required")
	}
	if !validDigest(payloadDigest) || !validDigest(requestDigest) {
		return Request{}, fmt.Errorf("operator request digest coordinates are malformed")
	}
	expected := deriveRequestDigest(effect, subject, payloadDigest)
	if expected != requestDigest {
		return Request{}, fmt.Errorf("operator request digest does not match its coordinates")
	}
	return Request{
		effect:        effect,
		subjectRef:    subject,
		payloadDigest: payloadDigest,
		requestDigest: requestDigest,
	}, nil
}

func (request Request) Provenance() Provenance { return HostRoutedOperatorRequest }
func (request Request) Effect() Effect         { return request.effect }
func (request Request) SubjectRef() string     { return request.subjectRef }
func (request Request) PayloadDigest() string  { return request.payloadDigest }
func (request Request) Digest() string         { return request.requestDigest }
func (request Request) Ref() string {
	return "operator-request:" + request.requestDigest
}

func (request Request) MatchesPayload(payload []byte) bool {
	if len(payload) == 0 {
		return false
	}
	hash := sha256.Sum256(payload)
	digest := "sha256:" + hex.EncodeToString(hash[:])
	return request.payloadDigest == digest
}

func validEffect(effect Effect) bool {
	return effect == DecisionBinding ||
		effect == ProfileDeclaration ||
		effect == ProjectTypeEnvHeadSelect
}

func deriveRequestDigest(
	effect Effect,
	subject string,
	payloadDigest string,
) string {
	requestHash := sha256.New()
	_, _ = requestHash.Write([]byte("haft.host-routed-operator-request/v1"))
	_, _ = requestHash.Write([]byte{0})
	_, _ = requestHash.Write([]byte(effect))
	_, _ = requestHash.Write([]byte{0})
	_, _ = requestHash.Write([]byte(subject))
	_, _ = requestHash.Write([]byte{0})
	_, _ = requestHash.Write([]byte(payloadDigest))
	return "sha256:" + hex.EncodeToString(requestHash.Sum(nil))
}

func validDigest(value string) bool {
	if len(value) != len("sha256:")+sha256.Size*2 ||
		!strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil
}
