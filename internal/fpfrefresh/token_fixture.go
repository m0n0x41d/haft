package fpfrefresh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	// TokenGateFixtureSchemaVersion identifies the human-reviewed behavior
	// corpus whose exact bytes are bound by the generated integration lock.
	TokenGateFixtureSchemaVersion = "haft.fpf-query-token-gate-corpus/v1" // #nosec G101 -- schema identifier, not a credential.

	maxTokenGateFixtureBytes    = 1 << 20
	maxTokenGateFixtureRevision = 256
)

type tokenGateFixtureEnvelope struct {
	SchemaVersion   string            `json:"schema_version"`
	FixtureRevision string            `json:"fixture_revision"`
	Cases           []json.RawMessage `json:"cases"`
}

// ReadTokenGateCoordinates validates the small behavior-fixture envelope and
// returns the exact byte identity recorded in an integration lock. It does not
// interpret query expectations or token thresholds; the token-gate test owns
// those acceptance semantics.
func ReadTokenGateCoordinates(
	path string,
) (coordinates TokenGateCoordinates, resultErr error) {
	file, err := os.Open(path)
	if err != nil {
		return TokenGateCoordinates{}, fmt.Errorf("inspect token-gate fixture: %w", err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("close token-gate fixture: %w", err),
			)
		}
	}()
	info, err := file.Stat()
	if err != nil {
		return TokenGateCoordinates{}, fmt.Errorf("inspect token-gate fixture: %w", err)
	}
	if !info.Mode().IsRegular() {
		return TokenGateCoordinates{}, fmt.Errorf("token-gate fixture %q is not a regular file", path)
	}
	if info.Size() <= 0 || info.Size() > maxTokenGateFixtureBytes {
		return TokenGateCoordinates{}, fmt.Errorf(
			"token-gate fixture size %d must be between 1 and %d bytes",
			info.Size(),
			maxTokenGateFixtureBytes,
		)
	}
	payload, err := readBoundedTokenGateFixture(file)
	if err != nil {
		return TokenGateCoordinates{}, err
	}
	envelope, err := parseTokenGateFixtureEnvelope(payload)
	if err != nil {
		return TokenGateCoordinates{}, err
	}
	return TokenGateCoordinates{
		FixtureRevision: envelope.FixtureRevision,
		FixtureDigest:   digestBytesSHA256(payload),
	}, nil
}

func readBoundedTokenGateFixture(reader io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(reader, maxTokenGateFixtureBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read token-gate fixture: %w", err)
	}
	if len(payload) > maxTokenGateFixtureBytes {
		return nil, fmt.Errorf(
			"token-gate fixture exceeds %d bytes",
			maxTokenGateFixtureBytes,
		)
	}
	return payload, nil
}

func parseTokenGateFixtureEnvelope(payload []byte) (tokenGateFixtureEnvelope, error) {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	var envelope tokenGateFixtureEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return tokenGateFixtureEnvelope{}, fmt.Errorf("decode token-gate fixture: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return tokenGateFixtureEnvelope{}, fmt.Errorf(
				"decode token-gate fixture: trailing JSON value",
			)
		}
		return tokenGateFixtureEnvelope{}, fmt.Errorf(
			"decode token-gate fixture trailing data: %w",
			err,
		)
	}
	if envelope.SchemaVersion != TokenGateFixtureSchemaVersion {
		return tokenGateFixtureEnvelope{}, fmt.Errorf(
			"token-gate fixture schema_version=%q, want %q",
			envelope.SchemaVersion,
			TokenGateFixtureSchemaVersion,
		)
	}
	revision := strings.TrimSpace(envelope.FixtureRevision)
	if revision == "" ||
		revision != envelope.FixtureRevision ||
		len(revision) > maxTokenGateFixtureRevision {
		return tokenGateFixtureEnvelope{}, fmt.Errorf(
			"token-gate fixture_revision must be non-empty, trimmed, and at most %d bytes",
			maxTokenGateFixtureRevision,
		)
	}
	if len(envelope.Cases) == 0 {
		return tokenGateFixtureEnvelope{}, fmt.Errorf(
			"token-gate fixture must contain at least one behavior case",
		)
	}
	for index, testCase := range envelope.Cases {
		if len(bytes.TrimSpace(testCase)) == 0 ||
			bytes.Equal(bytes.TrimSpace(testCase), []byte("null")) {
			return tokenGateFixtureEnvelope{}, fmt.Errorf(
				"token-gate fixture case %d must be a JSON object",
				index,
			)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(testCase, &object); err != nil || object == nil {
			return tokenGateFixtureEnvelope{}, fmt.Errorf(
				"token-gate fixture case %d must be a JSON object",
				index,
			)
		}
	}
	return envelope, nil
}
