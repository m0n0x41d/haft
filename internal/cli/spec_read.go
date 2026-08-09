package cli

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/m0n0x41d/haft/internal/project"
	"github.com/m0n0x41d/haft/internal/project/specflow"
)

func loadProjectSpecificationSetSQLFirstFromCanonicalProfile(
	ctx context.Context,
	projectRoot string,
	request projectSpecificationScopeRequest,
) (
	project.ProjectSpecificationSet,
	projectSpecificationApplicabilityResolution,
	error,
) {
	resolution, err := resolveCanonicalProjectSpecificationApplicability(
		ctx,
		projectRoot,
		request,
	)
	if err != nil {
		return project.ProjectSpecificationSet{},
			projectSpecificationApplicabilityResolution{},
			err
	}
	applicability, _, resolved := resolution.Resolved()
	if !resolved {
		return project.ProjectSpecificationSet{}, resolution, nil
	}
	canonicalRoot := resolution.ProjectRoot().String()
	specSet, err := loadProjectSpecificationSetSQLFirstForScope(
		canonicalRoot,
		applicability,
	)
	return specSet, resolution, err
}

func loadProjectSpecificationSetSQLFirst(projectRoot string) (project.ProjectSpecificationSet, error) {
	sqlSpecSet, hasSQLSource, err := loadProjectSpecificationSetFromSQLEditions(projectRoot)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	if hasSQLSource {
		carrierSpecSet, carrierErr := project.LoadProjectSpecificationSet(projectRoot)
		if carrierErr != nil {
			return sqlSpecSet, nil
		}

		return mergeSQLSpecSetWithCarrierSupport(sqlSpecSet, carrierSpecSet), nil
	}

	return project.LoadProjectSpecificationSet(projectRoot)
}

const specEditionCarrierDeltaJudgment = "not_truth_or_lifecycle_judgement"

type specEditionCarrierDelta struct {
	Comparison          string                           `json:"comparison"`
	Posture             string                           `json:"posture"`
	Judgment            string                           `json:"judgment"`
	CanonicalSource     string                           `json:"canonical_source"`
	CarrierSource       string                           `json:"carrier_source"`
	SQLSectionCount     int                              `json:"sql_section_count"`
	CarrierSectionCount int                              `json:"carrier_section_count"`
	ClaimCountBasis     string                           `json:"claim_count_basis"`
	SQLClaimCount       int                              `json:"sql_claim_count"`
	SQLStoredClaimCount int                              `json:"sql_stored_claim_count"`
	CarrierClaimCount   int                              `json:"carrier_claim_count"`
	CarrierObservation  specCarrierObservation           `json:"carrier_observation"`
	Sections            []specEditionCarrierSectionDelta `json:"sections"`
}

type specCarrierObservation struct {
	Posture      string   `json:"posture"`
	FindingCount int      `json:"finding_count"`
	FindingCodes []string `json:"finding_codes"`
	Detail       string   `json:"detail,omitempty"`
}

type specEditionCarrierSectionDelta struct {
	SectionID              string   `json:"section_id"`
	SQLSemanticHash        string   `json:"sql_semantic_hash,omitempty"`
	CarrierSemanticHash    string   `json:"carrier_semantic_hash,omitempty"`
	CarrierAddedClaimIDs   []string `json:"carrier_added_claim_ids"`
	CarrierRemovedClaimIDs []string `json:"carrier_removed_claim_ids"`
	CarrierChangedClaimIDs []string `json:"carrier_changed_claim_ids"`
	NonClaimFieldsChanged  bool     `json:"non_claim_fields_changed,omitempty"`
	ClaimOrderChanged      bool     `json:"claim_order_changed,omitempty"`
}

func loadProjectSpecificationSetSQLFirstWithCarrierDelta(
	projectRoot string,
) (project.ProjectSpecificationSet, specEditionCarrierDelta, error) {
	sqlSpecSet, hasSQLSource, err := loadProjectSpecificationSetFromSQLEditions(projectRoot)
	if err != nil {
		return project.ProjectSpecificationSet{}, specEditionCarrierDelta{}, err
	}
	if hasSQLSource {
		carrierSpecSet, carrierErr := project.LoadProjectSpecificationSet(projectRoot)
		if carrierErr != nil {
			return sqlSpecSet, unreadableSpecEditionCarrierDelta(
				sqlSpecSet,
				carrierErr,
			), nil
		}

		return mergeSQLSpecSetWithCarrierSupport(sqlSpecSet, carrierSpecSet),
			compareSpecEditionsWithCarriers(sqlSpecSet, carrierSpecSet),
			nil
	}

	carrierSpecSet, carrierErr := project.LoadProjectSpecificationSet(projectRoot)
	if carrierErr != nil {
		return project.ProjectSpecificationSet{}, specEditionCarrierDelta{}, carrierErr
	}
	return carrierSpecSet, specEditionCarrierDeltaWithoutSQL(carrierSpecSet), nil
}

func newSpecEditionCarrierDelta() specEditionCarrierDelta {
	return specEditionCarrierDelta{
		Comparison:      "sql_spec_section_editions_to_markdown_carriers",
		Judgment:        specEditionCarrierDeltaJudgment,
		CanonicalSource: "sql_spec_section_editions",
		CarrierSource:   "markdown_spec_carriers",
		ClaimCountBasis: "active_sql_review_claims_to_carrier_validation_claims",
		CarrierObservation: specCarrierObservation{
			Posture:      "parsed",
			FindingCodes: []string{},
		},
		Sections: []specEditionCarrierSectionDelta{},
	}
}

func specEditionCarrierDeltaWithoutSQL(
	carrier project.ProjectSpecificationSet,
) specEditionCarrierDelta {
	delta := newSpecEditionCarrierDelta()
	delta.Posture = "not_applicable_no_sql_editions"
	delta.CarrierSectionCount = len(carrier.Sections)
	delta.CarrierClaimCount = specClaimCount(carrier.Sections)
	delta.CarrierObservation = observeSpecCarrier(carrier)
	return delta
}

func unreadableSpecEditionCarrierDelta(
	sqlSpecSet project.ProjectSpecificationSet,
	carrierErr error,
) specEditionCarrierDelta {
	delta := newSpecEditionCarrierDelta()
	delta.Posture = "carrier_unreadable"
	delta.SQLSectionCount = len(sqlSpecSet.Sections)
	delta.SQLClaimCount = activeSpecClaimCount(sqlSpecSet.Sections)
	delta.SQLStoredClaimCount = specClaimCount(sqlSpecSet.Sections)
	delta.CarrierObservation = specCarrierObservation{
		Posture:      "unreadable",
		FindingCodes: []string{},
		Detail:       carrierErr.Error(),
	}
	return delta
}

func compareSpecEditionsWithCarriers(
	sqlSpecSet project.ProjectSpecificationSet,
	carrierSpecSet project.ProjectSpecificationSet,
) specEditionCarrierDelta {
	delta := newSpecEditionCarrierDelta()
	delta.SQLSectionCount = len(sqlSpecSet.Sections)
	delta.CarrierSectionCount = len(carrierSpecSet.Sections)
	delta.SQLClaimCount = activeSpecClaimCount(sqlSpecSet.Sections)
	delta.SQLStoredClaimCount = specClaimCount(sqlSpecSet.Sections)
	delta.CarrierClaimCount = specClaimCount(carrierSpecSet.Sections)
	delta.CarrierObservation = observeSpecCarrier(carrierSpecSet)

	sqlSections := specSectionsByID(sqlSpecSet.Sections)
	carrierSections := specSectionsByID(carrierSpecSet.Sections)
	sectionIDs := make([]string, 0, len(sqlSections)+len(carrierSections))
	seen := make(map[string]struct{}, len(sqlSections)+len(carrierSections))
	for sectionID := range sqlSections {
		seen[sectionID] = struct{}{}
		sectionIDs = append(sectionIDs, sectionID)
	}
	for sectionID := range carrierSections {
		if _, exists := seen[sectionID]; exists {
			continue
		}
		sectionIDs = append(sectionIDs, sectionID)
	}
	sort.Strings(sectionIDs)
	for _, sectionID := range sectionIDs {
		sqlSection, hasSQL := sqlSections[sectionID]
		carrierSection, hasCarrier := carrierSections[sectionID]
		sectionDelta := compareSpecEditionCarrierSection(
			sectionID,
			sqlSection,
			hasSQL,
			carrierSection,
			hasCarrier,
		)
		if sectionDelta.SQLSemanticHash == sectionDelta.CarrierSemanticHash &&
			hasSQL && hasCarrier {
			continue
		}
		delta.Sections = append(delta.Sections, sectionDelta)
	}
	if len(delta.Sections) == 0 {
		delta.Posture = "matches"
	} else {
		delta.Posture = "differs"
	}
	return delta
}

func observeSpecCarrier(
	carrierSpecSet project.ProjectSpecificationSet,
) specCarrierObservation {
	codes := make([]string, 0, len(carrierSpecSet.Findings))
	seen := make(map[string]struct{}, len(carrierSpecSet.Findings))
	for _, finding := range carrierSpecSet.Findings {
		code := strings.TrimSpace(finding.Code)
		if code == "" {
			continue
		}
		if _, duplicate := seen[code]; duplicate {
			continue
		}
		seen[code] = struct{}{}
		codes = append(codes, code)
	}
	sort.Strings(codes)
	posture := "parsed"
	if len(carrierSpecSet.Findings) > 0 {
		posture = "parsed_with_findings"
	}
	return specCarrierObservation{
		Posture:      posture,
		FindingCount: len(carrierSpecSet.Findings),
		FindingCodes: codes,
	}
}

func specClaimCount(sections []project.SpecSection) int {
	total := 0
	for _, section := range sections {
		total += len(section.Claims)
	}
	return total
}

func activeSpecClaimCount(sections []project.SpecSection) int {
	total := 0
	for _, section := range sections {
		if !strings.EqualFold(strings.TrimSpace(section.Status), "active") {
			continue
		}
		total += len(section.Claims)
	}
	return total
}

func specSectionsByID(
	sections []project.SpecSection,
) map[string]project.SpecSection {
	result := make(map[string]project.SpecSection, len(sections))
	for _, section := range sections {
		sectionID := strings.TrimSpace(section.ID)
		if sectionID == "" {
			continue
		}
		result[sectionID] = section
	}
	return result
}

func compareSpecEditionCarrierSection(
	sectionID string,
	sqlSection project.SpecSection,
	hasSQL bool,
	carrierSection project.SpecSection,
	hasCarrier bool,
) specEditionCarrierSectionDelta {
	delta := specEditionCarrierSectionDelta{
		SectionID:              sectionID,
		CarrierAddedClaimIDs:   []string{},
		CarrierRemovedClaimIDs: []string{},
		CarrierChangedClaimIDs: []string{},
	}
	if hasSQL {
		delta.SQLSemanticHash = specflow.HashSection(sqlSection)
	}
	if hasCarrier {
		delta.CarrierSemanticHash = specflow.HashSection(carrierSection)
	}

	sqlClaims := specClaimsByID(sqlSection.Claims)
	carrierClaims := specClaimsByID(carrierSection.Claims)
	for claimID, carrierClaim := range carrierClaims {
		sqlClaim, exists := sqlClaims[claimID]
		if !exists {
			delta.CarrierAddedClaimIDs = append(
				delta.CarrierAddedClaimIDs,
				claimID,
			)
			continue
		}
		if specClaimSemanticHash(sqlClaim) != specClaimSemanticHash(carrierClaim) {
			delta.CarrierChangedClaimIDs = append(
				delta.CarrierChangedClaimIDs,
				claimID,
			)
		}
	}
	for claimID := range sqlClaims {
		if _, exists := carrierClaims[claimID]; exists {
			continue
		}
		delta.CarrierRemovedClaimIDs = append(
			delta.CarrierRemovedClaimIDs,
			claimID,
		)
	}
	sort.Strings(delta.CarrierAddedClaimIDs)
	sort.Strings(delta.CarrierRemovedClaimIDs)
	sort.Strings(delta.CarrierChangedClaimIDs)
	if hasSQL && hasCarrier {
		sqlWithoutClaims := sqlSection
		carrierWithoutClaims := carrierSection
		sqlWithoutClaims.Claims = nil
		carrierWithoutClaims.Claims = nil
		delta.NonClaimFieldsChanged = specflow.HashSection(sqlWithoutClaims) !=
			specflow.HashSection(carrierWithoutClaims)
		delta.ClaimOrderChanged = !slices.Equal(
			specClaimIDs(sqlSection.Claims),
			specClaimIDs(carrierSection.Claims),
		)
	}
	return delta
}

func specClaimsByID(claims []project.SpecClaim) map[string]project.SpecClaim {
	result := make(map[string]project.SpecClaim, len(claims))
	for _, claim := range claims {
		claimID := strings.TrimSpace(claim.ID)
		if claimID == "" {
			continue
		}
		result[claimID] = claim
	}
	return result
}

func specClaimIDs(claims []project.SpecClaim) []string {
	result := make([]string, 0, len(claims))
	for _, claim := range claims {
		claimID := strings.TrimSpace(claim.ID)
		if claimID != "" {
			result = append(result, claimID)
		}
	}
	return result
}

func specClaimSemanticHash(claim project.SpecClaim) string {
	return specflow.HashSection(project.SpecSection{
		ID:     "claim-comparison",
		Claims: []project.SpecClaim{claim},
	})
}

func loadProjectSpecificationSetSQLFirstForScope(
	projectRoot string,
	applicability project.ProjectSpecificationSetApplicability,
) (project.ProjectSpecificationSet, error) {
	sqlSpecSet, hasSQLSource, err := loadProjectSpecificationSetFromSQLEditionsForScope(
		projectRoot,
		applicability,
	)
	if err != nil {
		return project.ProjectSpecificationSet{}, err
	}
	if hasSQLSource {
		carrierSpecSet, carrierErr := project.LoadProjectSpecificationSetForScope(
			projectRoot,
			applicability,
		)
		if carrierErr != nil {
			return sqlSpecSet, nil
		}

		return mergeSQLSpecSetWithCarrierSupport(sqlSpecSet, carrierSpecSet), nil
	}

	return project.LoadProjectSpecificationSetForScope(
		projectRoot,
		applicability,
	)
}

func mergeSQLSpecSetWithCarrierSupport(
	sqlSpecSet project.ProjectSpecificationSet,
	carrierSpecSet project.ProjectSpecificationSet,
) project.ProjectSpecificationSet {
	termMapPaths := termMapDocumentPaths(carrierSpecSet.Documents)

	merged := sqlSpecSet
	merged.Documents = append(append([]project.SpecDocument{}, sqlSpecSet.Documents...), termMapDocuments(carrierSpecSet.Documents)...)
	merged.TermMapEntries = append([]project.TermMapEntry{}, carrierSpecSet.TermMapEntries...)
	merged.Findings = append(append([]project.SpecCheckFinding{}, sqlSpecSet.Findings...), termMapFindings(carrierSpecSet.Findings, termMapPaths)...)
	merged.Findings = append(merged.Findings, specMigrationRequiredFindings(carrierSpecSet.Findings)...)

	return merged
}

func specMigrationRequiredFindings(findings []project.SpecCheckFinding) []project.SpecCheckFinding {
	out := []project.SpecCheckFinding{}
	for _, finding := range findings {
		if finding.Code != project.SpecMigrationRequiredFindingCode {
			continue
		}
		out = append(out, finding)
	}
	return out
}

func termMapDocuments(documents []project.SpecDocument) []project.SpecDocument {
	out := []project.SpecDocument{}
	for _, document := range documents {
		if document.Kind != project.SpecDocumentKindTermMap {
			continue
		}
		out = append(out, document)
	}
	return out
}

func termMapDocumentPaths(documents []project.SpecDocument) map[string]bool {
	paths := map[string]bool{}
	for _, document := range termMapDocuments(documents) {
		path := strings.TrimSpace(document.Path)
		if path == "" {
			continue
		}
		paths[path] = true
	}
	return paths
}

func termMapFindings(findings []project.SpecCheckFinding, termMapPaths map[string]bool) []project.SpecCheckFinding {
	out := []project.SpecCheckFinding{}
	for _, finding := range findings {
		path := strings.TrimSpace(finding.Path)
		if path == "" || !termMapPaths[path] {
			continue
		}
		out = append(out, finding)
	}
	return out
}

func loadProjectSpecificationSetFromSQLEditions(projectRoot string) (project.ProjectSpecificationSet, bool, error) {
	return loadProjectSpecificationSetFromSQLEditionsWith(
		projectRoot,
		specflow.ProjectSpecificationSetFromEditions,
	)
}

func loadProjectSpecificationSetFromSQLEditionsForScope(
	projectRoot string,
	applicability project.ProjectSpecificationSetApplicability,
) (project.ProjectSpecificationSet, bool, error) {
	projector := func(
		editions []specflow.SpecSectionEdition,
	) (project.ProjectSpecificationSet, error) {
		return specflow.ProjectSpecificationSetFromEditionsForScope(
			editions,
			applicability,
		)
	}
	return loadProjectSpecificationSetFromSQLEditionsWith(
		projectRoot,
		projector,
	)
}

func loadProjectSpecificationSetFromSQLEditionsWith(
	projectRoot string,
	projector func([]specflow.SpecSectionEdition) (project.ProjectSpecificationSet, error),
) (project.ProjectSpecificationSet, bool, error) {
	cfg, err := project.Load(haftDirFor(projectRoot))
	if err != nil {
		return project.ProjectSpecificationSet{}, false, err
	}
	if cfg == nil {
		return project.ProjectSpecificationSet{}, false, nil
	}

	projectID, store, closeStore, err := openSpecSectionEditionReadStore(
		context.Background(),
		projectRoot,
		cfg,
	)
	if err != nil {
		return project.ProjectSpecificationSet{}, false, err
	}
	defer closeStore()

	editions, err := store.ListCurrent(projectID)
	if err != nil {
		if errors.Is(err, specflow.ErrSpecSectionEditionNotFound) {
			return project.ProjectSpecificationSet{}, false, nil
		}
		return project.ProjectSpecificationSet{}, false, fmt.Errorf("read SQL spec section editions: %w", err)
	}
	if len(editions) == 0 {
		return project.ProjectSpecificationSet{}, false, nil
	}

	specSet, err := projector(editions)
	if err != nil {
		return project.ProjectSpecificationSet{}, true, fmt.Errorf("project specification SQL edition projection: %w", err)
	}

	return specSet, true, nil
}
