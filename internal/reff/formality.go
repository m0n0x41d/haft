package reff

import "strings"

const (
	FormalityScaleCurrent     = "fpf-2026-f0-f9"
	FormalityScaleLegacy      = "haft-legacy-f0-f3"
	FormalityScaleUnversioned = "unversioned-formality"

	FormalityBridgeNoLoss         = "none"
	FormalityBridgeLegacyLoss     = "legacy-scale-has-fewer-buckets"
	FormalityBridgeUnversionedGap = "source-scale-not-declared"
)

// FormalityScale is the explicit carrier for an evidence formality ordinal.
// Level remains projected to the legacy formality_level integer for old readers.
type FormalityScale struct {
	ScaleID string `json:"scale_id"`
	Level   int    `json:"level"`
}

// FormalityBridge documents how an older or undeclared scale is being read.
// It is diagnostic metadata only; it never grants authority or approval.
type FormalityBridge struct {
	SourceScaleID string `json:"source_scale_id"`
	TargetScaleID string `json:"target_scale_id"`
	SourceLevel   int    `json:"source_level"`
	TargetLevel   int    `json:"target_level"`
	Loss          string `json:"loss"`
	Diagnostic    string `json:"diagnostic"`
}

func CurrentFormalityScale(level int) FormalityScale {
	return FormalityScale{
		ScaleID: FormalityScaleCurrent,
		Level:   NormalizeFormalityLevel(level),
	}
}

func LegacyFormalityScale(level int) FormalityScale {
	return FormalityScale{
		ScaleID: FormalityScaleLegacy,
		Level:   NormalizeLegacyFormalityLevel(level),
	}
}

func UnversionedFormalityScale(level int) FormalityScale {
	return FormalityScale{
		ScaleID: FormalityScaleUnversioned,
		Level:   NormalizeFormalityLevel(level),
	}
}

func NormalizeFormalityScale(scale FormalityScale) FormalityScale {
	scaleID := NormalizeFormalityScaleID(scale.ScaleID)
	switch scaleID {
	case FormalityScaleLegacy:
		return LegacyFormalityScale(scale.Level)
	case FormalityScaleUnversioned:
		return UnversionedFormalityScale(scale.Level)
	default:
		return CurrentFormalityScale(scale.Level)
	}
}

func NormalizeFormalityScaleID(scaleID string) string {
	normalized := strings.ToLower(strings.TrimSpace(scaleID))
	switch normalized {
	case "", FormalityScaleCurrent:
		return FormalityScaleCurrent
	case FormalityScaleLegacy:
		return FormalityScaleLegacy
	case FormalityScaleUnversioned:
		return FormalityScaleUnversioned
	default:
		return FormalityScaleUnversioned
	}
}

func LegacyFormalityBridge(level int) FormalityBridge {
	legacy := LegacyFormalityScale(level)
	targetLevel := NormalizeFormalityLevel(level)
	return FormalityBridge{
		SourceScaleID: legacy.ScaleID,
		TargetScaleID: FormalityScaleCurrent,
		SourceLevel:   legacy.Level,
		TargetLevel:   targetLevel,
		Loss:          FormalityBridgeLegacyLoss,
		Diagnostic:    "legacy Haft F0-F3 value is preserved as a lower-bound reading; it is not silently promoted to current FPF semantics",
	}
}

func UnversionedFormalityBridge(level int) FormalityBridge {
	source := UnversionedFormalityScale(level)
	return FormalityBridge{
		SourceScaleID: source.ScaleID,
		TargetScaleID: FormalityScaleCurrent,
		SourceLevel:   source.Level,
		TargetLevel:   source.Level,
		Loss:          FormalityBridgeUnversionedGap,
		Diagnostic:    "stored formality_level has no declared scale_id; ordinal is preserved but source semantics are undeclared",
	}
}

func NormalizeLegacyFormalityLevel(level int) int {
	switch {
	case level < 0:
		return 0
	case level <= 3:
		return level
	default:
		return 3
	}
}

// NormalizeFormalityLevel preserves the current FPF F0-F9 ordinal directly.
// Values outside the scale clamp at the nearest endpoint.
func NormalizeFormalityLevel(level int) int {
	switch {
	case level < 0:
		return 0
	case level <= 9:
		return level
	default:
		return 9
	}
}
