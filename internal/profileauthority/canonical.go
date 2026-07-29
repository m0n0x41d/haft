package profileauthority

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/m0n0x41d/haft/internal/authority"
)

func canonicalDigest(domain string, value any) (authority.Digest, []byte, error) {
	canonical, err := json.Marshal(value)
	if err != nil {
		return authority.Digest{}, nil, fmt.Errorf("encode canonical profile authority value: %w", err)
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte(domain))
	_, _ = hash.Write(canonical)
	raw := "sha256:" + hex.EncodeToString(hash.Sum(nil))
	digest, err := authority.NewDigest(raw)
	if err != nil {
		return authority.Digest{}, nil, err
	}
	return digest, canonical, nil
}

func canonicalTime(value time.Time) time.Time {
	return value.Round(0).UTC()
}

func formatTime(value time.Time) string {
	return canonicalTime(value).Format(time.RFC3339Nano)
}

func validDigest(value authority.Digest) bool {
	_, err := authority.NewDigest(value.String())
	return err == nil
}

func coveredBy(outer authority.TimeWindow, inner authority.TimeWindow) bool {
	return outer.Contains(inner.From()) && !inner.Until().After(outer.Until())
}
