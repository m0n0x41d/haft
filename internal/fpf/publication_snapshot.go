package fpf

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// SourceDocumentDigest identifies the exact bytes of one source publication.
// It is intentionally distinct from SourceProvenance.ContentHash, which hashes
// the trimmed body of one derived SourceUnit.
type SourceDocumentDigest struct {
	value string
}

func (digest SourceDocumentDigest) String() string {
	return digest.value
}

func (digest SourceDocumentDigest) Equal(other SourceDocumentDigest) bool {
	return digest.value == other.value
}

// PublicationSnapshot owns one exact pair of FPF publications and every
// SourceUnit derived from that pair. Its fields stay private so callers cannot
// mutate the source basis shared by Query and later source compilers.
type PublicationSnapshot struct {
	revision        string
	readme          SourceDocument
	spec            SourceDocument
	readmeDigest    SourceDocumentDigest
	specDigest      SourceDocumentDigest
	sourceUnits     []SourceUnit
	sourceUnitsByID map[string]SourceUnit
}

// LoadPublicationSnapshot reads each upstream publication exactly once and
// resolves the revision before any derived source representation is built.
func LoadPublicationSnapshot(readmePath, specPath, sourceRevision string) (PublicationSnapshot, error) {
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		return PublicationSnapshot{}, fmt.Errorf("read FPF readme source: %w", err)
	}

	spec, err := os.ReadFile(specPath)
	if err != nil {
		return PublicationSnapshot{}, fmt.Errorf("read FPF specification source: %w", err)
	}

	revision := resolvePublicationRevision(sourceRevision, spec)
	bundle := SourceBundle{
		Readme: SourceDocument{
			Path:           filepath.Clean(readmePath),
			SourceRevision: revision,
			Markdown:       readme,
		},
		Spec: SourceDocument{
			Path:           filepath.Clean(specPath),
			SourceRevision: revision,
			Markdown:       spec,
		},
	}
	return BuildPublicationSnapshot(bundle)
}

// BuildPublicationSnapshot parses the source-owned publication grammar once
// and seals the exact input bytes together with the resulting SourceUnits.
func BuildPublicationSnapshot(bundle SourceBundle) (PublicationSnapshot, error) {
	return buildPublicationSnapshot(bundle, BuildSourceUnits)
}

type sourceUnitBuilder func(SourceBundle) ([]SourceUnit, error)

func buildPublicationSnapshot(bundle SourceBundle, build sourceUnitBuilder) (PublicationSnapshot, error) {
	if err := validateSourceBundle(bundle); err != nil {
		return PublicationSnapshot{}, err
	}

	ownedBundle := cloneSourceBundle(bundle)
	units, err := build(cloneSourceBundle(ownedBundle))
	if err != nil {
		return PublicationSnapshot{}, fmt.Errorf("build publication source units: %w", err)
	}

	ownedUnits := cloneSourceUnits(units)
	unitsByID, err := indexPublicationSourceUnits(ownedUnits)
	if err != nil {
		return PublicationSnapshot{}, err
	}

	return PublicationSnapshot{
		revision:        ownedBundle.Spec.SourceRevision,
		readme:          ownedBundle.Readme,
		spec:            ownedBundle.Spec,
		readmeDigest:    digestSourceDocument(ownedBundle.Readme),
		specDigest:      digestSourceDocument(ownedBundle.Spec),
		sourceUnits:     ownedUnits,
		sourceUnitsByID: unitsByID,
	}, nil
}

func (snapshot PublicationSnapshot) Revision() string {
	return snapshot.revision
}

func (snapshot PublicationSnapshot) Readme() SourceDocument {
	return cloneSourceDocument(snapshot.readme)
}

func (snapshot PublicationSnapshot) Spec() SourceDocument {
	return cloneSourceDocument(snapshot.spec)
}

func (snapshot PublicationSnapshot) SourceBundle() SourceBundle {
	return SourceBundle{
		Readme: snapshot.Readme(),
		Spec:   snapshot.Spec(),
	}
}

func (snapshot PublicationSnapshot) ReadmeDigest() SourceDocumentDigest {
	return snapshot.readmeDigest
}

func (snapshot PublicationSnapshot) SpecDigest() SourceDocumentDigest {
	return snapshot.specDigest
}

func (snapshot PublicationSnapshot) SourceUnits() []SourceUnit {
	return cloneSourceUnits(snapshot.sourceUnits)
}

// ResolveSourceUnit performs exact UnitID resolution. It deliberately does not
// trim or case-fold identifiers because that would broaden source identity.
func (snapshot PublicationSnapshot) ResolveSourceUnit(unitID string) (SourceUnit, bool) {
	unit, ok := snapshot.sourceUnitsByID[unitID]
	if !ok {
		return SourceUnit{}, false
	}
	return cloneSourceUnit(unit), true
}

func digestSourceDocument(document SourceDocument) SourceDocumentDigest {
	digest := sha256.Sum256(document.Markdown)
	encoded := hex.EncodeToString(digest[:])
	return SourceDocumentDigest{value: "sha256:" + encoded}
}

func cloneSourceBundle(bundle SourceBundle) SourceBundle {
	return SourceBundle{
		Readme: cloneSourceDocument(bundle.Readme),
		Spec:   cloneSourceDocument(bundle.Spec),
	}
}

func cloneSourceDocument(document SourceDocument) SourceDocument {
	return SourceDocument{
		Path:           document.Path,
		SourceRevision: document.SourceRevision,
		Markdown:       append([]byte(nil), document.Markdown...),
	}
}

func cloneSourceUnits(units []SourceUnit) []SourceUnit {
	cloned := make([]SourceUnit, len(units))
	for index, unit := range units {
		cloned[index] = cloneSourceUnit(unit)
	}
	return cloned
}

func cloneSourceUnit(unit SourceUnit) SourceUnit {
	unit.DirectRefs = append([]string(nil), unit.DirectRefs...)
	unit.Relations = cloneSourceRelations(unit.Relations)
	unit.AuthoredPhrases = append([]string(nil), unit.AuthoredPhrases...)
	unit.Keywords = append([]string(nil), unit.Keywords...)
	return unit
}

func indexPublicationSourceUnits(units []SourceUnit) (map[string]SourceUnit, error) {
	indexed := make(map[string]SourceUnit, len(units))
	for _, unit := range units {
		if _, exists := indexed[unit.UnitID]; exists {
			return nil, fmt.Errorf("publication snapshot has duplicate source unit id %q", unit.UnitID)
		}
		indexed[unit.UnitID] = unit
	}
	return indexed, nil
}
