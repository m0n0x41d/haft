package recordcarrier

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const maximumCanonicalCarrierBytes = 1 << 20

func encodeCanonicalJSON(value any) ([]byte, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode canonical record carrier: %w", err)
	}
	if !utf8.Valid(canonical) {
		return nil, fmt.Errorf("canonical record carrier contains invalid UTF-8")
	}
	if len(canonical) > maximumCanonicalCarrierBytes {
		return nil, fmt.Errorf("canonical record carrier exceeds %d bytes", maximumCanonicalCarrierBytes)
	}
	return canonical, nil
}

func decodeStrictCanonicalJSON(canonical []byte, target any) error {
	if len(canonical) == 0 {
		return fmt.Errorf("canonical record carrier bytes are required")
	}
	if len(canonical) > maximumCanonicalCarrierBytes {
		return fmt.Errorf("canonical record carrier exceeds %d bytes", maximumCanonicalCarrierBytes)
	}
	if !utf8.Valid(canonical) {
		return fmt.Errorf("canonical record carrier contains invalid UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode canonical record carrier: %w", err)
	}
	var trailing any
	err := decoder.Decode(&trailing)
	if err != io.EOF {
		if err == nil {
			return fmt.Errorf("canonical record carrier contains trailing JSON data")
		}
		return fmt.Errorf("canonical record carrier contains trailing data: %w", err)
	}
	return nil
}

func requireCanonicalJSON(canonical []byte, value any) error {
	reencoded, err := encodeCanonicalJSON(value)
	if err != nil {
		return err
	}
	if !bytes.Equal(reencoded, canonical) {
		return fmt.Errorf("record carrier bytes are not canonical")
	}
	return nil
}

func digestCanonical(canonical []byte) typedmemory.SHA256Digest {
	sum := sha256.Sum256(canonical)
	hexDigest := hex.EncodeToString(sum[:])
	digest, _ := typedmemory.NewSHA256Digest("sha256:" + hexDigest)
	return digest
}
