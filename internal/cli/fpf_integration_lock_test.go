package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/m0n0x41d/haft/internal/fpfrefresh"
)

func TestEmbeddedFPFSnapshotMatchesGeneratedIntegrationLock(t *testing.T) {
	root := filepath.Join("..", "..")
	lockPath := filepath.Join(root, fpfrefresh.DefaultIntegrationLockRelativePath)
	lockPayload, err := os.ReadFile(lockPath)
	if err != nil {
		t.Fatalf("read generated FPF integration lock: %v", err)
	}
	lock, err := fpfrefresh.ParseIntegrationLock(lockPayload)
	if err != nil {
		t.Fatalf("parse generated FPF integration lock: %v", err)
	}

	tokenGate, err := fpfrefresh.ReadTokenGateCoordinates(
		filepath.Join(root, fpfrefresh.DefaultTokenGateFixtureRelativePath),
	)
	if err != nil {
		t.Fatalf("read token-gate fixture identity: %v", err)
	}
	if lock.TokenGate == nil || *lock.TokenGate != tokenGate {
		t.Fatalf(
			"integration lock token fixture = %#v, source fixture = %#v",
			lock.TokenGate,
			tokenGate,
		)
	}

	digest := sha256.Sum256(embeddedFPFDB)
	embeddedDigest := "sha256:" + hex.EncodeToString(digest[:])
	if embeddedDigest != lock.Coordinates.DatabaseDigest {
		t.Fatalf(
			"embedded FPF database digest = %s, integration lock = %s",
			embeddedDigest,
			lock.Coordinates.DatabaseDigest,
		)
	}

	databasePath := filepath.Join(t.TempDir(), "fpf.db")
	if err := os.WriteFile(databasePath, embeddedFPFDB, 0o600); err != nil {
		t.Fatal(err)
	}
	input := fpfrefresh.IntegrationCoordinateInput{
		SourceRevision: lock.Coordinates.SourceRevision,
		ReadmePath: filepath.Join(
			root,
			fpfrefresh.DefaultSourceRelativePath,
			"Readme.md",
		),
		SpecPath: filepath.Join(
			root,
			fpfrefresh.DefaultSourceRelativePath,
			"FPF-Spec.md",
		),
		DatabasePath: databasePath,
		GeneratedBy:  lock.GeneratedBy,
		TokenGate:    &tokenGate,
	}
	if err := fpfrefresh.VerifyIntegrationLock(lock, input); err != nil {
		t.Fatalf("verify embedded source/DB/TypeEnv coordinates: %v", err)
	}
}
