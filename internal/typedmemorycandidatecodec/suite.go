package typedmemorycandidatecodec

import "github.com/m0n0x41d/haft/internal/typedmemory"

// Suite is a pure, candidate-only set of mechanisms bound to the exact
// content-derived shape graph. It intentionally exposes no CodecRegistry or
// RuntimeMechanismArtifact: registration and activation are separate work.
type Suite struct {
	shapes               ShapeSet
	text                 TextV1
	evidencePolarity     EvidencePolarityV1
	canonicalInstant     CanonicalInstantV1
	evidenceUseQualifier EvidenceUseQualifierV1
	performedInterval    PerformedIntervalV1
	codeAnchorLocator    CodeAnchorLocatorV1
}

func NewSuite(
	declarations []typedmemory.ValueShapeDeclaration,
) (Suite, error) {
	shapes, err := NewShapeSet(declarations)
	if err != nil {
		return Suite{}, err
	}
	text := TextV1{shape: shapes.text}
	polarity := EvidencePolarityV1{shape: shapes.evidencePolarity}
	instant := CanonicalInstantV1{shape: shapes.canonicalInstant}
	evidence := EvidenceUseQualifierV1{
		shape:    shapes.evidenceUseQualifier,
		polarity: polarity,
	}
	interval := PerformedIntervalV1{
		shape:   shapes.performedInterval,
		instant: instant,
	}
	anchor := CodeAnchorLocatorV1{
		shape: shapes.codeAnchorLocator,
		text:  text,
	}
	return Suite{
		shapes:               shapes,
		text:                 text,
		evidencePolarity:     polarity,
		canonicalInstant:     instant,
		evidenceUseQualifier: evidence,
		performedInterval:    interval,
		codeAnchorLocator:    anchor,
	}, nil
}

func (suite Suite) Shapes() ShapeSet { return suite.shapes }

func (suite Suite) Text() TextV1 { return suite.text }

func (suite Suite) EvidencePolarity() EvidencePolarityV1 {
	return suite.evidencePolarity
}

func (suite Suite) CanonicalInstant() CanonicalInstantV1 {
	return suite.canonicalInstant
}

func (suite Suite) EvidenceUseQualifier() EvidenceUseQualifierV1 {
	return suite.evidenceUseQualifier
}

func (suite Suite) PerformedInterval() PerformedIntervalV1 {
	return suite.performedInterval
}

func (suite Suite) CodeAnchorLocator() CodeAnchorLocatorV1 {
	return suite.codeAnchorLocator
}
