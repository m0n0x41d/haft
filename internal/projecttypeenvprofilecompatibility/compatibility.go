// Package projecttypeenvprofilecompatibility assesses immutable memory-view
// profiles against an exact project TypeEnv successor without participating in
// TypeEnv selection or profile installation effects.
package projecttypeenvprofilecompatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/m0n0x41d/haft/internal/projectmemory/projectionprofile"
	typeenvcompatibility "github.com/m0n0x41d/haft/internal/projecttypeenvcompatibility"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

const (
	profileCompatibilityCanonicalDomain = "haft.projection-profile-typeenv-compatibility.v1"
	profileGroundCanonicalDomain        = "haft.projection-profile-typeenv-ground.v1"
	profileFacetIssueCanonicalDomain    = "haft.projection-profile-typeenv-facet-issue.v1"
	maximumProfileCompatibilityBytes    = 32 << 20
	maximumProfileCompatibilityGrounds  = 1 << 20
	maximumProfileCompatibilityField    = 1 << 20
	maximumProfileEdition               = 1<<32 - 1
)

// ProjectionProfileCompatibility is a closed review result for one exact
// immutable ProjectionProfile edition against one exact successor diff.
// Compatible means every predecessor-visible declared SlotKind read remains
// exact and every declared role absent from both complete successor surfaces
// remains explicitly absent. DegradedFacets means reads still exist but their
// domain expanded. Blocked means a declared read was narrowed, removed, is
// missing only from the target, or cannot be ordered by the compiler.
type ProjectionProfileCompatibility interface {
	Kind() ProjectionProfileCompatibilityKind
	ProfileRef() projectionprofile.Ref
	ProfileEdition() uint32
	ProfileDigest() typedmemory.SHA256Digest
	BaseTypeEnv() typedmemory.TypeEnvRef
	TargetTypeEnv() typedmemory.TypeEnvRef
	SuccessorDiffDigest() typedmemory.SHA256Digest
	Grounds() []ProjectionProfileCompatibilityGround
	FacetIssues() []ProjectionProfileFacetIssue
	AffectedFacets() []projectionprofile.FacetKind
	CanonicalBytes() []byte
	Digest() typedmemory.SHA256Digest
	Verify() error
	projectionProfileCompatibilityVariant()
}

type ProjectionProfileCompatibilityKind uint8

const (
	ProfileCompatible ProjectionProfileCompatibilityKind = iota + 1
	ProfileDegradedFacets
	ProfileBlocked
)

func (kind ProjectionProfileCompatibilityKind) String() string {
	switch kind {
	case ProfileCompatible:
		return "compatible"
	case ProfileDegradedFacets:
		return "degraded_facets"
	case ProfileBlocked:
		return "blocked"
	default:
		return ""
	}
}

func parseProjectionProfileCompatibilityKind(
	raw string,
) (ProjectionProfileCompatibilityKind, error) {
	kinds := []ProjectionProfileCompatibilityKind{
		ProfileCompatible,
		ProfileDegradedFacets,
		ProfileBlocked,
	}
	for _, kind := range kinds {
		if kind.String() == raw {
			return kind, nil
		}
	}
	return 0, fmt.Errorf("unsupported projection-profile compatibility %q", raw)
}

type ProfileCompatibilityGroundKind string

const (
	ProfileGroundDeclaredSlotUnchanged   ProfileCompatibilityGroundKind = "declared_slot_unchanged"
	ProfileGroundDeclaredSlotAdded       ProfileCompatibilityGroundKind = "declared_slot_added"
	ProfileGroundDeclaredSlotWidened     ProfileCompatibilityGroundKind = "declared_slot_widened"
	ProfileGroundDeclaredSlotNarrowed    ProfileCompatibilityGroundKind = "declared_slot_narrowed"
	ProfileGroundDeclaredSlotRemoved     ProfileCompatibilityGroundKind = "declared_slot_removed"
	ProfileGroundDeclaredSlotCompilerGap ProfileCompatibilityGroundKind = "declared_slot_compiler_gap"
	// ProfileGroundDeclaredSlotMissing remains decodable for existing v1
	// artifacts. New complete successor assessments distinguish absence from
	// both surfaces instead of emitting this historical posture.
	ProfileGroundDeclaredSlotMissing    ProfileCompatibilityGroundKind = "declared_slot_missing_from_target"
	ProfileGroundDeclaredSlotAbsentBoth ProfileCompatibilityGroundKind = "declared_slot_absent_from_both"
)

var knownProfileCompatibilityGroundKinds = map[ProfileCompatibilityGroundKind]struct{}{
	ProfileGroundDeclaredSlotUnchanged:   {},
	ProfileGroundDeclaredSlotAdded:       {},
	ProfileGroundDeclaredSlotWidened:     {},
	ProfileGroundDeclaredSlotNarrowed:    {},
	ProfileGroundDeclaredSlotRemoved:     {},
	ProfileGroundDeclaredSlotCompilerGap: {},
	ProfileGroundDeclaredSlotMissing:     {},
	ProfileGroundDeclaredSlotAbsentBoth:  {},
}

type ProfileCompatibilityGroundPosture uint8

const (
	ProfileGroundSatisfied ProfileCompatibilityGroundPosture = iota + 1
	ProfileGroundDegraded
	ProfileGroundBlocking
)

func (posture ProfileCompatibilityGroundPosture) String() string {
	switch posture {
	case ProfileGroundSatisfied:
		return "satisfied"
	case ProfileGroundDegraded:
		return "degraded"
	case ProfileGroundBlocking:
		return "blocking"
	default:
		return ""
	}
}

// ProjectionProfileCompatibilityGround names the exact declared SlotKind and
// successor coordinate used by the assessment. Empty RuleKey is legal for the
// explicit absent-from-both ground and for decoding the historical blocking
// missing-from-target ground.
type ProjectionProfileCompatibilityGround struct {
	slot    typedmemory.SlotKindID
	kind    ProfileCompatibilityGroundKind
	posture ProfileCompatibilityGroundPosture
	ruleKey string
}

// ProjectionProfileFacetIssue is the exact facet-local consequence of one
// non-satisfied declared profile dependency. It does not claim that a facet
// is empty or false; it says that this immutable profile edition can no longer
// project that facet under the target TypeEnv with its former exact contract.
type ProjectionProfileFacetIssue struct {
	facet   projectionprofile.FacetKind
	slot    typedmemory.SlotKindID
	kind    ProfileCompatibilityGroundKind
	posture ProfileCompatibilityGroundPosture
	ruleKey string
}

func (issue ProjectionProfileFacetIssue) Facet() projectionprofile.FacetKind {
	return issue.facet
}

func (issue ProjectionProfileFacetIssue) SlotKind() typedmemory.SlotKindID {
	return issue.slot
}

func (issue ProjectionProfileFacetIssue) Kind() ProfileCompatibilityGroundKind {
	return issue.kind
}

func (issue ProjectionProfileFacetIssue) Posture() ProfileCompatibilityGroundPosture {
	return issue.posture
}

func (issue ProjectionProfileFacetIssue) RuleKey() string { return issue.ruleKey }

func (issue ProjectionProfileFacetIssue) CanonicalBytes() []byte {
	writer := newCanonicalWriter(profileFacetIssueCanonicalDomain)
	writer.addString(string(issue.facet))
	writer.addString(issue.slot.String())
	writer.addString(string(issue.kind))
	writer.addString(issue.posture.String())
	writer.addString(issue.ruleKey)
	return writer.bytes()
}

func (issue ProjectionProfileFacetIssue) verify() error {
	if !issue.facet.Valid() {
		return fmt.Errorf("profile compatibility facet issue has invalid facet")
	}
	ground := ProjectionProfileCompatibilityGround{
		slot:    issue.slot,
		kind:    issue.kind,
		posture: issue.posture,
		ruleKey: issue.ruleKey,
	}
	if err := ground.verify(); err != nil {
		return fmt.Errorf("profile compatibility facet issue: %w", err)
	}
	if issue.posture == ProfileGroundSatisfied {
		return fmt.Errorf("profile compatibility facet issue cannot be satisfied")
	}
	return nil
}

func (ground ProjectionProfileCompatibilityGround) SlotKind() typedmemory.SlotKindID {
	return ground.slot
}

func (ground ProjectionProfileCompatibilityGround) Kind() ProfileCompatibilityGroundKind {
	return ground.kind
}

func (ground ProjectionProfileCompatibilityGround) Posture() ProfileCompatibilityGroundPosture {
	return ground.posture
}

func (ground ProjectionProfileCompatibilityGround) RuleKey() string { return ground.ruleKey }

func (ground ProjectionProfileCompatibilityGround) CanonicalBytes() []byte {
	writer := newCanonicalWriter(profileGroundCanonicalDomain)
	writer.addString(ground.slot.String())
	writer.addString(string(ground.kind))
	writer.addString(ground.posture.String())
	writer.addString(ground.ruleKey)
	return writer.bytes()
}

func (ground ProjectionProfileCompatibilityGround) verify() error {
	if _, err := typedmemory.NewSlotKindID(ground.slot.String()); err != nil {
		return fmt.Errorf("profile compatibility SlotKind is invalid")
	}
	if _, found := knownProfileCompatibilityGroundKinds[ground.kind]; !found {
		return fmt.Errorf("profile compatibility ground kind is invalid")
	}
	if ground.posture.String() == "" {
		return fmt.Errorf("profile compatibility ground posture is invalid")
	}
	if ground.kind == ProfileGroundDeclaredSlotMissing {
		if ground.ruleKey != "" || ground.posture != ProfileGroundBlocking {
			return fmt.Errorf("missing-slot ground has invalid basis")
		}
		return nil
	}
	if ground.kind == ProfileGroundDeclaredSlotAbsentBoth {
		if ground.ruleKey != "" || ground.posture != ProfileGroundSatisfied {
			return fmt.Errorf("absent-from-both slot ground has invalid basis")
		}
		return nil
	}
	if ground.ruleKey == "" {
		return fmt.Errorf("profile compatibility rule coordinate is required")
	}
	expected := postureForProfileGroundKind(ground.kind)
	if ground.posture != expected {
		return fmt.Errorf("profile compatibility ground posture disagrees with kind")
	}
	return nil
}

type projectionProfileCompatibilityState struct {
	kind        ProjectionProfileCompatibilityKind
	profileRef  projectionprofile.Ref
	profileEdit uint32
	profileHash typedmemory.SHA256Digest
	base        typedmemory.TypeEnvRef
	target      typedmemory.TypeEnvRef
	diffDigest  typedmemory.SHA256Digest
	grounds     []ProjectionProfileCompatibilityGround
	issues      []ProjectionProfileFacetIssue
	facets      []projectionprofile.FacetKind
	digest      typedmemory.SHA256Digest
}

type CompatibleProjectionProfile struct {
	state projectionProfileCompatibilityState
}

type DegradedProjectionProfileFacets struct {
	state projectionProfileCompatibilityState
}

type BlockedProjectionProfile struct {
	state projectionProfileCompatibilityState
}

func (CompatibleProjectionProfile) projectionProfileCompatibilityVariant()     {}
func (DegradedProjectionProfileFacets) projectionProfileCompatibilityVariant() {}
func (BlockedProjectionProfile) projectionProfileCompatibilityVariant()        {}

func (value CompatibleProjectionProfile) Kind() ProjectionProfileCompatibilityKind {
	return value.state.kind
}

func (value DegradedProjectionProfileFacets) Kind() ProjectionProfileCompatibilityKind {
	return value.state.kind
}

func (value BlockedProjectionProfile) Kind() ProjectionProfileCompatibilityKind {
	return value.state.kind
}

func (value CompatibleProjectionProfile) ProfileRef() projectionprofile.Ref {
	return value.state.profileRef
}

func (value DegradedProjectionProfileFacets) ProfileRef() projectionprofile.Ref {
	return value.state.profileRef
}

func (value BlockedProjectionProfile) ProfileRef() projectionprofile.Ref {
	return value.state.profileRef
}

func (value CompatibleProjectionProfile) ProfileEdition() uint32 {
	return value.state.profileEdit
}

func (value DegradedProjectionProfileFacets) ProfileEdition() uint32 {
	return value.state.profileEdit
}

func (value BlockedProjectionProfile) ProfileEdition() uint32 {
	return value.state.profileEdit
}

func (value CompatibleProjectionProfile) ProfileDigest() typedmemory.SHA256Digest {
	return value.state.profileHash
}

func (value DegradedProjectionProfileFacets) ProfileDigest() typedmemory.SHA256Digest {
	return value.state.profileHash
}

func (value BlockedProjectionProfile) ProfileDigest() typedmemory.SHA256Digest {
	return value.state.profileHash
}

func (value CompatibleProjectionProfile) BaseTypeEnv() typedmemory.TypeEnvRef {
	return value.state.base
}

func (value DegradedProjectionProfileFacets) BaseTypeEnv() typedmemory.TypeEnvRef {
	return value.state.base
}

func (value BlockedProjectionProfile) BaseTypeEnv() typedmemory.TypeEnvRef {
	return value.state.base
}

func (value CompatibleProjectionProfile) TargetTypeEnv() typedmemory.TypeEnvRef {
	return value.state.target
}

func (value DegradedProjectionProfileFacets) TargetTypeEnv() typedmemory.TypeEnvRef {
	return value.state.target
}

func (value BlockedProjectionProfile) TargetTypeEnv() typedmemory.TypeEnvRef {
	return value.state.target
}

func (value CompatibleProjectionProfile) SuccessorDiffDigest() typedmemory.SHA256Digest {
	return value.state.diffDigest
}

func (value DegradedProjectionProfileFacets) SuccessorDiffDigest() typedmemory.SHA256Digest {
	return value.state.diffDigest
}

func (value BlockedProjectionProfile) SuccessorDiffDigest() typedmemory.SHA256Digest {
	return value.state.diffDigest
}

func (value CompatibleProjectionProfile) Grounds() []ProjectionProfileCompatibilityGround {
	return cloneProfileCompatibilityGrounds(value.state.grounds)
}

func (value DegradedProjectionProfileFacets) Grounds() []ProjectionProfileCompatibilityGround {
	return cloneProfileCompatibilityGrounds(value.state.grounds)
}

func (value BlockedProjectionProfile) Grounds() []ProjectionProfileCompatibilityGround {
	return cloneProfileCompatibilityGrounds(value.state.grounds)
}

func (value CompatibleProjectionProfile) FacetIssues() []ProjectionProfileFacetIssue {
	return cloneProjectionProfileFacetIssues(value.state.issues)
}

func (value DegradedProjectionProfileFacets) FacetIssues() []ProjectionProfileFacetIssue {
	return cloneProjectionProfileFacetIssues(value.state.issues)
}

func (value BlockedProjectionProfile) FacetIssues() []ProjectionProfileFacetIssue {
	return cloneProjectionProfileFacetIssues(value.state.issues)
}

func (value CompatibleProjectionProfile) AffectedFacets() []projectionprofile.FacetKind {
	return append([]projectionprofile.FacetKind(nil), value.state.facets...)
}

func (value DegradedProjectionProfileFacets) AffectedFacets() []projectionprofile.FacetKind {
	return append([]projectionprofile.FacetKind(nil), value.state.facets...)
}

func (value BlockedProjectionProfile) AffectedFacets() []projectionprofile.FacetKind {
	return append([]projectionprofile.FacetKind(nil), value.state.facets...)
}

func (value CompatibleProjectionProfile) CanonicalBytes() []byte {
	return profileCompatibilityCanonicalBytes(value.state)
}

func (value DegradedProjectionProfileFacets) CanonicalBytes() []byte {
	return profileCompatibilityCanonicalBytes(value.state)
}

func (value BlockedProjectionProfile) CanonicalBytes() []byte {
	return profileCompatibilityCanonicalBytes(value.state)
}

func (value CompatibleProjectionProfile) Digest() typedmemory.SHA256Digest {
	return value.state.digest
}

func (value DegradedProjectionProfileFacets) Digest() typedmemory.SHA256Digest {
	return value.state.digest
}

func (value BlockedProjectionProfile) Digest() typedmemory.SHA256Digest {
	return value.state.digest
}

func (value CompatibleProjectionProfile) Verify() error {
	return verifyProfileCompatibilityState(value.state)
}

func (value DegradedProjectionProfileFacets) Verify() error {
	return verifyProfileCompatibilityState(value.state)
}

func (value BlockedProjectionProfile) Verify() error {
	return verifyProfileCompatibilityState(value.state)
}

// AssessProjectionProfile evaluates only dependencies the immutable v1
// profile declares: exact SlotKind reads. The current EntityKind policy is
// AnyAdmitted, and item/facet/conformance rules are profile-owned and covered
// by the profile digest. The result never invents an undeclared dependency.
// Because v1 has no SlotKind-to-facet dependency map, any affected declared
// slot conservatively affects every facet in that exact profile edition.
func AssessProjectionProfile(
	profile projectionprofile.Descriptor,
	diff typeenvcompatibility.SuccessorDiff,
) (ProjectionProfileCompatibility, error) {
	if !profile.Valid() {
		return nil, fmt.Errorf("projection profile is invalid")
	}
	if err := diff.Verify(); err != nil {
		return nil, fmt.Errorf("successor diff: %w", err)
	}
	grounds := assessProjectionProfileSlots(profile, diff)
	kind := deriveProjectionProfileCompatibilityKind(grounds)
	issues := projectionProfileFacetIssues(profile, grounds)
	facets := affectedProjectionProfileFacets(issues)
	state := projectionProfileCompatibilityState{
		kind:        kind,
		profileRef:  profile.Ref(),
		profileEdit: profile.Edition(),
		profileHash: profile.Digest(),
		base:        diff.Base(),
		target:      diff.Target(),
		diffDigest:  diff.Digest(),
		grounds:     grounds,
		issues:      issues,
		facets:      facets,
	}
	digest, err := digestBytes(profileCompatibilityCanonicalBytes(state))
	if err != nil {
		return nil, err
	}
	state.digest = digest
	return projectionProfileCompatibilityFromState(state)
}

func assessProjectionProfileSlots(
	profile projectionprofile.Descriptor,
	diff typeenvcompatibility.SuccessorDiff,
) []ProjectionProfileCompatibilityGround {
	grounds := make([]ProjectionProfileCompatibilityGround, 0)
	for _, slot := range profile.SlotReads() {
		matched := false
		for _, rule := range diff.Rules() {
			if !successorRuleMatchesSlot(rule, slot) {
				continue
			}
			matched = true
			grounds = append(grounds, profileGroundFromSuccessorRule(slot, rule))
		}
		if matched {
			continue
		}
		// SuccessorDiff is the complete union of predecessor and target semantic
		// rows, including unchanged rows. No matching role therefore proves
		// absence from both surfaces; it is not evidence of target removal.
		grounds = append(grounds, ProjectionProfileCompatibilityGround{
			slot:    slot,
			kind:    ProfileGroundDeclaredSlotAbsentBoth,
			posture: ProfileGroundSatisfied,
		})
	}
	sort.Slice(grounds, func(left, right int) bool {
		return profileCompatibilityGroundKey(grounds[left]) < profileCompatibilityGroundKey(grounds[right])
	})
	return grounds
}

func successorRuleMatchesSlot(
	rule typeenvcompatibility.SuccessorRuleAssessment,
	slot typedmemory.SlotKindID,
) bool {
	if rule.Family() != typeenvcompatibility.RelationSlotFamily {
		return false
	}
	separator := "/slot/"
	index := strings.LastIndex(rule.Key(), separator)
	if index < 0 {
		return false
	}
	ruleSlot, err := typedmemory.NewSlotKindID(
		rule.Key()[index+len(separator):],
	)
	if err != nil {
		return false
	}
	return slotKindMatchesDeclaredRead(slot, ruleSlot)
}

// slotKindMatchesDeclaredRead preserves the profile v1 distinction between a
// generic declared role (for example ClaimGraphSlot) and an exact qualified
// relation-local SlotKind. A generic role covers every canonical SlotKind
// whose final source-name component is exactly that role; a qualified
// declaration remains exact and cannot alias a sibling relation's slot.
func slotKindMatchesDeclaredRead(
	declared typedmemory.SlotKindID,
	actual typedmemory.SlotKindID,
) bool {
	if declared == actual {
		return true
	}
	declaredName := declared.String()
	if strings.Contains(declaredName, ".") {
		return false
	}
	actualName := actual.String()
	separator := strings.LastIndex(actualName, ".")
	if separator < 0 {
		return false
	}
	return actualName[separator+1:] == declaredName
}

func profileGroundFromSuccessorRule(
	slot typedmemory.SlotKindID,
	rule typeenvcompatibility.SuccessorRuleAssessment,
) ProjectionProfileCompatibilityGround {
	kind := profileGroundKindForSuccessorClass(rule.Class())
	return ProjectionProfileCompatibilityGround{
		slot:    slot,
		kind:    kind,
		posture: postureForProfileGroundKind(kind),
		ruleKey: rule.Key(),
	}
}

func profileGroundKindForSuccessorClass(
	class typeenvcompatibility.SuccessorRuleClass,
) ProfileCompatibilityGroundKind {
	switch class {
	case typeenvcompatibility.SuccessorUnchanged:
		return ProfileGroundDeclaredSlotUnchanged
	case typeenvcompatibility.SuccessorAdditive:
		return ProfileGroundDeclaredSlotAdded
	case typeenvcompatibility.SuccessorWidened:
		return ProfileGroundDeclaredSlotWidened
	case typeenvcompatibility.SuccessorNarrowed:
		return ProfileGroundDeclaredSlotNarrowed
	case typeenvcompatibility.SuccessorRemoved:
		return ProfileGroundDeclaredSlotRemoved
	default:
		return ProfileGroundDeclaredSlotCompilerGap
	}
}

func postureForProfileGroundKind(
	kind ProfileCompatibilityGroundKind,
) ProfileCompatibilityGroundPosture {
	switch kind {
	case ProfileGroundDeclaredSlotUnchanged,
		ProfileGroundDeclaredSlotAbsentBoth:
		return ProfileGroundSatisfied
	case ProfileGroundDeclaredSlotAdded, ProfileGroundDeclaredSlotWidened:
		return ProfileGroundDegraded
	default:
		return ProfileGroundBlocking
	}
}

func deriveProjectionProfileCompatibilityKind(
	grounds []ProjectionProfileCompatibilityGround,
) ProjectionProfileCompatibilityKind {
	result := ProfileCompatible
	for _, ground := range grounds {
		if ground.posture == ProfileGroundBlocking {
			return ProfileBlocked
		}
		if ground.posture == ProfileGroundDegraded {
			result = ProfileDegradedFacets
		}
	}
	return result
}

func projectionProfileFacetIssues(
	profile projectionprofile.Descriptor,
	grounds []ProjectionProfileCompatibilityGround,
) []ProjectionProfileFacetIssue {
	issues := make([]ProjectionProfileFacetIssue, 0)
	for _, ground := range grounds {
		if ground.posture == ProfileGroundSatisfied {
			continue
		}
		for _, facet := range profile.Facets() {
			issues = append(issues, ProjectionProfileFacetIssue{
				facet:   facet,
				slot:    ground.slot,
				kind:    ground.kind,
				posture: ground.posture,
				ruleKey: ground.ruleKey,
			})
		}
	}
	sort.Slice(issues, func(left, right int) bool {
		return profileFacetIssueKey(issues[left]) < profileFacetIssueKey(issues[right])
	})
	return issues
}

func affectedProjectionProfileFacets(
	issues []ProjectionProfileFacetIssue,
) []projectionprofile.FacetKind {
	values := make([]projectionprofile.FacetKind, 0, len(issues))
	for _, issue := range issues {
		values = append(values, issue.facet)
	}
	sort.Slice(values, func(left, right int) bool {
		return values[left] < values[right]
	})
	return slicesCompactFacetKinds(values)
}

func profileCompatibilityCanonicalBytes(
	state projectionProfileCompatibilityState,
) []byte {
	writer := newCanonicalWriter(profileCompatibilityCanonicalDomain)
	writer.addString(state.kind.String())
	writer.addString(state.profileRef.String())
	writer.addUint64(uint64(state.profileEdit))
	writer.addString(state.profileHash.String())
	writer.addString(state.base.String())
	writer.addString(state.target.String())
	writer.addString(state.diffDigest.String())
	writer.addUint64(uint64(len(state.grounds)))
	for _, ground := range state.grounds {
		writer.addBytes(ground.CanonicalBytes())
	}
	writer.addUint64(uint64(len(state.issues)))
	for _, issue := range state.issues {
		writer.addBytes(issue.CanonicalBytes())
	}
	writer.addUint64(uint64(len(state.facets)))
	for _, facet := range state.facets {
		writer.addString(string(facet))
	}
	return writer.bytes()
}

func projectionProfileCompatibilityFromState(
	state projectionProfileCompatibilityState,
) (ProjectionProfileCompatibility, error) {
	var result ProjectionProfileCompatibility
	switch state.kind {
	case ProfileCompatible:
		result = CompatibleProjectionProfile{state: state}
	case ProfileDegradedFacets:
		result = DegradedProjectionProfileFacets{state: state}
	case ProfileBlocked:
		result = BlockedProjectionProfile{state: state}
	default:
		return nil, fmt.Errorf("projection-profile compatibility kind is invalid")
	}
	if err := result.Verify(); err != nil {
		return nil, err
	}
	return result, nil
}

func verifyProfileCompatibilityState(state projectionProfileCompatibilityState) error {
	if state.kind.String() == "" {
		return fmt.Errorf("projection-profile compatibility kind is invalid")
	}
	ref, err := projectionprofile.ParseRef(state.profileRef.String())
	if err != nil || ref != state.profileRef {
		return fmt.Errorf("projection-profile compatibility ref is invalid")
	}
	profile, found := projectionprofile.Lookup(state.profileRef)
	if !found {
		return fmt.Errorf("projection-profile compatibility ref is not installed")
	}
	if profile.Edition() != state.profileEdit || state.profileEdit == 0 {
		return fmt.Errorf("projection-profile compatibility edition is invalid")
	}
	if profile.Digest() != state.profileHash {
		return fmt.Errorf("projection-profile compatibility profile digest is invalid")
	}
	if _, err := typedmemory.ParseTypeEnvRef(state.base.String()); err != nil {
		return fmt.Errorf("projection-profile compatibility base is invalid")
	}
	if _, err := typedmemory.ParseTypeEnvRef(state.target.String()); err != nil {
		return fmt.Errorf("projection-profile compatibility target is invalid")
	}
	if _, err := typedmemory.NewSHA256Digest(state.diffDigest.String()); err != nil {
		return fmt.Errorf("projection-profile compatibility diff digest is invalid")
	}
	if len(state.grounds) == 0 || len(state.grounds) > maximumProfileCompatibilityGrounds {
		return fmt.Errorf("projection-profile compatibility grounds are invalid")
	}
	for index, ground := range state.grounds {
		if err := ground.verify(); err != nil {
			return fmt.Errorf("projection-profile compatibility ground %d: %w", index, err)
		}
		if index > 0 && profileCompatibilityGroundKey(state.grounds[index-1]) >= profileCompatibilityGroundKey(ground) {
			return fmt.Errorf("projection-profile compatibility grounds are not canonical")
		}
	}
	derivedKind := deriveProjectionProfileCompatibilityKind(state.grounds)
	if derivedKind != state.kind {
		return fmt.Errorf("projection-profile compatibility kind disagrees with grounds")
	}
	if err := verifyProfileGroundCoverage(profile, state.grounds); err != nil {
		return err
	}
	if err := verifyProfileFacetIssues(profile, state.grounds, state.issues); err != nil {
		return err
	}
	if err := verifyAffectedFacets(state.kind, state.issues, state.facets); err != nil {
		return err
	}
	digest, err := digestBytes(profileCompatibilityCanonicalBytes(state))
	if err != nil {
		return err
	}
	if digest != state.digest {
		return fmt.Errorf("projection-profile compatibility digest mismatch")
	}
	return nil
}

func verifyAffectedFacets(
	kind ProjectionProfileCompatibilityKind,
	issues []ProjectionProfileFacetIssue,
	facets []projectionprofile.FacetKind,
) error {
	if kind == ProfileCompatible && len(facets) != 0 {
		return fmt.Errorf("compatible projection profile has affected facets")
	}
	if kind != ProfileCompatible && len(facets) == 0 {
		return fmt.Errorf("non-compatible projection profile has no affected facets")
	}
	for index, facet := range facets {
		if !facet.Valid() {
			return fmt.Errorf("projection-profile compatibility facet is invalid")
		}
		if index > 0 && facets[index-1] >= facet {
			return fmt.Errorf("projection-profile compatibility facets are not canonical")
		}
	}
	expected := affectedProjectionProfileFacets(issues)
	if !equalFacetKinds(facets, expected) {
		return fmt.Errorf("projection-profile compatibility facets disagree with exact profile")
	}
	return nil
}

func verifyProfileFacetIssues(
	profile projectionprofile.Descriptor,
	grounds []ProjectionProfileCompatibilityGround,
	issues []ProjectionProfileFacetIssue,
) error {
	for index, issue := range issues {
		if err := issue.verify(); err != nil {
			return fmt.Errorf("projection-profile facet issue %d: %w", index, err)
		}
		if !profile.AllowsFacet(issue.facet) {
			return fmt.Errorf("projection-profile facet issue names an undeclared facet")
		}
		if index > 0 && profileFacetIssueKey(issues[index-1]) >= profileFacetIssueKey(issue) {
			return fmt.Errorf("projection-profile facet issues are not canonical")
		}
	}
	expected := projectionProfileFacetIssues(profile, grounds)
	if !equalProjectionProfileFacetIssues(issues, expected) {
		return fmt.Errorf("projection-profile facet issues disagree with grounds")
	}
	return nil
}

func verifyProfileGroundCoverage(
	profile projectionprofile.Descriptor,
	grounds []ProjectionProfileCompatibilityGround,
) error {
	expected := make(map[string]struct{}, len(profile.SlotReads()))
	for _, slot := range profile.SlotReads() {
		expected[slot.String()] = struct{}{}
	}
	covered := make(map[string]struct{}, len(expected))
	for _, ground := range grounds {
		if _, found := expected[ground.slot.String()]; !found {
			return fmt.Errorf("profile compatibility ground names an undeclared SlotKind")
		}
		covered[ground.slot.String()] = struct{}{}
	}
	if len(covered) != len(expected) {
		return fmt.Errorf("profile compatibility grounds do not cover every declared SlotKind")
	}
	return nil
}

func equalFacetKinds(
	left []projectionprofile.FacetKind,
	right []projectionprofile.FacetKind,
) bool {
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

func profileCompatibilityGroundKey(
	ground ProjectionProfileCompatibilityGround,
) string {
	return ground.slot.String() + "\x00" + ground.ruleKey + "\x00" + string(ground.kind)
}

func cloneProfileCompatibilityGrounds(
	values []ProjectionProfileCompatibilityGround,
) []ProjectionProfileCompatibilityGround {
	return append([]ProjectionProfileCompatibilityGround(nil), values...)
}

func cloneProjectionProfileFacetIssues(
	values []ProjectionProfileFacetIssue,
) []ProjectionProfileFacetIssue {
	return append([]ProjectionProfileFacetIssue(nil), values...)
}

func profileFacetIssueKey(issue ProjectionProfileFacetIssue) string {
	return string(issue.facet) + "\x00" +
		issue.slot.String() + "\x00" +
		issue.ruleKey + "\x00" +
		string(issue.kind)
}

func equalProjectionProfileFacetIssues(
	left []ProjectionProfileFacetIssue,
	right []ProjectionProfileFacetIssue,
) bool {
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

func slicesCompactFacetKinds(
	values []projectionprofile.FacetKind,
) []projectionprofile.FacetKind {
	if len(values) == 0 {
		return values
	}
	result := append([]projectionprofile.FacetKind(nil), values[:1]...)
	for _, value := range values[1:] {
		if result[len(result)-1] == value {
			continue
		}
		result = append(result, value)
	}
	return result
}

func DecodeProjectionProfileCompatibility(
	canonical []byte,
) (ProjectionProfileCompatibility, error) {
	if len(canonical) == 0 || len(canonical) > maximumProfileCompatibilityBytes {
		return nil, fmt.Errorf("projection-profile compatibility byte length is invalid")
	}
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("profile compatibility domain")
	if err != nil {
		return nil, err
	}
	if domain != profileCompatibilityCanonicalDomain {
		return nil, fmt.Errorf("projection-profile compatibility domain is invalid")
	}
	kindText, err := reader.readString("profile compatibility kind")
	if err != nil {
		return nil, err
	}
	kind, err := parseProjectionProfileCompatibilityKind(kindText)
	if err != nil {
		return nil, err
	}
	profileRefText, err := reader.readString("profile compatibility ref")
	if err != nil {
		return nil, err
	}
	profileRef, err := projectionprofile.ParseRef(profileRefText)
	if err != nil {
		return nil, err
	}
	profileEdition, err := reader.readCount(
		"profile compatibility edition",
		maximumProfileEdition,
	)
	if err != nil || profileEdition == 0 {
		return nil, fmt.Errorf("projection-profile compatibility edition is invalid")
	}
	profileHash, err := reader.readDigest("profile compatibility profile digest")
	if err != nil {
		return nil, err
	}
	baseText, err := reader.readString("profile compatibility base")
	if err != nil {
		return nil, err
	}
	base, err := typedmemory.ParseTypeEnvRef(baseText)
	if err != nil {
		return nil, err
	}
	targetText, err := reader.readString("profile compatibility target")
	if err != nil {
		return nil, err
	}
	target, err := typedmemory.ParseTypeEnvRef(targetText)
	if err != nil {
		return nil, err
	}
	diffDigest, err := reader.readDigest("profile compatibility diff digest")
	if err != nil {
		return nil, err
	}
	groundCount, err := reader.readCount(
		"profile compatibility grounds",
		maximumProfileCompatibilityGrounds,
	)
	if err != nil || groundCount == 0 {
		return nil, fmt.Errorf("projection-profile compatibility ground count is invalid")
	}
	grounds := make([]ProjectionProfileCompatibilityGround, 0, groundCount)
	for index := 0; index < groundCount; index++ {
		raw, readErr := reader.readBytes("profile compatibility ground")
		if readErr != nil {
			return nil, readErr
		}
		ground, decodeErr := decodeProfileCompatibilityGround(raw)
		if decodeErr != nil {
			return nil, decodeErr
		}
		grounds = append(grounds, ground)
	}
	issueCount, err := reader.readCount(
		"profile compatibility facet issues",
		maximumProfileCompatibilityGrounds,
	)
	if err != nil {
		return nil, err
	}
	issues := make([]ProjectionProfileFacetIssue, 0, issueCount)
	for index := 0; index < issueCount; index++ {
		raw, readErr := reader.readBytes("profile compatibility facet issue")
		if readErr != nil {
			return nil, readErr
		}
		issue, decodeErr := decodeProjectionProfileFacetIssue(raw)
		if decodeErr != nil {
			return nil, decodeErr
		}
		issues = append(issues, issue)
	}
	facetCount, err := reader.readCount("profile compatibility facets", len(projectionprofile.KnownFacetKinds()))
	if err != nil {
		return nil, err
	}
	facets := make([]projectionprofile.FacetKind, 0, facetCount)
	for index := 0; index < facetCount; index++ {
		raw, readErr := reader.readString("profile compatibility facet")
		if readErr != nil {
			return nil, readErr
		}
		facets = append(facets, projectionprofile.FacetKind(raw))
	}
	if reader.remaining() != 0 {
		return nil, fmt.Errorf("projection-profile compatibility has trailing bytes")
	}
	profileEditionValue := uint32(profileEdition) // #nosec G115 -- readCount bounded the edition by maximumProfileEdition.
	state := projectionProfileCompatibilityState{
		kind:        kind,
		profileRef:  profileRef,
		profileEdit: profileEditionValue,
		profileHash: profileHash,
		base:        base,
		target:      target,
		diffDigest:  diffDigest,
		grounds:     grounds,
		issues:      issues,
		facets:      facets,
	}
	digest, err := digestBytes(profileCompatibilityCanonicalBytes(state))
	if err != nil {
		return nil, err
	}
	state.digest = digest
	if !bytes.Equal(profileCompatibilityCanonicalBytes(state), canonical) {
		return nil, fmt.Errorf("projection-profile compatibility is not canonical")
	}
	return projectionProfileCompatibilityFromState(state)
}

func decodeProjectionProfileFacetIssue(
	canonical []byte,
) (ProjectionProfileFacetIssue, error) {
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("profile facet issue domain")
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	if domain != profileFacetIssueCanonicalDomain {
		return ProjectionProfileFacetIssue{}, fmt.Errorf("profile facet issue domain is invalid")
	}
	facetText, err := reader.readString("profile facet issue facet")
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	slotText, err := reader.readString("profile facet issue SlotKind")
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	slot, err := typedmemory.NewSlotKindID(slotText)
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	kindText, err := reader.readString("profile facet issue kind")
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	postureText, err := reader.readString("profile facet issue posture")
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	posture, err := parseProfileCompatibilityGroundPosture(postureText)
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	ruleKey, err := reader.readString("profile facet issue rule key")
	if err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	if reader.remaining() != 0 {
		return ProjectionProfileFacetIssue{}, fmt.Errorf("profile facet issue has trailing bytes")
	}
	issue := ProjectionProfileFacetIssue{
		facet:   projectionprofile.FacetKind(facetText),
		slot:    slot,
		kind:    ProfileCompatibilityGroundKind(kindText),
		posture: posture,
		ruleKey: ruleKey,
	}
	if err := issue.verify(); err != nil {
		return ProjectionProfileFacetIssue{}, err
	}
	if !bytes.Equal(issue.CanonicalBytes(), canonical) {
		return ProjectionProfileFacetIssue{}, fmt.Errorf("profile facet issue is not canonical")
	}
	return issue, nil
}

func decodeProfileCompatibilityGround(
	canonical []byte,
) (ProjectionProfileCompatibilityGround, error) {
	reader := newCanonicalReader(canonical)
	domain, err := reader.readString("profile ground domain")
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	if domain != profileGroundCanonicalDomain {
		return ProjectionProfileCompatibilityGround{}, fmt.Errorf("profile ground domain is invalid")
	}
	slotText, err := reader.readString("profile ground SlotKind")
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	slot, err := typedmemory.NewSlotKindID(slotText)
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	kindText, err := reader.readString("profile ground kind")
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	postureText, err := reader.readString("profile ground posture")
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	ruleKey, err := reader.readString("profile ground rule key")
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	if reader.remaining() != 0 {
		return ProjectionProfileCompatibilityGround{}, fmt.Errorf("profile ground has trailing bytes")
	}
	posture, err := parseProfileCompatibilityGroundPosture(postureText)
	if err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	ground := ProjectionProfileCompatibilityGround{
		slot:    slot,
		kind:    ProfileCompatibilityGroundKind(kindText),
		posture: posture,
		ruleKey: ruleKey,
	}
	if err := ground.verify(); err != nil {
		return ProjectionProfileCompatibilityGround{}, err
	}
	if !bytes.Equal(ground.CanonicalBytes(), canonical) {
		return ProjectionProfileCompatibilityGround{}, fmt.Errorf("profile ground is not canonical")
	}
	return ground, nil
}

func parseProfileCompatibilityGroundPosture(
	raw string,
) (ProfileCompatibilityGroundPosture, error) {
	postures := []ProfileCompatibilityGroundPosture{
		ProfileGroundSatisfied,
		ProfileGroundDegraded,
		ProfileGroundBlocking,
	}
	for _, posture := range postures {
		if posture.String() == raw {
			return posture, nil
		}
	}
	return 0, fmt.Errorf("unsupported profile compatibility ground posture %q", raw)
}

func digestBytes(canonical []byte) (typedmemory.SHA256Digest, error) {
	sum := sha256.Sum256(canonical)
	encoded := hex.EncodeToString(sum[:])
	return typedmemory.NewSHA256Digest("sha256:" + encoded)
}

type canonicalWriter struct {
	buffer bytes.Buffer
}

func newCanonicalWriter(domain string) *canonicalWriter {
	writer := &canonicalWriter{}
	writer.addString(domain)
	return writer
}

func (writer *canonicalWriter) addString(value string) {
	writer.addBytes([]byte(value))
}

func (writer *canonicalWriter) addBytes(value []byte) {
	length := make([]byte, 8)
	binary.BigEndian.PutUint64(length, uint64(len(value)))
	_, _ = writer.buffer.Write(length)
	_, _ = writer.buffer.Write(value)
}

func (writer *canonicalWriter) addUint64(value uint64) {
	encoded := make([]byte, 8)
	binary.BigEndian.PutUint64(encoded, value)
	writer.addBytes(encoded)
}

func (writer *canonicalWriter) bytes() []byte {
	return append([]byte(nil), writer.buffer.Bytes()...)
}

type canonicalReader struct {
	value  []byte
	offset int
}

func newCanonicalReader(value []byte) *canonicalReader {
	return &canonicalReader{value: append([]byte(nil), value...)}
}

func (reader *canonicalReader) readString(label string) (string, error) {
	value, err := reader.readBytes(label)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(value) {
		return "", fmt.Errorf("%s is not valid UTF-8", label)
	}
	return string(value), nil
}

func (reader *canonicalReader) readDigest(
	label string,
) (typedmemory.SHA256Digest, error) {
	text, err := reader.readString(label)
	if err != nil {
		return typedmemory.SHA256Digest{}, err
	}
	digest, err := typedmemory.NewSHA256Digest(text)
	if err != nil {
		return typedmemory.SHA256Digest{}, fmt.Errorf("%s: %w", label, err)
	}
	return digest, nil
}

func (reader *canonicalReader) readCount(label string, maximum int) (int, error) {
	value, err := reader.readBytes(label)
	if err != nil {
		return 0, err
	}
	if len(value) != 8 {
		return 0, fmt.Errorf("%s must be an encoded uint64", label)
	}
	count := binary.BigEndian.Uint64(value)
	if maximum < 0 {
		return 0, fmt.Errorf("%s maximum must be nonnegative", label)
	}
	maximumValue := uint64(maximum) // #nosec G115 -- maximum is nonnegative above.
	if count > maximumValue {
		return 0, fmt.Errorf("%s exceeds %d", label, maximum)
	}
	return int(count), nil // #nosec G115 -- count is bounded by the nonnegative int maximum above.
}

func (reader *canonicalReader) readBytes(label string) ([]byte, error) {
	if reader.remaining() < 8 {
		return nil, fmt.Errorf("%s length is truncated", label)
	}
	length := binary.BigEndian.Uint64(reader.value[reader.offset : reader.offset+8])
	reader.offset += 8
	if length > uint64(maximumProfileCompatibilityField) {
		return nil, fmt.Errorf("%s exceeds %d bytes", label, maximumProfileCompatibilityField)
	}
	remaining := reader.remaining()
	if remaining < 0 {
		return nil, fmt.Errorf("%s reader offset is invalid", label)
	}
	remainingValue := uint64(remaining) // #nosec G115 -- remaining is nonnegative above.
	if length > remainingValue {
		return nil, fmt.Errorf("%s bytes are truncated", label)
	}
	end := reader.offset + int(length)
	value := append([]byte(nil), reader.value[reader.offset:end]...)
	reader.offset = end
	return value, nil
}

func (reader *canonicalReader) remaining() int {
	return len(reader.value) - reader.offset
}
