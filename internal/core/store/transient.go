package store

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/m0n0x41d/haft/internal/core/carrier"
)

const transientLimit = 128 << 20

// PutTransient publishes disposable, content-addressed delivery bytes. They are
// excluded from memory generations and never admitted as project history.
func (s Store) PutTransient(ctx context.Context, raw []byte) (string, error) {
	if len(raw) > transientLimit {
		return "", fmt.Errorf("transient_entry_limit: 128 MiB")
	}
	h, err := s.open(ctx, true)
	if err != nil {
		return "", err
	}
	defer h.close()
	dir := ".cache/disclosure"
	if err = ensureDirs(h.root, dir, 0700); err != nil {
		return "", err
	}
	digest := carrier.Digest(raw)
	p := dir + "/" + strings.TrimPrefix(digest, "sha256:") + ".json"
	if old, e := regularBytes(h.root, p, transientLimit); e == nil {
		if carrier.Digest(old) != digest {
			return "", fmt.Errorf("corrupt: cached result digest mismatch")
		}
		return "result:" + digest, nil
	} else if !os.IsNotExist(e) {
		return "", e
	}
	entries, err := h.root.Open(dir)
	if err != nil {
		return "", err
	}
	infos, err := entries.Readdir(-1)
	entries.Close()
	if err != nil {
		return "", err
	}
	total := len(raw)
	for _, info := range infos {
		total += int(info.Size())
	}
	if total > 512<<20 {
		return "", fmt.Errorf("transient_cache_limit: remove disposable cache to reclaim space; no canonical data is affected")
	}
	if err = publishBytes(h.root, p, raw); err != nil {
		return "", err
	}
	if err = h.checkIdentity(); err != nil {
		return "", err
	}
	return "result:" + digest, nil
}
func (s Store) ReadTransient(ctx context.Context, ref string) ([]byte, error) {
	digest := strings.TrimPrefix(ref, "result:")
	if ref == digest || !carrier.ValidDigest(digest) {
		return nil, fmt.Errorf("invalid_result_ref")
	}
	h, err := s.open(ctx, false)
	if err != nil {
		return nil, err
	}
	defer h.close()
	raw, err := regularBytes(h.root, ".cache/disclosure/"+strings.TrimPrefix(digest, "sha256:")+".json", transientLimit)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("expired: disposable result cache missing; repeat the original operation")
	}
	if err != nil {
		return nil, err
	}
	if carrier.Digest(raw) != digest {
		return nil, fmt.Errorf("corrupt: transient result digest mismatch")
	}
	return raw, h.checkIdentity()
}
