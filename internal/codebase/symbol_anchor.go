package codebase

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

const SymbolAnchorVersion = 2

// SymbolAnchor is the durable identity of one code declaration inside a
// project-scoped code graph. Source coordinates and body hashes deliberately do
// not participate: inserting lines or editing an implementation must not mint a
// new node. The project database supplies the repository namespace.
type SymbolAnchor struct {
	Version       int
	ID            string
	FilePath      string
	Language      string
	Kind          string
	QualifiedName string
	SignatureHash string
}

// BuildSymbolAnchor is the pure identity core. It accepts a normalized snapshot
// and returns the one canonical anchor representation used by persistence.
func BuildSymbolAnchor(snapshot SymbolSnapshot, language string) SymbolAnchor {
	normalized := normalizeSymbolSnapshotIdentity(snapshot)
	anchor := SymbolAnchor{
		Version:       SymbolAnchorVersion,
		FilePath:      normalizeAnchorPath(normalized.FilePath),
		Language:      strings.TrimSpace(language),
		Kind:          strings.TrimSpace(normalized.SymbolKind),
		QualifiedName: strings.TrimSpace(normalized.QualifiedName),
		SignatureHash: strings.TrimSpace(normalized.SignatureHash),
	}
	anchor.ID = symbolAnchorID(anchor)
	return anchor
}

func normalizeSymbolSnapshots(snapshots []SymbolSnapshot) []SymbolSnapshot {
	out := make([]SymbolSnapshot, 0, len(snapshots))
	for _, snapshot := range snapshots {
		out = append(out, normalizeSymbolSnapshotIdentity(snapshot))
	}
	return out
}

func normalizeSymbolSnapshotIdentity(snapshot SymbolSnapshot) SymbolSnapshot {
	snapshot.FilePath = normalizeAnchorPath(snapshot.FilePath)
	snapshot.SymbolName = strings.TrimSpace(snapshot.SymbolName)
	snapshot.SymbolKind = strings.TrimSpace(snapshot.SymbolKind)
	snapshot.Receiver = strings.TrimSpace(snapshot.Receiver)
	if strings.TrimSpace(snapshot.QualifiedName) == "" {
		snapshot.QualifiedName = qualifiedSymbolName(snapshot.Receiver, snapshot.SymbolName)
	}
	if strings.TrimSpace(snapshot.SignatureHash) == "" {
		snapshot.SignatureHash = signatureHash(snapshot.SymbolKind, snapshot.QualifiedName, "")
	}
	return snapshot
}

func qualifiedSymbolName(receiver, name string) string {
	receiver = strings.TrimSpace(receiver)
	name = strings.TrimSpace(name)
	if receiver == "" {
		return name
	}
	return receiver + "." + name
}

// signatureHashFromDeclaration distinguishes callable overloads while excluding
// implementation bodies. Non-callable declarations use kind+qualified-name as
// their stable signature because their shape belongs to drift, not identity.
func signatureHashFromDeclaration(kind, qualifiedName string, declaration []byte) string {
	header := callableDeclarationHeader(kind, declaration)
	normalizedHeader := strings.Join(strings.Fields(string(header)), " ")
	return signatureHash(kind, qualifiedName, normalizedHeader)
}

func signatureHash(kind, qualifiedName, normalizedHeader string) string {
	parts := []string{
		"symbol-signature-v1",
		strings.TrimSpace(kind),
		strings.TrimSpace(qualifiedName),
		strings.TrimSpace(normalizedHeader),
	}
	payload := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func callableDeclarationHeader(kind string, declaration []byte) []byte {
	if !callableSymbolKind(kind) {
		return nil
	}
	boundary := callableBodyBoundary(declaration)
	return declaration[:boundary]
}

func callableSymbolKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "func", "function", "method":
		return true
	default:
		return false
	}
}

func callableBodyBoundary(declaration []byte) int {
	parenDepth := 0
	bracketDepth := 0
	quote := byte(0)
	escaped := false
	for index, current := range declaration {
		if quote != 0 {
			switch {
			case escaped:
				escaped = false
			case current == '\\':
				escaped = true
			case current == quote:
				quote = 0
			}
			continue
		}
		switch current {
		case '\'', '"', '`':
			quote = current
		case '(':
			parenDepth++
		case ')':
			if parenDepth > 0 {
				parenDepth--
			}
		case '[':
			bracketDepth++
		case ']':
			if bracketDepth > 0 {
				bracketDepth--
			}
		case '=':
			if parenDepth == 0 && bracketDepth == 0 && index+1 < len(declaration) && declaration[index+1] == '>' {
				return index
			}
		case '{':
			if parenDepth == 0 && bracketDepth == 0 {
				return index
			}
		}
	}
	return len(declaration)
}

func symbolAnchorID(anchor SymbolAnchor) string {
	parts := []string{
		fmt.Sprintf("v%d", anchor.Version),
		anchor.FilePath,
		anchor.Language,
		anchor.Kind,
		anchor.QualifiedName,
		anchor.SignatureHash,
	}
	payload := strings.Join(parts, "\x00")
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("sym:v%d:%s", anchor.Version, hex.EncodeToString(sum[:]))
}

func normalizeAnchorPath(path string) string {
	cleaned := filepath.Clean(strings.TrimSpace(path))
	if cleaned == "." {
		return ""
	}
	return filepath.ToSlash(cleaned)
}
