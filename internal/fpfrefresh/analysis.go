package fpfrefresh

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/m0n0x41d/haft/internal/fpf"
	"github.com/m0n0x41d/haft/internal/fpf/localpractice"
	"github.com/m0n0x41d/haft/internal/fpf/typeenv"
	"github.com/m0n0x41d/haft/internal/fpf/typeenvsql"
	"github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

// SnapshotAnalysisInput identifies the two read-only indexes and the latest
// repo-owned Local-Practice candidate whose Base TypeEnv basis must remain
// visible in a refresh report.
type SnapshotAnalysisInput struct {
	PredecessorDatabasePath      string
	CandidateDatabasePath        string
	LatestLocalPracticeCandidate string
}

// SnapshotAnalysis contains source-derived deltas and review diagnostics. It
// describes compatibility evidence only; it carries no approval, selection,
// applicability, activation, or release field.
type SnapshotAnalysis struct {
	Deltas                     []Delta
	Diagnostics                []Diagnostic
	LocalPracticeCompatibility *LocalPracticeCompatibilityAssessment
}

// AnalyzeSnapshotCompatibility compares two verified FPF indexes without
// mutating either one. It reports only facts encoded in source/index bytes and
// the explicit Local-Practice candidate basis.
func AnalyzeSnapshotCompatibility(input SnapshotAnalysisInput) (SnapshotAnalysis, error) {
	predecessor, err := loadAnalysisIndex(input.PredecessorDatabasePath)
	if err != nil {
		return SnapshotAnalysis{}, fmt.Errorf("load predecessor FPF index: %w", err)
	}
	candidate, err := loadAnalysisIndex(input.CandidateDatabasePath)
	if err != nil {
		return SnapshotAnalysis{}, fmt.Errorf("load candidate FPF index: %w", err)
	}

	deltas := make([]Delta, 0)
	appendDelta := func(delta Delta, deltaErr error) error {
		if deltaErr != nil {
			return deltaErr
		}
		deltas = append(deltas, delta)
		return nil
	}

	if predecessor.meta["fpf_commit"] != candidate.meta["fpf_commit"] {
		if err := appendDelta(NewDelta(
			DeltaSourceIdentity,
			DeltaSourceIdentityChanged,
			"FPF source revision",
			predecessor.meta["fpf_commit"],
			candidate.meta["fpf_commit"],
			"",
		)); err != nil {
			return SnapshotAnalysis{}, err
		}
	}
	for _, document := range []struct {
		subject string
		key     string
	}{
		{subject: "Readme.md", key: "readme_document_digest"},
		{subject: "FPF-Spec.md", key: "spec_document_digest"},
	} {
		if predecessor.meta[document.key] == candidate.meta[document.key] {
			continue
		}
		if err := appendDelta(NewDelta(
			DeltaSourceContent,
			DeltaContentOnlyCompatible,
			document.subject,
			predecessor.meta[document.key],
			candidate.meta[document.key],
			"",
		)); err != nil {
			return SnapshotAnalysis{}, err
		}
	}
	if predecessor.meta["indexed_source_units"] != candidate.meta["indexed_source_units"] {
		if err := appendDelta(NewDelta(
			DeltaSourceContent,
			DeltaContentOnlyCompatible,
			"source-unit count",
			predecessor.meta["indexed_source_units"],
			candidate.meta["indexed_source_units"],
			"",
		)); err != nil {
			return SnapshotAnalysis{}, err
		}
	}

	practicalDeltas, queryDeltas, err := comparePracticalUseCards(predecessor, candidate)
	if err != nil {
		return SnapshotAnalysis{}, err
	}
	deltas = append(deltas, practicalDeltas...)
	deltas = append(deltas, queryDeltas...)
	grammarDelta, exists, err := comparePracticalUseGrammar(predecessor, candidate)
	if err != nil {
		return SnapshotAnalysis{}, err
	}
	if exists {
		deltas = append(deltas, grammarDelta)
	}

	patternDeltas, patternQueryDeltas, err := comparePatternIDs(predecessor, candidate)
	if err != nil {
		return SnapshotAnalysis{}, err
	}
	deltas = append(deltas, patternDeltas...)
	deltas = append(deltas, patternQueryDeltas...)

	structuralDeltas, err := compareSourceUnitStructure(predecessor, candidate)
	if err != nil {
		return SnapshotAnalysis{}, err
	}
	deltas = append(deltas, structuralDeltas...)

	diagnostics := make([]Diagnostic, 0)
	if predecessor.meta["typeenv_artifact_digest"] !=
		candidate.meta["typeenv_artifact_digest"] {
		typeEnvDeltas, typeEnvErr := candidateTypeEnvDeltas(input.CandidateDatabasePath)
		if typeEnvErr != nil {
			return SnapshotAnalysis{}, typeEnvErr
		}
		deltas = append(deltas, typeEnvDeltas...)
		semanticDeltas, semanticDiagnostics, semanticErr :=
			compareExecutableTypeEnvCompatibility(
				input.PredecessorDatabasePath,
				input.CandidateDatabasePath,
			)
		if semanticErr != nil {
			return SnapshotAnalysis{}, semanticErr
		}
		deltas = append(deltas, semanticDeltas...)
		diagnostics = append(diagnostics, semanticDiagnostics...)
	}

	var localPracticeCompatibility *LocalPracticeCompatibilityAssessment
	if input.LatestLocalPracticeCandidate != "" {
		assessment, localDeltas, localDiagnostics, localErr := compareLocalPracticeCandidate(
			input.LatestLocalPracticeCandidate,
			input.PredecessorDatabasePath,
			input.CandidateDatabasePath,
			candidate.meta["fpf_commit"],
			candidate.meta["spec_document_digest"],
		)
		if localErr != nil {
			return SnapshotAnalysis{}, localErr
		}
		localPracticeCompatibility = &assessment
		deltas = append(deltas, localDeltas...)
		diagnostics = append(diagnostics, localDiagnostics...)
	}

	return SnapshotAnalysis{
		Deltas:                     deltas,
		Diagnostics:                diagnostics,
		LocalPracticeCompatibility: localPracticeCompatibility,
	}, nil
}

// ReportSnapshotsFromIntegrationCoordinates converts verified generated-lock
// coordinates into the strong report model.
func ReportSnapshotsFromIntegrationCoordinates(
	predecessor IntegrationCoordinates,
	candidate IntegrationCoordinates,
) (PredecessorSnapshot, CandidateSnapshot, error) {
	predecessorSource, predecessorDerived, err := reportCoordinates(predecessor)
	if err != nil {
		return PredecessorSnapshot{}, CandidateSnapshot{}, fmt.Errorf(
			"predecessor report coordinates: %w",
			err,
		)
	}
	candidateSource, candidateDerived, err := reportCoordinates(candidate)
	if err != nil {
		return PredecessorSnapshot{}, CandidateSnapshot{}, fmt.Errorf(
			"candidate report coordinates: %w",
			err,
		)
	}
	predecessorSnapshot, err := NewPredecessorSnapshot(
		predecessorSource,
		predecessorDerived,
	)
	if err != nil {
		return PredecessorSnapshot{}, CandidateSnapshot{}, err
	}
	candidateSnapshot, err := NewCandidateSnapshot(candidateSource, candidateDerived)
	if err != nil {
		return PredecessorSnapshot{}, CandidateSnapshot{}, err
	}
	return predecessorSnapshot, candidateSnapshot, nil
}

type analysisIndex struct {
	meta  map[string]string
	units map[string]analysisSourceUnit
}

type analysisSourceUnit struct {
	UnitID             string
	SourceID           string
	Role               string
	Title              string
	Body               string
	PatternID          string
	DirectRefsJSON     string
	RelationsDigest    string
	RelationProjection string
	UseCuesJSON        string
	SourcePath         string
	StartLine          int
	EndLine            int
	ContentHash        string
	SourceRevision     string
}

func loadAnalysisIndex(path string) (analysisIndex, error) {
	database, err := openIntegrationDatabaseReadOnly(path)
	if err != nil {
		return analysisIndex{}, err
	}
	defer func() { _ = database.Close() }()

	meta, err := readRequiredIntegrationMeta(database)
	if err != nil {
		return analysisIndex{}, err
	}
	rows, err := database.Query(`
		SELECT
			unit_id,
			source_id,
			source_role,
			title,
			pattern_id,
			direct_refs_json,
			relations_digest,
			use_cues_json,
			source_path,
			start_line,
			end_line,
			content_hash,
			source_revision
		FROM source_units
		ORDER BY unit_id
	`)
	if err != nil {
		return analysisIndex{}, fmt.Errorf("read source units: %w", err)
	}
	defer func() { _ = rows.Close() }()

	units := make(map[string]analysisSourceUnit)
	for rows.Next() {
		var unit analysisSourceUnit
		if err := rows.Scan(
			&unit.UnitID,
			&unit.SourceID,
			&unit.Role,
			&unit.Title,
			&unit.PatternID,
			&unit.DirectRefsJSON,
			&unit.RelationsDigest,
			&unit.UseCuesJSON,
			&unit.SourcePath,
			&unit.StartLine,
			&unit.EndLine,
			&unit.ContentHash,
			&unit.SourceRevision,
		); err != nil {
			return analysisIndex{}, fmt.Errorf("decode source unit: %w", err)
		}
		if _, exists := units[unit.UnitID]; exists {
			return analysisIndex{}, fmt.Errorf("duplicate source unit %q", unit.UnitID)
		}
		unit.RelationProjection = unit.RelationsDigest
		units[unit.UnitID] = unit
	}
	if err := rows.Err(); err != nil {
		return analysisIndex{}, fmt.Errorf("iterate source units: %w", err)
	}
	if err := loadSemanticRelationProjections(database, units); err != nil {
		return analysisIndex{}, err
	}
	if err := loadAnalysisSourceBodies(database, units); err != nil {
		return analysisIndex{}, err
	}
	return analysisIndex{meta: meta, units: units}, nil
}

func loadAnalysisSourceBodies(
	database *sql.DB,
	units map[string]analysisSourceUnit,
) error {
	rows, err := database.Query(`PRAGMA table_info(source_units)`)
	if err != nil {
		return fmt.Errorf("inspect source unit columns: %w", err)
	}
	hasBody := false
	for rows.Next() {
		var ordinal int
		var name, dataType string
		var notNull, primaryKey int
		var defaultValue any
		if err := rows.Scan(
			&ordinal,
			&name,
			&dataType,
			&notNull,
			&defaultValue,
			&primaryKey,
		); err != nil {
			_ = rows.Close()
			return fmt.Errorf("decode source unit column: %w", err)
		}
		if name == "body" {
			hasBody = true
		}
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if !hasBody {
		return nil
	}
	bodyRows, err := database.Query(`SELECT unit_id, body FROM source_units ORDER BY unit_id`)
	if err != nil {
		return fmt.Errorf("read source unit bodies: %w", err)
	}
	defer func() { _ = bodyRows.Close() }()
	for bodyRows.Next() {
		var unitID, body string
		if err := bodyRows.Scan(&unitID, &body); err != nil {
			return fmt.Errorf("decode source unit body: %w", err)
		}
		unit, exists := units[unitID]
		if !exists {
			return fmt.Errorf("source unit body references unknown unit %q", unitID)
		}
		unit.Body = body
		units[unitID] = unit
	}
	return bodyRows.Err()
}

func loadSemanticRelationProjections(
	database *sql.DB,
	units map[string]analysisSourceUnit,
) error {
	var relationTableCount int
	if err := database.QueryRow(`
		SELECT COUNT(*)
		FROM sqlite_master
		WHERE type = 'table' AND name = 'source_unit_relations'
	`).Scan(&relationTableCount); err != nil {
		return fmt.Errorf("inspect source relation table: %w", err)
	}
	if relationTableCount == 0 {
		// Small historical/test fixtures may predate normalized relation rows.
		// Their stored digest remains the only available comparison basis.
		return nil
	}
	rows, err := database.Query(`
		SELECT unit_id, relation_kind, target_pattern_id, target_class, origin
		FROM source_unit_relations
		ORDER BY unit_id, relation_kind, target_pattern_id, target_class, origin
	`)
	if err != nil {
		return fmt.Errorf("read semantic source relations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	projected := make(map[string][]string)
	for rows.Next() {
		var unitID, kind, target, targetClass, origin string
		if err := rows.Scan(&unitID, &kind, &target, &targetClass, &origin); err != nil {
			return fmt.Errorf("decode semantic source relation: %w", err)
		}
		projected[unitID] = append(
			projected[unitID],
			strings.Join([]string{kind, target, targetClass, origin}, "\x00"),
		)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate semantic source relations: %w", err)
	}
	for unitID, unit := range units {
		encoded, err := json.Marshal(projected[unitID])
		if err != nil {
			return fmt.Errorf("encode semantic source relations for %s: %w", unitID, err)
		}
		unit.RelationProjection = digestBytesSHA256(encoded)
		units[unitID] = unit
	}
	return nil
}

func comparePracticalUseCards(
	predecessor analysisIndex,
	candidate analysisIndex,
) ([]Delta, []Delta, error) {
	before := sourceUnitsByIdentity(predecessor.units, string(fpf.SourceUnitRolePracticalUseCard))
	after := sourceUnitsByIdentity(candidate.units, string(fpf.SourceUnitRolePracticalUseCard))
	identities := unionSortedKeys(before, after)
	deltas := make([]Delta, 0)
	queryDeltas := make([]Delta, 0)
	for _, identity := range identities {
		oldUnit, hadOld := before[identity]
		newUnit, hasNew := after[identity]
		switch {
		case !hadOld:
			delta, err := NewDelta(
				DeltaPracticalUseCards,
				DeltaPracticalUseCardAdded,
				identity,
				"",
				practicalCardProjection(newUnit),
				sourceUnitRef(newUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			deltas = append(deltas, delta)
			queryDelta, err := NewDelta(
				DeltaQueryBehavior,
				DeltaQueryExpectationChanged,
				"exact practical-use identifier "+identity,
				fpf.ReasonExactSourceUnitNotFound,
				newUnit.UnitID,
				sourceUnitRef(newUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			queryDeltas = append(queryDeltas, queryDelta)
		case !hasNew:
			delta, err := NewDelta(
				DeltaPracticalUseCards,
				DeltaPracticalUseCardRemoved,
				identity,
				practicalCardProjection(oldUnit),
				"",
				sourceUnitRef(oldUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			deltas = append(deltas, delta)
			queryDelta, err := NewDelta(
				DeltaQueryBehavior,
				DeltaQueryExpectationChanged,
				"exact practical-use identifier "+identity,
				oldUnit.UnitID,
				fpf.ReasonExactSourceUnitNotFound,
				sourceUnitRef(oldUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			queryDeltas = append(queryDeltas, queryDelta)
		default:
			oldProjection := practicalCardProjection(oldUnit)
			newProjection := practicalCardProjection(newUnit)
			if oldProjection != newProjection {
				delta, err := NewDelta(
					DeltaPracticalUseCards,
					DeltaPracticalUseCardChanged,
					identity,
					oldProjection,
					newProjection,
					sourceUnitRef(newUnit),
				)
				if err != nil {
					return nil, nil, err
				}
				deltas = append(deltas, delta)
			}
			if oldUnit.DirectRefsJSON != newUnit.DirectRefsJSON {
				delta, err := NewDelta(
					DeltaPracticalCardDirectRefs,
					DeltaPracticalCardDirectRefsChanged,
					identity,
					oldUnit.DirectRefsJSON,
					newUnit.DirectRefsJSON,
					sourceUnitRef(newUnit),
				)
				if err != nil {
					return nil, nil, err
				}
				deltas = append(deltas, delta)
			}
		}
	}
	continuityDeltas, err := compareSourceAuthoredPracticalUseContinuity(
		candidate,
		before,
		after,
	)
	if err != nil {
		return nil, nil, err
	}
	deltas = append(deltas, continuityDeltas...)
	return deltas, queryDeltas, nil
}

const e11HistoricalReadPathPrefix = "E.11 records one F.13-form historical read path: "

var e11SourceAuthoredSplitPattern = regexp.MustCompile(
	"^" + regexp.QuoteMeta(e11HistoricalReadPathPrefix) +
		"`splits\\(([A-Z][A-Z0-9-]*) -> \\{([A-Z][A-Z0-9-]*(?:, [A-Z][A-Z0-9-]*)*)\\}\\)`\\.(?: |$)",
)

type sourceAuthoredPracticalUseSplit struct {
	predecessor string
	successors  []string
	sourceRef   string
}

func compareSourceAuthoredPracticalUseContinuity(
	candidate analysisIndex,
	beforeCards map[string]analysisSourceUnit,
	afterCards map[string]analysisSourceUnit,
) ([]Delta, error) {
	splits, err := sourceAuthoredPracticalUseSplits(candidate)
	if err != nil {
		return nil, err
	}
	deltas := make([]Delta, 0)
	for _, split := range splits {
		_, predecessorHadCard := beforeCards[split.predecessor]
		_, candidateStillHasCard := afterCards[split.predecessor]
		if !predecessorHadCard || candidateStillHasCard {
			// The authored historical relation is not a delta for this exact
			// predecessor/candidate pair.
			continue
		}
		for _, successor := range split.successors {
			if _, exists := afterCards[successor]; !exists {
				return nil, fmt.Errorf(
					"source-authored E.11 split %s names absent candidate practical-use card %s",
					split.predecessor,
					successor,
				)
			}
		}
		delta, err := NewDelta(
			DeltaPracticalUseCards,
			DeltaPracticalUseCardSplit,
			"source-authored split "+split.predecessor,
			split.predecessor,
			strings.Join(split.successors, ","),
			split.sourceRef,
		)
		if err != nil {
			return nil, err
		}
		deltas = append(deltas, delta)
	}
	return deltas, nil
}

func sourceAuthoredPracticalUseSplits(
	index analysisIndex,
) ([]sourceAuthoredPracticalUseSplit, error) {
	unitIDs := make([]string, 0, len(index.units))
	for unitID := range index.units {
		unitIDs = append(unitIDs, unitID)
	}
	sort.Strings(unitIDs)
	bySource := make(map[string]sourceAuthoredPracticalUseSplit)
	for _, unitID := range unitIDs {
		unit := index.units[unitID]
		if unit.PatternID != "E.11" ||
			unit.Role != string(fpf.SourceUnitRolePatternBody) ||
			unit.SourcePath != "data/FPF/FPF-Spec.md" ||
			unit.Body == "" {
			continue
		}
		for offset, line := range strings.Split(unit.Body, "\n") {
			if !strings.HasPrefix(line, e11HistoricalReadPathPrefix) {
				continue
			}
			match := e11SourceAuthoredSplitPattern.FindStringSubmatch(line)
			if len(match) != 3 {
				return nil, fmt.Errorf(
					"source-authored E.11 historical read path has unsupported splits(...) grammar in %s",
					unit.UnitID,
				)
			}
			lineNumber := unit.StartLine + offset
			if unit.StartLine <= 0 || lineNumber > unit.EndLine {
				return nil, fmt.Errorf(
					"source-authored E.11 split line is outside source span for %s",
					unit.UnitID,
				)
			}
			successors := strings.Split(match[2], ", ")
			seenSuccessors := make(map[string]struct{}, len(successors))
			for _, successor := range successors {
				if successor == match[1] {
					return nil, fmt.Errorf(
						"source-authored E.11 split repeats predecessor %s as a successor",
						match[1],
					)
				}
				if _, duplicate := seenSuccessors[successor]; duplicate {
					return nil, fmt.Errorf(
						"source-authored E.11 split repeats successor %s",
						successor,
					)
				}
				seenSuccessors[successor] = struct{}{}
			}
			sourceRef := fmt.Sprintf(
				"%s:%d-%d",
				unit.SourcePath,
				lineNumber,
				lineNumber,
			)
			split := sourceAuthoredPracticalUseSplit{
				predecessor: match[1],
				successors:  successors,
				sourceRef:   sourceRef,
			}
			key := sourceRef + "\x00" + match[1]
			if previous, exists := bySource[key]; exists {
				if !equalStrings(previous.successors, split.successors) {
					return nil, fmt.Errorf(
						"source-authored E.11 split has contradictory duplicate at %s",
						sourceRef,
					)
				}
				continue
			}
			bySource[key] = split
		}
	}
	keys := make([]string, 0, len(bySource))
	for key := range bySource {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	result := make([]sourceAuthoredPracticalUseSplit, 0, len(keys))
	for _, key := range keys {
		result = append(result, bySource[key])
	}
	return result, nil
}

func equalStrings(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func comparePatternIDs(
	predecessor analysisIndex,
	candidate analysisIndex,
) ([]Delta, []Delta, error) {
	before := patternBodyUnits(predecessor.units)
	after := patternBodyUnits(candidate.units)
	patternIDs := unionSortedKeys(before, after)
	deltas := make([]Delta, 0)
	queryDeltas := make([]Delta, 0)
	for _, patternID := range patternIDs {
		oldUnit, hadOld := before[patternID]
		newUnit, hasNew := after[patternID]
		switch {
		case !hadOld:
			delta, err := NewDelta(
				DeltaPatternIDs,
				DeltaPatternIDAdded,
				patternID,
				"",
				newUnit.UnitID,
				sourceUnitRef(newUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			deltas = append(deltas, delta)
			queryDelta, err := NewDelta(
				DeltaQueryBehavior,
				DeltaQueryExpectationChanged,
				"exact PatternID "+patternID,
				fpf.ReasonExactSourceUnitNotFound,
				newUnit.UnitID,
				sourceUnitRef(newUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			queryDeltas = append(queryDeltas, queryDelta)
		case !hasNew:
			delta, err := NewDelta(
				DeltaPatternIDs,
				DeltaPatternIDRemoved,
				patternID,
				oldUnit.UnitID,
				"",
				sourceUnitRef(oldUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			deltas = append(deltas, delta)
			queryDelta, err := NewDelta(
				DeltaQueryBehavior,
				DeltaQueryExpectationChanged,
				"exact PatternID "+patternID,
				oldUnit.UnitID,
				fpf.ReasonExactSourceUnitNotFound,
				sourceUnitRef(oldUnit),
			)
			if err != nil {
				return nil, nil, err
			}
			queryDeltas = append(queryDeltas, queryDelta)
		}
	}
	return deltas, queryDeltas, nil
}

func comparePracticalUseGrammar(
	predecessor analysisIndex,
	candidate analysisIndex,
) (Delta, bool, error) {
	before, err := practicalUseGrammarFamilies(predecessor)
	if err != nil {
		return Delta{}, false, err
	}
	after, err := practicalUseGrammarFamilies(candidate)
	if err != nil {
		return Delta{}, false, err
	}
	if len(before) == 0 || len(after) == 0 {
		// Historical/minimal DB fixtures without source bodies cannot prove a
		// grammar-family change.
		return Delta{}, false, nil
	}
	added := make([]string, 0)
	for family := range after {
		if _, existed := before[family]; !existed {
			added = append(added, family)
		}
	}
	if len(added) == 0 {
		return Delta{}, false, nil
	}
	sort.Strings(added)
	beforeValues := sortedSetValues(before)
	afterValues := sortedSetValues(after)
	beforeJSON, _ := json.Marshal(beforeValues)
	afterJSON, _ := json.Marshal(afterValues)
	delta, err := NewDelta(
		DeltaPublicationGrammar,
		DeltaPublicationGrammarExtended,
		"practical-use labeled-block grammar; added="+strings.Join(added, ","),
		string(beforeJSON),
		string(afterJSON),
		"data/FPF/FPF-Spec.md@"+candidate.meta["fpf_commit"],
	)
	if err != nil {
		return Delta{}, false, err
	}
	return delta, true, nil
}

func practicalUseGrammarFamilies(
	index analysisIndex,
) (map[string]struct{}, error) {
	families := make(map[string]struct{})
	for _, unit := range index.units {
		if unit.Role != string(fpf.SourceUnitRolePracticalUseCard) || unit.Body == "" {
			continue
		}
		projection, _, err := fpf.ProjectPracticalUseCardSource(fpf.PracticalUseCardSource{
			SourceID:       unit.SourceID,
			Title:          unit.Title,
			Body:           unit.Body,
			SourcePath:     unit.SourcePath,
			SourceRevision: unit.SourceRevision,
			StartLine:      unit.StartLine,
			EndLine:        unit.EndLine,
		})
		if err != nil {
			return nil, fmt.Errorf(
				"recover practical-use grammar families for %s: %w",
				unit.SourceID,
				err,
			)
		}
		for _, block := range projection.Blocks {
			family := practicalBlockLabelFamily(block.Label)
			families[string(block.Kind)+":"+family] = struct{}{}
		}
	}
	return families, nil
}

func practicalBlockLabelFamily(label string) string {
	normalized := strings.ToLower(strings.Join(strings.Fields(label), " "))
	for _, family := range []string{"template", "direct", "leading", "branch"} {
		if normalized == family ||
			strings.HasPrefix(normalized, family+" ") ||
			strings.HasPrefix(normalized, family+":") ||
			strings.HasPrefix(normalized, family+"-") {
			return family
		}
	}
	return normalized
}

func sortedSetValues(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func compareSourceUnitStructure(
	predecessor analysisIndex,
	candidate analysisIndex,
) ([]Delta, error) {
	unitIDs := unionSortedKeys(predecessor.units, candidate.units)
	deltas := make([]Delta, 0)
	for _, unitID := range unitIDs {
		oldUnit, hadOld := predecessor.units[unitID]
		newUnit, hasNew := candidate.units[unitID]
		if !hadOld || !hasNew {
			continue
		}
		if oldUnit.Role != newUnit.Role {
			delta, err := NewDelta(
				DeltaSourceRoles,
				DeltaSourceRoleChanged,
				unitID,
				oldUnit.Role,
				newUnit.Role,
				sourceUnitRef(newUnit),
			)
			if err != nil {
				return nil, err
			}
			deltas = append(deltas, delta)
		}
		if oldUnit.RelationProjection != newUnit.RelationProjection {
			delta, err := NewDelta(
				DeltaToCRelations,
				DeltaToCRelationChanged,
				unitID,
				oldUnit.RelationProjection,
				newUnit.RelationProjection,
				sourceUnitRef(newUnit),
			)
			if err != nil {
				return nil, err
			}
			deltas = append(deltas, delta)
		}
	}
	return deltas, nil
}

func candidateTypeEnvDeltas(path string) ([]Delta, error) {
	database, err := openIntegrationDatabaseReadOnly(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = database.Close() }()
	rows, err := database.Query(`
		SELECT symbol, change_kind, rationale
		FROM fpf_typeenv_compatibility_changes
		ORDER BY change_ordinal
	`)
	if err != nil {
		return nil, fmt.Errorf("read candidate Base TypeEnv compatibility: %w", err)
	}
	defer func() { _ = rows.Close() }()
	deltas := make([]Delta, 0)
	for rows.Next() {
		var symbol, changeKind, rationale string
		if err := rows.Scan(&symbol, &changeKind, &rationale); err != nil {
			return nil, fmt.Errorf("decode Base TypeEnv compatibility change: %w", err)
		}
		var kind DeltaKind
		switch changeKind {
		case "added":
			kind = DeltaTypeEnvAdditive
		case "changed":
			kind = DeltaTypeEnvChanged
		case "removed":
			kind = DeltaTypeEnvRemoved
		default:
			return nil, fmt.Errorf(
				"unknown Base TypeEnv compatibility change kind %q for %s",
				changeKind,
				symbol,
			)
		}
		delta, err := NewDelta(
			DeltaBaseTypeEnv,
			kind,
			symbol,
			changeKind,
			rationale,
			"",
		)
		if err != nil {
			return nil, err
		}
		deltas = append(deltas, delta)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate Base TypeEnv compatibility changes: %w", err)
	}
	return deltas, nil
}

type typeEnvSuccessorRule interface {
	Family() projecttypeenvcompatibility.Family
	Key() string
	Class() projecttypeenvcompatibility.SuccessorRuleClass
	Ground() projecttypeenvcompatibility.SuccessorRuleGround
	BeforeDigest() (typedmemory.SHA256Digest, bool)
	AfterDigest() (typedmemory.SHA256Digest, bool)
}

func compareExecutableTypeEnvCompatibility(
	predecessorPath string,
	candidatePath string,
) ([]Delta, []Diagnostic, error) {
	predecessor, err := loadExecutableBaseTypeEnv(predecessorPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load predecessor executable Base TypeEnv: %w", err)
	}
	candidate, err := loadExecutableBaseTypeEnv(candidatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("load candidate executable Base TypeEnv: %w", err)
	}
	diff, err := projecttypeenvcompatibility.CompareSuccessor(predecessor, candidate)
	if err != nil {
		return nil, nil, fmt.Errorf("compare executable Base TypeEnv successor: %w", err)
	}
	return typeEnvSuccessorDeltas(diff.Rules())
}

func loadExecutableBaseTypeEnv(path string) (typedmemory.TypeEnv, error) {
	database, err := openIntegrationDatabaseReadOnly(path)
	if err != nil {
		return typedmemory.TypeEnv{}, err
	}
	defer func() { _ = database.Close() }()
	artifact, err := typeenvsql.LoadArtifactReadOnlyDB(context.Background(), database)
	if err != nil {
		return typedmemory.TypeEnv{}, err
	}
	environment, err := typeenv.LowerBaseTypeEnvArtifact(artifact)
	if err != nil {
		return typedmemory.TypeEnv{}, err
	}
	return environment, nil
}

func typeEnvSuccessorDeltas[T typeEnvSuccessorRule](
	rules []T,
) ([]Delta, []Diagnostic, error) {
	ordered := append([]T(nil), rules...)
	sort.Slice(ordered, func(left, right int) bool {
		return typeEnvSuccessorRuleKey(ordered[left]) <
			typeEnvSuccessorRuleKey(ordered[right])
	})
	deltas := make([]Delta, 0)
	diagnostics := make([]Diagnostic, 0)
	previousKey := ""
	for _, rule := range ordered {
		key := typeEnvSuccessorRuleKey(rule)
		if key == previousKey {
			return nil, nil, fmt.Errorf("duplicate executable Base TypeEnv successor rule %q", key)
		}
		previousKey = key
		if rule.Class() == projecttypeenvcompatibility.SuccessorUnchanged {
			continue
		}
		kind, err := typeEnvSuccessorDeltaKind(rule.Class())
		if err != nil {
			return nil, nil, fmt.Errorf("classify executable Base TypeEnv rule %s: %w", key, err)
		}
		before := ""
		if digest, exists := rule.BeforeDigest(); exists {
			before = digest.String()
		}
		after := ""
		if digest, exists := rule.AfterDigest(); exists {
			after = digest.String()
		}
		subject := "Base TypeEnv semantic " + key
		delta, err := NewDelta(
			DeltaBaseTypeEnv,
			kind,
			subject,
			before,
			after,
			"",
		)
		if err != nil {
			return nil, nil, err
		}
		deltas = append(deltas, delta)
		if rule.Class() != projecttypeenvcompatibility.SuccessorCompilerGap {
			continue
		}
		diagnostic, err := NewDiagnostic(
			DiagnosticTypeEnvCompilerGap,
			subject,
			"executable Base TypeEnv semantic order is not provable: "+string(rule.Ground()),
			"",
			"",
		)
		if err != nil {
			return nil, nil, err
		}
		diagnostics = append(diagnostics, diagnostic)
	}
	return deltas, diagnostics, nil
}

func typeEnvSuccessorRuleKey(rule typeEnvSuccessorRule) string {
	return rule.Family().String() + "/" + rule.Key()
}

func typeEnvSuccessorDeltaKind(
	class projecttypeenvcompatibility.SuccessorRuleClass,
) (DeltaKind, error) {
	switch class {
	case projecttypeenvcompatibility.SuccessorAdditive:
		return DeltaTypeEnvAdditive, nil
	case projecttypeenvcompatibility.SuccessorWidened:
		// Widening is an exact semantic change but is not narrowing. Keep the
		// existing neutral review classification rather than inventing an order.
		return DeltaTypeEnvChanged, nil
	case projecttypeenvcompatibility.SuccessorNarrowed:
		return DeltaTypeEnvNarrowed, nil
	case projecttypeenvcompatibility.SuccessorRemoved:
		return DeltaTypeEnvRemoved, nil
	case projecttypeenvcompatibility.SuccessorCompilerGap:
		return DeltaTypeEnvCompilerGap, nil
	default:
		return 0, fmt.Errorf("unsupported successor class %q", class.String())
	}
}

type localPracticeFPFSourcePin struct {
	symbol  string
	edition string
	digest  string
	start   uint64
	end     uint64
}

func compareLocalPracticeCandidate(
	path string,
	predecessorDatabasePath string,
	candidateDatabasePath string,
	candidateSourceRevision string,
	candidateSpecDigest string,
) (LocalPracticeCompatibilityAssessment, []Delta, []Diagnostic, error) {
	predecessor, err := loadExecutableBaseTypeEnv(predecessorDatabasePath)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"load predecessor Base TypeEnv for Local-Practice assessment: %w",
			err,
		)
	}
	candidate, err := loadExecutableBaseTypeEnv(candidateDatabasePath)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"load candidate Base TypeEnv for Local-Practice assessment: %w",
			err,
		)
	}
	return compareLocalPracticeCandidateAgainstEnvironments(
		path,
		predecessor,
		candidate,
		candidateSourceRevision,
		candidateSpecDigest,
	)
}

func compareLocalPracticeCandidateAgainstEnvironments(
	path string,
	predecessor typedmemory.TypeEnv,
	candidate typedmemory.TypeEnv,
	candidateSourceRevision string,
	candidateSpecDigest string,
) (LocalPracticeCompatibilityAssessment, []Delta, []Diagnostic, error) {
	diff, err := projecttypeenvcompatibility.CompareSuccessor(predecessor, candidate)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"compare predecessor/candidate Base TypeEnv for Local-Practice assessment: %w",
			err,
		)
	}
	return compareLocalPracticeCandidateAgainstSuccessor(
		path,
		diff.Base(),
		diff.Target(),
		diff.Rules(),
		candidateSourceRevision,
		candidateSpecDigest,
	)
}

func compareLocalPracticeCandidateAgainstSuccessor[T typeEnvSuccessorRule](
	path string,
	predecessorTypeEnvRef typedmemory.TypeEnvRef,
	candidateTypeEnvRef typedmemory.TypeEnvRef,
	rules []T,
	candidateSourceRevision string,
	candidateSpecDigest string,
) (LocalPracticeCompatibilityAssessment, []Delta, []Diagnostic, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"read latest Local-Practice candidate: %w",
			err,
		)
	}
	parsed, err := localpractice.Parse(content)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"decode latest Local-Practice candidate: %w",
			err,
		)
	}
	carrier := parsed.Carrier()
	carrierTypeEnvRef, err := typedmemory.ParseTypeEnvRef(carrier.BaseTypeEnvRef().Value())
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"decode latest Local-Practice candidate Base TypeEnv reference: %w",
			err,
		)
	}
	compatibilityResult, err := classifyLocalPracticeCompatibility(
		carrierTypeEnvRef,
		predecessorTypeEnvRef,
		candidateTypeEnvRef,
		rules,
	)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, err
	}
	assessment, err := NewLocalPracticeCompatibilityAssessment(
		carrierTypeEnvRef,
		predecessorTypeEnvRef,
		candidateTypeEnvRef,
		compatibilityResult,
	)
	if err != nil {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, err
	}
	deltas := make([]Delta, 0)
	diagnostics := make([]Diagnostic, 0)
	if compatibilityResult != LocalPracticeExact {
		delta, err := NewDelta(
			DeltaLocalPracticeBasis,
			DeltaLocalPracticeBasisChanged,
			"latest Local-Practice candidate base_type_env_ref",
			carrierTypeEnvRef.String(),
			candidateTypeEnvRef.String(),
			DefaultLocalPracticeCandidateRelative,
		)
		if err != nil {
			return LocalPracticeCompatibilityAssessment{}, nil, nil, err
		}
		deltas = append(deltas, delta)
	}

	pins := make([]localPracticeFPFSourcePin, 0)
	for _, declaration := range carrier.Signature().Vocabulary().Declarations() {
		signature, ok := declaration.(localpractice.KindClassificationSignatureDeclaration)
		if !ok {
			continue
		}
		pin := signature.ReferenceScheme()
		if pin.CarrierRef().Value() != "fpf-source:FPF-Spec.md" {
			continue
		}
		span := pin.Span()
		pins = append(pins, localPracticeFPFSourcePin{
			symbol:  signature.Symbol().Value(),
			edition: pin.Edition().Value(),
			digest:  pin.Digest().Value(),
			start:   span.Start(),
			end:     span.End(),
		})
	}
	if len(pins) == 0 {
		return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
			"latest Local-Practice candidate has no fpf-source:FPF-Spec.md reference_scheme pins",
		)
	}
	sort.Slice(pins, func(left, right int) bool {
		return pins[left].symbol < pins[right].symbol
	})
	pinCoordinate := pins[0].edition + "@" + pins[0].digest
	for index, pin := range pins {
		if index > 0 && pin.symbol == pins[index-1].symbol {
			return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
				"latest Local-Practice candidate has duplicate FPF-Spec pin subject %q",
				pin.symbol,
			)
		}
		coordinate := pin.edition + "@" + pin.digest
		if coordinate != pinCoordinate {
			return LocalPracticeCompatibilityAssessment{}, nil, nil, fmt.Errorf(
				"latest Local-Practice candidate has contradictory FPF-Spec pins %q and %q",
				pinCoordinate,
				coordinate,
			)
		}
	}
	candidateCoordinate := candidateSourceRevision + "@" + candidateSpecDigest
	if pinCoordinate == candidateCoordinate {
		return assessment, deltas, diagnostics, nil
	}
	for _, pin := range pins {
		subject := "Local-Practice " + pin.symbol + " FPF-Spec reference"
		delta, err := NewDelta(
			DeltaSpecCarrierReferences,
			DeltaSpecSemanticReviewRequired,
			subject,
			pin.edition+"@"+pin.digest,
			candidateCoordinate,
			fmt.Sprintf(
				"%s:%d-%d",
				DefaultLocalPracticeCandidateRelative,
				pin.start,
				pin.end,
			),
		)
		if err != nil {
			return LocalPracticeCompatibilityAssessment{}, nil, nil, err
		}
		deltas = append(deltas, delta)
	}
	return assessment, deltas, diagnostics, nil
}

func classifyLocalPracticeCompatibility[T typeEnvSuccessorRule](
	carrierTypeEnvRef typedmemory.TypeEnvRef,
	predecessorTypeEnvRef typedmemory.TypeEnvRef,
	candidateTypeEnvRef typedmemory.TypeEnvRef,
	rules []T,
) (LocalPracticeCompatibilityResult, error) {
	if carrierTypeEnvRef == candidateTypeEnvRef {
		return LocalPracticeExact, nil
	}
	if carrierTypeEnvRef != predecessorTypeEnvRef {
		// The refresh has no executable artifact for the carrier's exact
		// basis, so it cannot prove a semantic successor order.
		return LocalPracticeCompilerGap, nil
	}
	hasSemanticReview := false
	hasCompilerGap := false
	for _, rule := range rules {
		switch rule.Class() {
		case projecttypeenvcompatibility.SuccessorUnchanged,
			projecttypeenvcompatibility.SuccessorAdditive,
			projecttypeenvcompatibility.SuccessorWidened:
		case projecttypeenvcompatibility.SuccessorNarrowed,
			projecttypeenvcompatibility.SuccessorRemoved:
			hasSemanticReview = true
		case projecttypeenvcompatibility.SuccessorCompilerGap:
			hasCompilerGap = true
		default:
			return 0, fmt.Errorf(
				"unsupported Local-Practice successor rule class %q for %s",
				rule.Class().String(),
				typeEnvSuccessorRuleKey(rule),
			)
		}
	}
	if hasCompilerGap {
		return LocalPracticeCompilerGap, nil
	}
	if hasSemanticReview {
		return LocalPracticeSemanticReviewRequired, nil
	}
	return LocalPracticeCompatibleSuccessorCandidatePossible, nil
}

func reportCoordinates(
	coordinates IntegrationCoordinates,
) (SourceCoordinates, DerivedCoordinates, error) {
	if coordinates.SourceUnitCount <= 0 {
		return SourceCoordinates{}, DerivedCoordinates{}, fmt.Errorf(
			"source-unit count must be positive",
		)
	}
	revision, err := typedmemory.NewSourceRevision(coordinates.SourceRevision)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	readmeDigest, err := typedmemory.NewSHA256Digest(coordinates.ReadmeDocumentDigest)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	specDigest, err := typedmemory.NewSHA256Digest(coordinates.SpecDocumentDigest)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	source, err := NewSourceCoordinates(revision, readmeDigest, specDigest)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	typeEnvRef, err := typedmemory.ParseTypeEnvRef(coordinates.BaseTypeEnvRef)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	typeEnvDigest, err := typedmemory.NewSHA256Digest(coordinates.BaseTypeEnvDigest)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	compiler, err := typedmemory.NewCompilerSchemaVersion(
		coordinates.TypeEnvCompilerEdition,
	)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	databaseDigest, err := typedmemory.NewSHA256Digest(coordinates.DatabaseDigest)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	schemaVersion, err := strconv.ParseUint(coordinates.IndexSchemaVersion, 10, 64)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, fmt.Errorf(
			"parse index schema version: %w",
			err,
		)
	}
	derived, err := NewDerivedCoordinates(
		uint64(coordinates.SourceUnitCount),
		typeEnvRef,
		typeEnvDigest,
		compiler,
		databaseDigest,
		schemaVersion,
	)
	if err != nil {
		return SourceCoordinates{}, DerivedCoordinates{}, err
	}
	return source, derived, nil
}

func sourceUnitsByIdentity(
	units map[string]analysisSourceUnit,
	role string,
) map[string]analysisSourceUnit {
	selected := make(map[string]analysisSourceUnit)
	for _, unit := range units {
		if unit.Role != role {
			continue
		}
		identity := unit.SourceID
		if identity == "" {
			identity = unit.UnitID
		}
		selected[identity] = unit
	}
	return selected
}

func patternBodyUnits(
	units map[string]analysisSourceUnit,
) map[string]analysisSourceUnit {
	selected := make(map[string]analysisSourceUnit)
	for _, unit := range units {
		if unit.Role != string(fpf.SourceUnitRolePatternBody) || unit.PatternID == "" {
			continue
		}
		selected[unit.PatternID] = unit
	}
	return selected
}

func unionSortedKeys[T any](sets ...map[string]T) []string {
	seen := make(map[string]struct{})
	for _, set := range sets {
		for key := range set {
			seen[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func practicalCardProjection(unit analysisSourceUnit) string {
	var cues fpf.SourceUseCues
	if err := json.Unmarshal([]byte(unit.UseCuesJSON), &cues); err != nil {
		cues = fpf.SourceUseCues{
			ConditionText: "invalid-use-cues-json:" +
				digestBytesSHA256([]byte(unit.UseCuesJSON)),
		}
	}
	var directRefs []string
	if err := json.Unmarshal([]byte(unit.DirectRefsJSON), &directRefs); err != nil {
		directRefs = []string{
			"invalid-direct-refs-json:" +
				digestBytesSHA256([]byte(unit.DirectRefsJSON)),
		}
	}
	value := struct {
		Title      string                `json:"title"`
		Condition  boundedTextProjection `json:"condition"`
		Result     boundedTextProjection `json:"result"`
		Boundary   boundedTextProjection `json:"boundary"`
		DirectRefs []string              `json:"direct_refs"`
	}{
		Title:      boundedOneLine(unit.Title, 300),
		Condition:  projectBoundedText(cues.ConditionText),
		Result:     projectBoundedText(cues.FirstResultText),
		Boundary:   projectBoundedText(cues.StopReturnText),
		DirectRefs: directRefs,
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return "projection-error:" + digestBytesSHA256(
			[]byte(unit.Title+"\x00"+unit.UseCuesJSON+"\x00"+unit.DirectRefsJSON),
		)
	}
	return string(encoded)
}

type boundedTextProjection struct {
	Digest    string `json:"digest"`
	RuneCount int    `json:"rune_count"`
	Excerpt   string `json:"excerpt,omitempty"`
}

func projectBoundedText(value string) boundedTextProjection {
	return boundedTextProjection{
		Digest:    digestBytesSHA256([]byte(value)),
		RuneCount: len([]rune(value)),
		Excerpt:   boundedOneLine(value, 240),
	}
}

func boundedOneLine(value string, maximumRunes int) string {
	normalized := strings.Join(strings.Fields(value), " ")
	runes := []rune(normalized)
	if len(runes) <= maximumRunes {
		return normalized
	}
	return string(runes[:maximumRunes]) + "..."
}

func sourceUnitRef(unit analysisSourceUnit) string {
	return fmt.Sprintf(
		"%s:%d-%d@%s",
		unit.SourcePath,
		unit.StartLine,
		unit.EndLine,
		unit.SourceRevision,
	)
}

// Ensure the read-only database import remains part of this adapter's static
// contract even when build-tagged tests replace the SQLite driver.
var _ *sql.DB
