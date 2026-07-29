package onboarding

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	MemoryReviewRef                   = "review:onboard-memory"
	MemoryDeferralSchema              = "haft.onboarding.memory-deferral/v1"
	MemoryDeferralProvenance          = "dedicated_onboard_memory_defer_cli"
	MemoryDeferralInterpretationLimit = "non_binding_disposition_only"
)

// MemoryDeferral is the exact non-binding record of an operator choosing
// "Not now" for one current structured-memory review. It carries only the
// task-level review identity and never exposes or selects an internal schema.
type MemoryDeferral struct {
	projectID    string
	reviewRef    string
	reviewDigest string
	choice       string
	provenance   string
	limit        string
	recordedAt   time.Time
}

type MemoryDeferralInput struct {
	ProjectID    string
	ReviewRef    string
	ReviewDigest string
	Choice       string
	RecordedAt   time.Time
}

type memoryDeferralWire struct {
	Schema       string `json:"schema"`
	ProjectID    string `json:"project_id"`
	ReviewRef    string `json:"review_ref"`
	ReviewDigest string `json:"review_digest"`
	Choice       string `json:"choice"`
	Provenance   string `json:"provenance"`
	Limit        string `json:"interpretation_limit"`
	RecordedAt   string `json:"recorded_at"`
}

func NewMemoryDeferral(input MemoryDeferralInput) (MemoryDeferral, error) {
	if err := requireExactReadableText("project_id", input.ProjectID, 200); err != nil {
		return MemoryDeferral{}, err
	}
	if input.ReviewRef != MemoryReviewRef {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral review_ref must be %q",
			MemoryReviewRef,
		)
	}
	if err := requireSHA256Digest(input.ReviewDigest); err != nil {
		return MemoryDeferral{}, err
	}
	if input.Choice != DeferStructuredMemoryChoice {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral choice must be %q",
			DeferStructuredMemoryChoice,
		)
	}
	recordedAt := input.RecordedAt.Round(0).UTC()
	if recordedAt.IsZero() {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral recorded_at is required",
		)
	}
	return MemoryDeferral{
		projectID:    input.ProjectID,
		reviewRef:    input.ReviewRef,
		reviewDigest: input.ReviewDigest,
		choice:       input.Choice,
		provenance:   MemoryDeferralProvenance,
		limit:        MemoryDeferralInterpretationLimit,
		recordedAt:   recordedAt,
	}, nil
}

func DecodeMemoryDeferral(content []byte) (MemoryDeferral, error) {
	wire := memoryDeferralWire{}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return MemoryDeferral{}, fmt.Errorf(
			"decode memory deferral: %w",
			err,
		)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral contains trailing material",
		)
	}
	if wire.Schema != MemoryDeferralSchema {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral schema %q is unsupported",
			wire.Schema,
		)
	}
	if wire.Provenance != MemoryDeferralProvenance {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral provenance %q is unsupported",
			wire.Provenance,
		)
	}
	if wire.Limit != MemoryDeferralInterpretationLimit {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral interpretation_limit %q is unsupported",
			wire.Limit,
		)
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, wire.RecordedAt)
	if err != nil {
		return MemoryDeferral{}, fmt.Errorf(
			"parse memory deferral recorded_at: %w",
			err,
		)
	}
	deferral, err := NewMemoryDeferral(
		MemoryDeferralInput{
			ProjectID:    wire.ProjectID,
			ReviewRef:    wire.ReviewRef,
			ReviewDigest: wire.ReviewDigest,
			Choice:       wire.Choice,
			RecordedAt:   recordedAt,
		},
	)
	if err != nil {
		return MemoryDeferral{}, err
	}
	canonical, err := deferral.CanonicalJSON()
	if err != nil {
		return MemoryDeferral{}, err
	}
	if !bytes.Equal(content, canonical) {
		return MemoryDeferral{}, fmt.Errorf(
			"memory deferral bytes are not canonical",
		)
	}
	return deferral, nil
}

func (deferral MemoryDeferral) ProjectID() string {
	return deferral.projectID
}

func (deferral MemoryDeferral) ReviewRef() string {
	return deferral.reviewRef
}

func (deferral MemoryDeferral) ReviewDigest() string {
	return deferral.reviewDigest
}

func (deferral MemoryDeferral) Choice() string {
	return deferral.choice
}

func (deferral MemoryDeferral) Provenance() string {
	return deferral.provenance
}

func (deferral MemoryDeferral) InterpretationLimit() string {
	return deferral.limit
}

func (deferral MemoryDeferral) RecordedAt() time.Time {
	return deferral.recordedAt
}

func (deferral MemoryDeferral) CanonicalJSON() ([]byte, error) {
	if deferral.projectID == "" ||
		deferral.reviewRef == "" ||
		deferral.reviewDigest == "" ||
		deferral.choice == "" ||
		deferral.provenance != MemoryDeferralProvenance ||
		deferral.limit != MemoryDeferralInterpretationLimit ||
		deferral.recordedAt.IsZero() {
		return nil, fmt.Errorf("memory deferral is invalid")
	}
	content, err := json.MarshalIndent(
		memoryDeferralWire{
			Schema:       MemoryDeferralSchema,
			ProjectID:    deferral.projectID,
			ReviewRef:    deferral.reviewRef,
			ReviewDigest: deferral.reviewDigest,
			Choice:       deferral.choice,
			Provenance:   deferral.provenance,
			Limit:        deferral.limit,
			RecordedAt:   deferral.recordedAt.Format(time.RFC3339Nano),
		},
		"",
		"  ",
	)
	if err != nil {
		return nil, fmt.Errorf("encode memory deferral: %w", err)
	}
	return append(content, '\n'), nil
}

func requireSHA256Digest(value string) error {
	const prefix = "sha256:"
	if !strings.HasPrefix(value, prefix) {
		return fmt.Errorf(
			"memory deferral review_digest must use the sha256: prefix",
		)
	}
	decoded, err := hex.DecodeString(strings.TrimPrefix(value, prefix))
	if err != nil || len(decoded) != 32 {
		return fmt.Errorf(
			"memory deferral review_digest must contain exactly 32 bytes",
		)
	}
	if value != strings.ToLower(value) {
		return fmt.Errorf(
			"memory deferral review_digest must use lowercase hexadecimal",
		)
	}
	return nil
}
