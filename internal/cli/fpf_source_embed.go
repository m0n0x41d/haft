package cli

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/m0n0x41d/haft/internal/fpf"
)

// This source-access archive has its own publication manifest and preserves
// the compiler rejection and exact memory basis inside the same atomic file.
// The memory runtime continues to read embeddedFPFDB through openFPFDBContext.
//
//go:embed fpf-source.db.gz
var embeddedFPFSourceArchive []byte

func openFPFSourceDB() (*sql.DB, func(), error) {
	reader, err := gzip.NewReader(bytes.NewReader(embeddedFPFSourceArchive))
	if err != nil {
		return nil, nil, fmt.Errorf("open embedded source archive: %w", err)
	}
	payload, err := io.ReadAll(reader)
	closeErr := reader.Close()
	if err != nil {
		return nil, nil, err
	}
	if closeErr != nil {
		return nil, nil, closeErr
	}
	database, cleanup, err := openFPFDBImage(context.Background(), payload)
	if err != nil {
		return nil, nil, err
	}
	digest := sha256.Sum256(embeddedFPFDB)
	memoryDigest := "sha256:" + hex.EncodeToString(digest[:])
	if err := fpf.VerifySourceAccessBasisDB(database, memoryDigest); err != nil {
		cleanup()
		return nil, nil, err
	}
	return database, cleanup, nil
}
