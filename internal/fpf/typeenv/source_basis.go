package typeenv

import (
	"fmt"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

func sourceLocation(unit fpf.SourceUnit) (typedmemory.SourceLocation, error) {
	unitID, err := typedmemory.NewSourceUnitID(unit.UnitID)
	if err != nil {
		return typedmemory.SourceLocation{}, fmt.Errorf("source unit identity: %w", err)
	}
	revision, err := typedmemory.NewSourceRevision(unit.Provenance.SourceRevision)
	if err != nil {
		return typedmemory.SourceLocation{}, fmt.Errorf("source revision for %s: %w", unit.UnitID, err)
	}
	digest, err := sourceContentDigest(unit.Provenance.ContentHash)
	if err != nil {
		return typedmemory.SourceLocation{}, fmt.Errorf("source content hash for %s: %w", unit.UnitID, err)
	}
	lineRange, err := sourceLineRange(unit)
	if err != nil {
		return typedmemory.SourceLocation{}, err
	}

	patternID := strings.TrimSpace(unit.PatternID)
	if patternID == "" {
		return typedmemory.NewUnpatternedSourceLocation(
			unitID,
			revision,
			digest,
			lineRange,
		)
	}
	parsedPatternID, err := typedmemory.NewPatternID(patternID)
	if err != nil {
		return typedmemory.SourceLocation{}, fmt.Errorf("source PatternID for %s: %w", unit.UnitID, err)
	}
	return typedmemory.NewPatternedSourceLocation(
		unitID,
		revision,
		digest,
		lineRange,
		parsedPatternID,
	)
}

func sourceFPFProvenance(
	unit fpf.SourceUnit,
	ruleID typedmemory.CompilerRuleID,
) (typedmemory.FPFSourceProvenance, error) {
	location, err := sourceLocation(unit)
	if err != nil {
		return typedmemory.FPFSourceProvenance{}, err
	}
	reference, err := typedmemory.NewProvenanceRef(
		"fpf-source:" + unit.UnitID + ":" + ruleID.String(),
	)
	if err != nil {
		return typedmemory.FPFSourceProvenance{}, fmt.Errorf("source provenance reference: %w", err)
	}
	return typedmemory.NewFPFSourceProvenance(reference, location, ruleID)
}

func sourceContentDigest(raw string) (typedmemory.SHA256Digest, error) {
	value := strings.TrimSpace(raw)
	if !strings.HasPrefix(value, "sha256:") {
		value = "sha256:" + value
	}
	return typedmemory.NewSHA256Digest(value)
}

func sourceLineRange(unit fpf.SourceUnit) (typedmemory.SourceLineRange, error) {
	if unit.Provenance.StartLine <= 0 || unit.Provenance.EndLine <= 0 {
		return typedmemory.SourceLineRange{}, fmt.Errorf("source line range for %s must be positive", unit.UnitID)
	}
	lineRange, err := typedmemory.NewSourceLineRange(
		uint64(unit.Provenance.StartLine),
		uint64(unit.Provenance.EndLine),
	)
	if err != nil {
		return typedmemory.SourceLineRange{}, fmt.Errorf("source line range for %s: %w", unit.UnitID, err)
	}
	return lineRange, nil
}
