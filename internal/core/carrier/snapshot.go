package carrier

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const InterpretationVersion = "haft.interpretation/1"

type BasisFile struct {
	Name  string `json:"name"`
	Bytes []byte `json:"bytes_base64"`
}
type InterpretationBasis struct {
	Version   string      `json:"version"`
	Namespace string      `json:"namespace"`
	Terms     *BasisFile  `json:"terms"` // null explicitly records absent term basis.
	Other     []BasisFile `json:"other"`
}
type Snapshot struct {
	Format         string              `json:"format"`
	Raw            []byte              `json:"carrier_bytes_base64"`
	Interpretation InterpretationBasis `json:"interpretation"`
}

func Digest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// NewSnapshot uses fixed struct field order, sorted named bases and one final LF.
// The hash covers these exact output bytes, including all source carrier bytes.
func NewSnapshot(raw []byte, basis InterpretationBasis) (Snapshot, []byte, string, error) {
	if basis.Version == "" {
		basis.Version = InterpretationVersion
	}
	basis.Other = append([]BasisFile{}, basis.Other...)
	sort.Slice(basis.Other, func(i, j int) bool { return basis.Other[i].Name < basis.Other[j].Name })
	s := Snapshot{Format: "haft.snapshot/1", Raw: bytes.Clone(raw), Interpretation: basis}
	if err := validateBasis(basis); err != nil {
		return Snapshot{}, nil, "", err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return Snapshot{}, nil, "", err
	}
	b = append(b, '\n')
	return s, b, Digest(b), nil
}
func ReadSnapshot(raw []byte, expectedDigest string) (Snapshot, error) {
	var s Snapshot
	if !ValidDigest(expectedDigest) || Digest(raw) != expectedDigest {
		return s, fmt.Errorf("snapshot_digest_mismatch: expected %s", expectedDigest)
	}
	if err := decodeSnapshotJSON(raw, &s); err != nil {
		return s, err
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return s, err
	}
	var interpretation map[string]json.RawMessage
	if err := json.Unmarshal(envelope["interpretation"], &interpretation); err != nil {
		return s, fmt.Errorf("snapshot_interpretation_absent")
	}
	for _, field := range []string{"version", "namespace", "terms", "other"} {
		if _, exists := interpretation[field]; !exists {
			return s, fmt.Errorf("snapshot_interpretation_field_absent: %s", field)
		}
	}
	if s.Format != "haft.snapshot/1" {
		return s, fmt.Errorf("unsupported_snapshot_format: %s", s.Format)
	}
	if err := validateBasis(s.Interpretation); err != nil {
		return s, err
	}
	if s.Raw == nil {
		return s, fmt.Errorf("snapshot_carrier_bytes_absent")
	}
	return s, nil
}
func (s Snapshot) Document() Document { return Parse(s.Raw) }
func validateBasis(b InterpretationBasis) error {
	if b.Version != InterpretationVersion {
		return fmt.Errorf("unsupported_interpretation: %s", b.Version)
	}
	seen := map[string]bool{}
	if b.Terms != nil {
		if b.Terms.Name == "" || b.Terms.Bytes == nil {
			return fmt.Errorf("invalid_term_basis")
		}
		seen[b.Terms.Name] = true
	}
	for _, f := range b.Other {
		if f.Name == "" || f.Bytes == nil || seen[f.Name] {
			return fmt.Errorf("invalid_or_duplicate_interpretation_basis: %s", f.Name)
		}
		seen[f.Name] = true
	}
	return nil
}
func decodeSnapshotJSON(raw []byte, dst any) error {
	// JSON duplicate object keys are rejected before normal decoding to avoid
	// making a digest-valid but ambiguous envelope into semantic authority.
	d := json.NewDecoder(bytes.NewReader(raw))
	var scan func() error
	scan = func() error {
		t, err := d.Token()
		if err != nil {
			return err
		}
		switch t {
		case json.Delim('{'):
			seen := map[string]bool{}
			for d.More() {
				k, err := d.Token()
				if err != nil {
					return err
				}
				name, ok := k.(string)
				if !ok || seen[name] {
					return fmt.Errorf("duplicate_or_invalid_snapshot_key: %v", k)
				}
				seen[name] = true
				if err := scan(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		case json.Delim('['):
			for d.More() {
				if err := scan(); err != nil {
					return err
				}
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := scan(); err != nil {
		return fmt.Errorf("snapshot_json: %w", err)
	}
	if _, err := d.Token(); err != io.EOF {
		return fmt.Errorf("snapshot_json: trailing data")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return fmt.Errorf("snapshot_json: %w", err)
	}
	return nil
}

type SourceSnapshot struct {
	Format     string `json:"format"`
	Raw        []byte `json:"source_bytes_base64"`
	Provenance Source `json:"provenance"`
}

// NewSourceSnapshot records exactly the retrieved slice, not an inferred full
// publication. Provenance.SnapshotRef is empty inside its own envelope.
func NewSourceSnapshot(raw []byte, provenance Source) (SourceSnapshot, []byte, string, error) {
	provenance.SnapshotRef = ""
	if provenance.BodyDigest != Digest(raw) {
		return SourceSnapshot{}, nil, "", fmt.Errorf("source_body_digest_mismatch")
	}
	validation := provenance
	validation.SnapshotRef = "sha256:" + string(bytes.Repeat([]byte{'0'}, 64))
	if ds := validateSource(validation, "provenance"); HasErrors(ds) {
		return SourceSnapshot{}, nil, "", fmt.Errorf("invalid_source_provenance: %v", ds)
	}
	s := SourceSnapshot{Format: "haft.source-snapshot/1", Raw: bytes.Clone(raw), Provenance: provenance}
	b, err := json.Marshal(s)
	if err != nil {
		return SourceSnapshot{}, nil, "", err
	}
	b = append(b, '\n')
	return s, b, Digest(b), nil
}
func ReadSourceSnapshot(raw []byte, expectedDigest string) (SourceSnapshot, error) {
	var s SourceSnapshot
	if !ValidDigest(expectedDigest) || Digest(raw) != expectedDigest {
		return s, fmt.Errorf("source_snapshot_digest_mismatch")
	}
	if err := decodeSnapshotJSON(raw, &s); err != nil {
		return s, err
	}
	if s.Format != "haft.source-snapshot/1" {
		return s, fmt.Errorf("unsupported_source_snapshot_format")
	}
	if s.Raw == nil || s.Provenance.BodyDigest != Digest(s.Raw) {
		return s, fmt.Errorf("source_body_digest_mismatch")
	}
	if s.Provenance.SnapshotRef != "" {
		return s, fmt.Errorf("source_snapshot_self_reference")
	}
	p := s.Provenance
	p.SnapshotRef = expectedDigest
	if ds := validateSource(p, "provenance"); HasErrors(ds) {
		return s, fmt.Errorf("invalid_source_provenance: %v", ds)
	}
	return s, nil
}
