package typeenv

import (
	"fmt"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const baseTypeEnvCompilerSchema = "fpf-base-typeenv.cov2.v5"

// BaseTypeEnvCompilation is the closed result family for source compilation.
// Source rejection is a normal result; implementation/invariant failures are
// returned as Go errors by CompileBaseTypeEnv.
type BaseTypeEnvCompilation interface {
	SourceRevision() typedmemory.SourceRevision
	CompilerSchemaVersion() typedmemory.CompilerSchemaVersion
	Diagnostics() []CompilerDiagnostic
	Artifact() (BaseTypeEnvArtifact, bool)
	Environment() (typedmemory.TypeEnv, bool)
	CodecRegistry() (typedmemory.CodecRegistry, bool)
	Rejected() bool
	baseTypeEnvCompilationVariant()
}

type compilationAccepted struct {
	artifact    BaseTypeEnvArtifact
	environment typedmemory.TypeEnv
	codecs      typedmemory.CodecRegistry
}

func (result compilationAccepted) SourceRevision() typedmemory.SourceRevision {
	return result.artifact.SourceRevision()
}

func (result compilationAccepted) CompilerSchemaVersion() typedmemory.CompilerSchemaVersion {
	return result.artifact.CompilerSchemaVersion()
}

func (compilationAccepted) Diagnostics() []CompilerDiagnostic { return nil }

func (compilationAccepted) Rejected() bool { return false }

func (compilationAccepted) baseTypeEnvCompilationVariant() {}

func (result compilationAccepted) Artifact() (BaseTypeEnvArtifact, bool) {
	return result.artifact, true
}

func (result compilationAccepted) Environment() (typedmemory.TypeEnv, bool) {
	return result.environment, true
}

func (result compilationAccepted) CodecRegistry() (typedmemory.CodecRegistry, bool) {
	return result.codecs, true
}

type compilationRejected struct {
	revision    typedmemory.SourceRevision
	compiler    typedmemory.CompilerSchemaVersion
	diagnostics []CompilerDiagnostic
}

func (result compilationRejected) SourceRevision() typedmemory.SourceRevision {
	return result.revision
}

func (result compilationRejected) CompilerSchemaVersion() typedmemory.CompilerSchemaVersion {
	return result.compiler
}

func (result compilationRejected) Diagnostics() []CompilerDiagnostic {
	return append([]CompilerDiagnostic(nil), result.diagnostics...)
}

func (compilationRejected) Rejected() bool { return true }

func (compilationRejected) baseTypeEnvCompilationVariant() {}

func (compilationRejected) Artifact() (BaseTypeEnvArtifact, bool) {
	return BaseTypeEnvArtifact{}, false
}

func (compilationRejected) Environment() (typedmemory.TypeEnv, bool) {
	return typedmemory.TypeEnv{}, false
}

func (compilationRejected) CodecRegistry() (typedmemory.CodecRegistry, bool) {
	return typedmemory.NewCodecRegistry(), false
}

// CompileBaseTypeEnv compiles exactly one PublicationSnapshot. Query and this
// compiler can therefore share both publication bytes and source provenance.
func CompileBaseTypeEnv(snapshot fpf.PublicationSnapshot) (BaseTypeEnvCompilation, error) {
	revision, err := typedmemory.NewSourceRevision(snapshot.Revision())
	if err != nil {
		return nil, fmt.Errorf("base TypeEnv source revision: %w", err)
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion(baseTypeEnvCompilerSchema)
	if err != nil {
		return nil, fmt.Errorf("base TypeEnv compiler schema: %w", err)
	}
	return compileSourceUnits(revision, compiler, snapshot.SourceUnits())
}

func compileSourceUnits(
	revision typedmemory.SourceRevision,
	compiler typedmemory.CompilerSchemaVersion,
	units []fpf.SourceUnit,
) (BaseTypeEnvCompilation, error) {
	declarations, diagnostics := parseStructuralUnits(units)
	diagnostics = append(
		diagnostics,
		auditStructuralCompleteness(units, declarations, diagnostics)...,
	)
	sortCompilerDiagnostics(diagnostics)
	if len(diagnostics) > 0 {
		result := compilationRejected{
			revision:    revision,
			compiler:    compiler,
			diagnostics: diagnostics,
		}
		return result, nil
	}

	scopeGaps, err := publicationScopeCoverage(units)
	if err != nil {
		return nil, fmt.Errorf("derive base TypeEnv coverage scope: %w", err)
	}
	artifact, err := linkStructuralDeclarations(revision, compiler, declarations, scopeGaps)
	if err != nil {
		return nil, fmt.Errorf("link source-derived base TypeEnv: %w", err)
	}
	environment, codecs, err := LowerBaseTypeEnvArtifactWithCodecs(artifact)
	if err != nil {
		return nil, fmt.Errorf("lower source-derived base TypeEnv: %w", err)
	}
	result := compilationAccepted{
		artifact:    artifact,
		environment: environment,
		codecs:      codecs,
	}
	return result, nil
}

func parseStructuralUnits(
	units []fpf.SourceUnit,
) ([]StructuralDeclaration, []CompilerDiagnostic) {
	declarations := make([]StructuralDeclaration, 0)
	diagnostics := make([]CompilerDiagnostic, 0)
	for _, unit := range units {
		outcome := ParseStructuralUnit(unit)
		switch parsed := outcome.(type) {
		case GrammarNoMatch:
			continue
		case GrammarParsed:
			declarations = append(declarations, parsed.Declarations()...)
		case GrammarMalformed:
			diagnostics = append(diagnostics, parsed.Diagnostics()...)
		}
	}
	sort.Slice(diagnostics, func(left, right int) bool {
		leftKey := diagnosticKey(diagnostics[left])
		rightKey := diagnosticKey(diagnostics[right])
		return leftKey < rightKey
	})
	return declarations, diagnostics
}

type structuralFamily uint8

const (
	familySlotSpecProduction structuralFamily = iota + 1
	familySlotRule
	familyRelationRoot
	familySlotFragment
	familyRelationProfile
	familySymbolicRelationSignature
	familySymbolicRelationSemantics
	familySubkindRelationContract
	familySubkindOrderContract
	familyKindSignatureContract
	familyKindClassificationJudgementContract
	familyKindExtensionContract
	familyKindBridgeContract
	familyRoleMaskContract
	familyKindGuardSeparationContract
)

func (family structuralFamily) String() string {
	switch family {
	case familySlotSpecProduction:
		return "slot_spec_production"
	case familySlotRule:
		return "slot_rule"
	case familyRelationRoot:
		return "relation_root"
	case familySlotFragment:
		return "slot_fragment"
	case familyRelationProfile:
		return "relation_profile"
	case familySymbolicRelationSignature:
		return "symbolic_relation_signature"
	case familySymbolicRelationSemantics:
		return "symbolic_relation_semantics"
	case familySubkindRelationContract:
		return "subkind_relation_contract"
	case familySubkindOrderContract:
		return "subkind_order_contract"
	case familyKindSignatureContract:
		return "kind_signature_contract"
	case familyKindClassificationJudgementContract:
		return "kind_classification_judgement_contract"
	case familyKindExtensionContract:
		return "kind_extension_contract"
	case familyKindBridgeContract:
		return "kind_bridge_contract"
	case familyRoleMaskContract:
		return "role_mask_contract"
	case familyKindGuardSeparationContract:
		return "kind_guard_separation_contract"
	default:
		return ""
	}
}

var structuralCompletenessContract = map[string]map[structuralFamily]int{
	"A.6.5": {
		familySlotSpecProduction: 1,
		familySlotRule:           7,
	},
	"C.2.1": {
		familySymbolicRelationSignature: 3,
		familySymbolicRelationSemantics: 3,
	},
	"C.3.1": {
		familySubkindRelationContract: 1,
		familySubkindOrderContract:    1,
	},
	"C.3.2": {
		familyKindSignatureContract:               1,
		familyKindClassificationJudgementContract: 1,
		familyKindExtensionContract:               1,
	},
	"C.3.3": {
		familyKindBridgeContract: 1,
	},
	"C.3.4": {
		familyRoleMaskContract: 1,
	},
	"C.3.A": {
		familyKindGuardSeparationContract: 1,
	},
}

func auditStructuralCompleteness(
	units []fpf.SourceUnit,
	declarations []StructuralDeclaration,
	existing []CompilerDiagnostic,
) []CompilerDiagnostic {
	malformedOwners := malformedPatternOwners(units, existing)
	counts := map[string]map[structuralFamily]int{}
	for _, declaration := range declarations {
		owner := strings.TrimSpace(declaration.Source().ParentPatternID)
		family := declarationFamily(declaration)
		if owner == "" || family.String() == "" {
			continue
		}
		if counts[owner] == nil {
			counts[owner] = map[structuralFamily]int{}
		}
		counts[owner][family]++
	}
	diagnostics := make([]CompilerDiagnostic, 0)
	patterns := make([]string, 0, len(structuralCompletenessContract))
	for pattern := range structuralCompletenessContract {
		patterns = append(patterns, pattern)
	}
	sort.Strings(patterns)
	for _, pattern := range patterns {
		if _, malformed := malformedOwners[pattern]; malformed {
			continue
		}
		families := structuralCompletenessContract[pattern]
		for _, family := range sortedStructuralFamilies(families) {
			want := families[family]
			got := counts[pattern][family]
			if got == want {
				continue
			}
			unitID := patternAuditUnitID(units, pattern)
			message := fmt.Sprintf(
				"publication adapter expected %d %s declarations under %s, found %d",
				want,
				family.String(),
				pattern,
				got,
			)
			diagnostic, err := NewCompilerDiagnostic(
				"structural_declaration_family_count_mismatch",
				unitID,
				message,
			)
			if err == nil {
				diagnostics = append(diagnostics, diagnostic)
			}
		}
	}
	return diagnostics
}

func declarationFamily(declaration StructuralDeclaration) structuralFamily {
	switch typed := declaration.(type) {
	case SlotSpecProductionDeclaration:
		return familySlotSpecProduction
	case SlotRuleDeclaration:
		return familySlotRule
	case RelationRootDeclaration:
		return familyRelationRoot
	case SlotDeclarationFragment:
		return familySlotFragment
	case RelationProfileDeclaration:
		return familyRelationProfile
	case SymbolicRelationSignatureDeclaration:
		return familySymbolicRelationSignature
	case SymbolicRelationSemanticsDeclaration:
		return familySymbolicRelationSemantics
	case C3ContractDeclaration:
		return c3ContractStructuralFamily(typed.Kind())
	default:
		return 0
	}
}

func c3ContractStructuralFamily(kind C3ContractKind) structuralFamily {
	switch kind {
	case C3SubkindRelationContract:
		return familySubkindRelationContract
	case C3SubkindOrderContract:
		return familySubkindOrderContract
	case C3KindSignatureContract:
		return familyKindSignatureContract
	case C3KindClassificationJudgementContract:
		return familyKindClassificationJudgementContract
	case C3KindExtensionContract:
		return familyKindExtensionContract
	case C3KindBridgeContract:
		return familyKindBridgeContract
	case C3RoleMaskContract:
		return familyRoleMaskContract
	case C3KindGuardSeparationContract:
		return familyKindGuardSeparationContract
	default:
		return 0
	}
}

func sortedStructuralFamilies(expectations map[structuralFamily]int) []structuralFamily {
	families := make([]structuralFamily, 0, len(expectations))
	for family := range expectations {
		families = append(families, family)
	}
	sort.Slice(families, func(left, right int) bool {
		return families[left].String() < families[right].String()
	})
	return families
}

func malformedPatternOwners(
	units []fpf.SourceUnit,
	diagnostics []CompilerDiagnostic,
) map[string]struct{} {
	unitsByID := make(map[string]fpf.SourceUnit, len(units))
	for _, unit := range units {
		unitsByID[unit.UnitID] = unit
	}
	owners := map[string]struct{}{}
	for _, diagnostic := range diagnostics {
		unit, exists := unitsByID[diagnostic.UnitID()]
		if !exists || unit.ParentPatternID == "" {
			continue
		}
		owners[unit.ParentPatternID] = struct{}{}
	}
	return owners
}

func patternAuditUnitID(units []fpf.SourceUnit, pattern string) string {
	for _, unit := range units {
		if unit.Role == fpf.SourceUnitRolePatternBody && unit.PatternID == pattern {
			return unit.UnitID
		}
	}
	for _, unit := range units {
		if unit.Role == fpf.SourceUnitRolePatternSection && unit.ParentPatternID == pattern {
			return unit.UnitID
		}
	}
	return "publication-pattern:" + pattern
}

func sortCompilerDiagnostics(diagnostics []CompilerDiagnostic) {
	sort.Slice(diagnostics, func(left, right int) bool {
		return diagnosticKey(diagnostics[left]) < diagnosticKey(diagnostics[right])
	})
}

func diagnosticKey(diagnostic CompilerDiagnostic) string {
	return diagnostic.UnitID() + "\x00" + diagnostic.Code() + "\x00" + diagnostic.Message()
}
