package carrierfamily

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const maximumCanonicalBytes = 4 << 20

func encodeCanonical(value any) ([]byte, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) == 0 || len(encoded) > maximumCanonicalBytes {
		return nil, fmt.Errorf("carrier-family canonical payload is outside size bounds")
	}
	return encoded, nil
}

func decodeCanonical(input []byte, target any) error {
	if len(input) == 0 || len(input) > maximumCanonicalBytes {
		return fmt.Errorf("carrier-family canonical payload is outside size bounds")
	}
	decoder := json.NewDecoder(bytes.NewReader(input))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("carrier-family canonical payload has trailing content")
	}
	exact, err := encodeCanonical(target)
	if err != nil {
		return err
	}
	if !bytes.Equal(exact, input) {
		return fmt.Errorf("carrier-family payload is not canonical JSON")
	}
	return nil
}

func digestCanonical(input []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(input)
	return typedmemory.NewSHA256Digest("sha256:" + hex.EncodeToString(sum[:]))
}

func validSemanticToken(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	for _, char := range value {
		if unicode.IsControl(char) {
			return false
		}
	}
	return true
}
