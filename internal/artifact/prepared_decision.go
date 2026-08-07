package artifact

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/m0n0x41d/haft/internal/projectpath"
)

const preparedDecisionSchemaV1 = "haft.artifact.prepared-decision/v1"

var decisionReservationPattern = regexp.MustCompile(
	`^dec-[0-9]{8}(?:-[a-z0-9]+)*-[0-9a-f]{8}$`,
)

// DecisionReservation is an identity reservation only. It does not assert
// that a decision occurred and therefore contains no creation timestamp.
// Persistent occupancy and replay checks belong to the later effect shell.
type DecisionReservation struct {
	value string
}

func NewDecisionReservation(raw string) (DecisionReservation, error) {
	if !decisionReservationPattern.MatchString(raw) {
		return DecisionReservation{}, fmt.Errorf(
			"decision reservation must be an exact canonical dec-YYYYMMDD-...-<8 lowercase hex> identity",
		)
	}
	return DecisionReservation{value: raw}, nil
}

// ReserveDecisionIdentity generates a collision-resistant future
// DecisionRecord identity. It performs no project-state write.
func ReserveDecisionIdentity(taskContext string) (DecisionReservation, error) {
	identity := GenerateIDWithTaskContext(KindDecisionRecord, 0, taskContext)
	return NewDecisionReservation(identity)
}

func (reservation DecisionReservation) String() string {
	return reservation.value
}

func (reservation DecisionReservation) valid() bool {
	return decisionReservationPattern.MatchString(reservation.value)
}

type PreparedDecisionDigest struct {
	value string
}

func (digest PreparedDecisionDigest) String() string {
	return digest.value
}

func (digest PreparedDecisionDigest) valid() bool {
	return canonicalSHA256Digest(digest.value)
}

// PreparedDecision is the time-free semantic snapshot reviewed before a
// manual decision SpeechAct. It already contains normalization, filesystem
// binding enrichment, linked-artifact resolution, canonical links and
// affected files, plus the read pins needed to detect later source drift.
// It is not a persisted DecisionRecord and contains no occurrence time.
type PreparedDecision struct {
	state *preparedDecisionState
}

// PreparedDecisionReview is a read-only human-review projection of the exact
// semantic snapshot. It exposes no persistable Artifact and no timestamp
// injection seam.
type PreparedDecisionReview struct {
	state *preparedDecisionReviewState
}

type preparedDecisionReviewState struct {
	title   string
	context string
	mode    Mode
	body    string
}

type preparedDecisionState struct {
	projectRoot      string
	reservation      DecisionReservation
	proposalInput    []byte
	resolvedInput    []byte
	semanticArtifact Artifact
	links            []Link
	affectedFiles    []AffectedFile
	sourcePins       []preparedDecisionSourcePinJSONV1
	canonicalJSON    []byte
	digest           PreparedDecisionDigest
}

type preparedDecisionJSONV1 struct {
	Schema        string                            `json:"schema"`
	ProjectRoot   string                            `json:"project_root,omitempty"`
	DecisionRef   string                            `json:"decision_ref"`
	ProposalInput json.RawMessage                   `json:"proposal_input"`
	ResolvedInput json.RawMessage                   `json:"resolved_input"`
	Artifact      preparedDecisionArtifactJSONV1    `json:"artifact"`
	Links         []preparedDecisionLinkJSONV1      `json:"links"`
	AffectedFiles []preparedDecisionAffectedJSONV1  `json:"affected_files"`
	SourcePins    []preparedDecisionSourcePinJSONV1 `json:"source_pins"`
}

type preparedDecisionArtifactJSONV1 struct {
	ID             string          `json:"id"`
	Kind           string          `json:"kind"`
	Version        int             `json:"version"`
	Status         string          `json:"status"`
	Context        string          `json:"context"`
	Mode           string          `json:"mode"`
	Title          string          `json:"title"`
	ValidUntil     string          `json:"valid_until"`
	Body           string          `json:"body"`
	SearchKeywords string          `json:"search_keywords"`
	StructuredData json.RawMessage `json:"structured_data"`
}

type preparedDecisionLinkJSONV1 struct {
	Ref  string `json:"ref"`
	Type string `json:"type"`
}

type preparedDecisionAffectedJSONV1 struct {
	Path string `json:"path"`
}

type preparedDecisionSourcePinJSONV1 struct {
	Operation string                               `json:"operation"`
	Ref       string                               `json:"ref"`
	Outcome   string                               `json:"outcome"`
	Version   int                                  `json:"version,omitempty"`
	Digest    string                               `json:"digest,omitempty"`
	Members   []preparedDecisionSourceMemberJSONV1 `json:"members,omitempty"`
}

type preparedDecisionSourceMemberJSONV1 struct {
	Ref     string `json:"ref"`
	Version int    `json:"version"`
	Digest  string `json:"digest"`
}

// PrepareDecision resolves the same design-time semantics historically hidden
// inside Decide but performs no persistence and writes no Markdown projection.
func PrepareDecision(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	reservation DecisionReservation,
	proposal DecideInput,
) (PreparedDecision, error) {
	projectRoot := projectRootFromHaftDir(haftDir)
	if !validPreparedDecisionProjectRoot(projectRoot) {
		return PreparedDecision{}, fmt.Errorf(
			"decision preparation requires an absolute project .haft directory",
		)
	}
	return prepareDecision(
		ctx,
		store,
		reservation,
		proposal,
		projectRoot,
		projectRoot,
	)
}

func prepareDecision(
	ctx context.Context,
	store ArtifactStore,
	reservation DecisionReservation,
	proposal DecideInput,
	projectRoot string,
	bindingProjectRoot string,
) (PreparedDecision, error) {
	if ctx == nil {
		return PreparedDecision{}, fmt.Errorf("decision preparation requires a context")
	}
	if err := ctx.Err(); err != nil {
		return PreparedDecision{}, err
	}
	if store == nil {
		return PreparedDecision{}, fmt.Errorf("decision preparation requires an artifact store")
	}
	if !reservation.valid() {
		return PreparedDecision{}, fmt.Errorf("decision preparation requires a valid identity reservation")
	}
	normalizedProposal := normalizeDecisionInput(proposal)
	proposalBytes, err := EncodeDecideInputCanonicalJSON(normalizedProposal)
	if err != nil {
		return PreparedDecision{}, err
	}
	resolvedInput := enrichDecisionInputBindingTargets(
		bindingProjectRoot,
		normalizedProposal,
	)
	resolvedInput = normalizeDecisionInput(resolvedInput)
	affectedPaths, err := canonicalDecisionAffectedPaths(resolvedInput.AffectedFiles)
	if err != nil {
		return PreparedDecision{}, err
	}
	resolvedInput.AffectedFiles = affectedPaths
	resolvedInput = normalizeDecisionInput(resolvedInput)
	if _, err := ParseGovernanceMode(resolvedInput.GovernanceMode); err != nil {
		return PreparedDecision{}, err
	}

	readStore := newDecisionPreparationReadStore(store)
	problemRefs := MergeProblemRefs(resolvedInput.ProblemRef, resolvedInput.ProblemRefs)
	problemBasisRefs := resolveIncomingDecisionProblemRefs(
		ctx,
		readStore,
		problemRefs,
		resolvedInput.PortfolioRef,
	)
	conflictErr := validateNoActiveDecisionConflict(
		ctx,
		readStore,
		problemRefs,
		resolvedInput.PortfolioRef,
	)
	if conflictErr != nil {
		return PreparedDecision{}, conflictErr
	}
	// artifact_links has strict Artifact -> Artifact foreign keys. SpecSection
	// refs are typed cross-carrier relations carried canonically in the
	// DecisionRecord structured data and resolved by spec views; projecting
	// them into artifact_links would manufacture a dangling Artifact target.
	links, err := canonicalDecisionLinks(
		BuildLinks(problemRefs, resolvedInput.PortfolioRef),
	)
	if err != nil {
		return PreparedDecision{}, err
	}
	mode, err := resolvePreparedDecisionMode(
		ctx,
		readStore,
		problemRefs,
		resolvedInput.PortfolioRef,
		resolvedInput.Mode,
	)
	if err != nil {
		return PreparedDecision{}, err
	}
	resolvedContext := resolvePreparedDecisionContext(
		ctx,
		readStore,
		problemRefs,
		resolvedInput.PortfolioRef,
		resolvedInput.Context,
	)
	problemBody, problemStructured := resolvePreparedDecisionProblemMaterial(
		ctx,
		readStore,
		problemRefs,
		problemBasisRefs,
		resolvedInput.ProblemRef,
	)
	if err := readStore.Err(); err != nil {
		return PreparedDecision{}, err
	}
	semanticArtifact, err := BuildDecisionArtifact(DecideContext{
		ID:                reservation.String(),
		Now:               time.Time{},
		Mode:              mode,
		Context:           resolvedContext,
		ProblemBody:       problemBody,
		ProblemStructured: problemStructured,
		Links:             links,
		ProblemRefs:       problemRefs,
		ProblemBasisRefs:  problemBasisRefs,
	}, resolvedInput)
	if err != nil {
		return PreparedDecision{}, err
	}
	semanticArtifact.Meta.CreatedAt = time.Time{}
	semanticArtifact.Meta.UpdatedAt = time.Time{}
	semanticArtifact.Meta.Links = slices.Clone(links)
	resolvedBytes, err := EncodeDecideInputCanonicalJSON(resolvedInput)
	if err != nil {
		return PreparedDecision{}, err
	}
	affectedFiles := decisionAffectedFiles(resolvedInput.AffectedFiles)
	return newPreparedDecision(
		projectRoot,
		reservation,
		proposalBytes,
		resolvedBytes,
		*semanticArtifact,
		links,
		affectedFiles,
		readStore.SourcePins(),
	)
}

func newPreparedDecision(
	projectRoot string,
	reservation DecisionReservation,
	proposalInput []byte,
	resolvedInput []byte,
	semanticArtifact Artifact,
	links []Link,
	affectedFiles []AffectedFile,
	sourcePins []preparedDecisionSourcePinJSONV1,
) (PreparedDecision, error) {
	state := preparedDecisionState{
		projectRoot:      projectRoot,
		reservation:      reservation,
		proposalInput:    slices.Clone(proposalInput),
		resolvedInput:    slices.Clone(resolvedInput),
		semanticArtifact: cloneArtifactValue(semanticArtifact),
		links:            slices.Clone(links),
		affectedFiles:    slices.Clone(affectedFiles),
		sourcePins:       canonicalDecisionSourcePins(sourcePins),
	}
	projection, err := preparedDecisionProjection(state)
	if err != nil {
		return PreparedDecision{}, err
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil {
		return PreparedDecision{}, fmt.Errorf("encode prepared decision: %w", err)
	}
	state.canonicalJSON = canonicalJSON
	state.digest = digestPreparedDecision(canonicalJSON)
	prepared := PreparedDecision{state: &state}
	if !prepared.valid() {
		return PreparedDecision{}, fmt.Errorf("prepared decision is inconsistent")
	}
	return prepared, nil
}

func (prepared PreparedDecision) ProjectRoot() (string, bool) {
	if !prepared.valid() {
		return "", false
	}
	return prepared.state.projectRoot, true
}

func (prepared PreparedDecision) DecisionRef() (string, bool) {
	if !prepared.valid() {
		return "", false
	}
	return prepared.state.reservation.String(), true
}

func (prepared PreparedDecision) ResolvedInput() (DecideInput, bool) {
	if !prepared.valid() {
		return DecideInput{}, false
	}
	input, err := DecodeDecideInputCanonicalJSON(prepared.state.resolvedInput)
	return input, err == nil
}

func (prepared PreparedDecision) ReviewSnapshot() (
	PreparedDecisionReview,
	bool,
) {
	if !prepared.valid() {
		return PreparedDecisionReview{}, false
	}
	artifact := prepared.state.semanticArtifact
	state := preparedDecisionReviewState{
		title:   artifact.Meta.Title,
		context: artifact.Meta.Context,
		mode:    artifact.Meta.Mode,
		body:    artifact.Body,
	}
	return PreparedDecisionReview{state: &state}, true
}

func (review PreparedDecisionReview) Title() (string, bool) {
	if review.state == nil {
		return "", false
	}
	return review.state.title, true
}

func (review PreparedDecisionReview) Context() (string, bool) {
	if review.state == nil {
		return "", false
	}
	return review.state.context, true
}

func (review PreparedDecisionReview) Mode() (Mode, bool) {
	if review.state == nil {
		return "", false
	}
	return review.state.mode, true
}

func (review PreparedDecisionReview) Body() (string, bool) {
	if review.state == nil {
		return "", false
	}
	return review.state.body, true
}

func (prepared PreparedDecision) Links() ([]Link, bool) {
	if !prepared.valid() {
		return nil, false
	}
	return slices.Clone(prepared.state.links), true
}

func (prepared PreparedDecision) AffectedFiles() ([]AffectedFile, bool) {
	if !prepared.valid() {
		return nil, false
	}
	return slices.Clone(prepared.state.affectedFiles), true
}

func (prepared PreparedDecision) CanonicalBytes() ([]byte, bool) {
	if !prepared.valid() {
		return nil, false
	}
	return slices.Clone(prepared.state.canonicalJSON), true
}

func (prepared PreparedDecision) Digest() (PreparedDecisionDigest, bool) {
	if !prepared.valid() {
		return PreparedDecisionDigest{}, false
	}
	return prepared.state.digest, true
}

// artifactAt derives DecisionRecord occurrence time from the supplied verified
// SpeechAct occurrence. The prepared snapshot itself never carries this time.
func (prepared PreparedDecision) artifactAt(
	verifiedSpeechActOccurredAt time.Time,
) (*Artifact, error) {
	if !prepared.valid() {
		return nil, fmt.Errorf("prepared decision is invalid")
	}
	occurredAt := verifiedSpeechActOccurredAt.UTC().Round(0)
	if occurredAt.IsZero() {
		return nil, fmt.Errorf("verified decision SpeechAct occurrence time is required")
	}
	artifact := cloneArtifactValue(prepared.state.semanticArtifact)
	artifact.Meta.CreatedAt = occurredAt
	artifact.Meta.UpdatedAt = occurredAt
	return &artifact, nil
}

// RevalidatePreparedDecision reruns proposal resolution against current read
// sources. Any changed source pin or semantic snapshot rejects the stale
// preparation before a later effect transaction may use it.
func RevalidatePreparedDecision(
	ctx context.Context,
	store ArtifactStore,
	haftDir string,
	prepared PreparedDecision,
) error {
	if !prepared.valid() {
		return fmt.Errorf("prepared decision is invalid")
	}
	proposal, err := DecodeDecideInputCanonicalJSON(prepared.state.proposalInput)
	if err != nil {
		return err
	}
	rebuilt, err := PrepareDecision(
		ctx,
		store,
		haftDir,
		prepared.state.reservation,
		proposal,
	)
	if err != nil {
		return err
	}
	want, _ := prepared.Digest()
	got, _ := rebuilt.Digest()
	if want.String() != got.String() {
		return fmt.Errorf("prepared decision is stale against current project sources")
	}
	return nil
}

func (prepared PreparedDecision) valid() bool {
	if prepared.state == nil {
		return false
	}
	state := *prepared.state
	if !state.reservation.valid() || !validPreparedDecisionProjectRoot(state.projectRoot) {
		return false
	}
	if _, err := DecodeDecideInputCanonicalJSON(state.proposalInput); err != nil {
		return false
	}
	if _, err := DecodeDecideInputCanonicalJSON(state.resolvedInput); err != nil {
		return false
	}
	if !state.semanticArtifact.Meta.CreatedAt.IsZero() ||
		!state.semanticArtifact.Meta.UpdatedAt.IsZero() {
		return false
	}
	if state.semanticArtifact.Meta.ID != state.reservation.String() ||
		state.semanticArtifact.Meta.Kind != KindDecisionRecord ||
		state.semanticArtifact.Meta.Version != 1 ||
		state.semanticArtifact.Meta.Status != StatusActive {
		return false
	}
	links, err := canonicalDecisionLinks(state.links)
	if err != nil || !reflect.DeepEqual(links, state.links) {
		return false
	}
	affected := canonicalDecisionAffectedFiles(state.affectedFiles)
	if !reflect.DeepEqual(affected, state.affectedFiles) {
		return false
	}
	if !reflect.DeepEqual(state.semanticArtifact.Meta.Links, state.links) {
		return false
	}
	if !reflect.DeepEqual(
		canonicalDecisionSourcePins(state.sourcePins),
		state.sourcePins,
	) {
		return false
	}
	projection, err := preparedDecisionProjection(state)
	if err != nil {
		return false
	}
	canonicalJSON, err := json.Marshal(projection)
	if err != nil || !slices.Equal(canonicalJSON, state.canonicalJSON) {
		return false
	}
	digest := digestPreparedDecision(canonicalJSON)
	return digest.valid() && digest.String() == state.digest.String()
}

func preparedDecisionProjection(
	state preparedDecisionState,
) (preparedDecisionJSONV1, error) {
	structured := []byte(state.semanticArtifact.StructuredData)
	if !json.Valid(structured) {
		return preparedDecisionJSONV1{}, fmt.Errorf("prepared decision structured data is not canonical JSON")
	}
	artifactProjection := preparedDecisionArtifactJSONV1{
		ID:             state.semanticArtifact.Meta.ID,
		Kind:           string(state.semanticArtifact.Meta.Kind),
		Version:        state.semanticArtifact.Meta.Version,
		Status:         string(state.semanticArtifact.Meta.Status),
		Context:        state.semanticArtifact.Meta.Context,
		Mode:           string(state.semanticArtifact.Meta.Mode),
		Title:          state.semanticArtifact.Meta.Title,
		ValidUntil:     state.semanticArtifact.Meta.ValidUntil,
		Body:           state.semanticArtifact.Body,
		SearchKeywords: state.semanticArtifact.SearchKeywords,
		StructuredData: slices.Clone(structured),
	}
	linkProjection := make([]preparedDecisionLinkJSONV1, 0, len(state.links))
	for _, link := range state.links {
		linkProjection = append(linkProjection, preparedDecisionLinkJSONV1(link))
	}
	affectedProjection := make([]preparedDecisionAffectedJSONV1, 0, len(state.affectedFiles))
	for _, file := range state.affectedFiles {
		affectedProjection = append(affectedProjection, preparedDecisionAffectedJSONV1{
			Path: file.Path,
		})
	}
	return preparedDecisionJSONV1{
		Schema:        preparedDecisionSchemaV1,
		ProjectRoot:   state.projectRoot,
		DecisionRef:   state.reservation.String(),
		ProposalInput: slices.Clone(state.proposalInput),
		ResolvedInput: slices.Clone(state.resolvedInput),
		Artifact:      artifactProjection,
		Links:         linkProjection,
		AffectedFiles: affectedProjection,
		SourcePins:    cloneDecisionSourcePins(state.sourcePins),
	}, nil
}

func resolvePreparedDecisionMode(
	ctx context.Context,
	store ArtifactStore,
	problemRefs []string,
	portfolioRef string,
	declared string,
) (Mode, error) {
	declaredMode := ModeStandard
	if declared != "" {
		parsed, err := ParseMode(declared)
		if err != nil {
			return "", fmt.Errorf("%w (valid: note, tactical, standard, deep)", err)
		}
		declaredMode = parsed
	}
	chainMode := inferModeFromChain(ctx, store, problemRefs, portfolioRef)
	return maxMode(declaredMode, chainMode), nil
}

func resolvePreparedDecisionContext(
	ctx context.Context,
	store ArtifactStore,
	problemRefs []string,
	portfolioRef string,
	explicit string,
) string {
	if explicit != "" {
		return explicit
	}
	if portfolioRef != "" {
		portfolio, err := store.Get(ctx, portfolioRef)
		if err == nil {
			return portfolio.Meta.Context
		}
		return ""
	}
	if len(problemRefs) == 0 {
		return ""
	}
	problem, err := store.Get(ctx, problemRefs[0])
	if err != nil {
		return ""
	}
	return problem.Meta.Context
}

func resolvePreparedDecisionProblemMaterial(
	ctx context.Context,
	store ArtifactStore,
	problemRefs []string,
	problemBasisRefs []string,
	primaryProblemRef string,
) (string, string) {
	primaryRef := primaryProblemRef
	if primaryRef == "" && len(problemRefs) > 0 {
		primaryRef = problemRefs[0]
	}
	if primaryRef == "" && len(problemBasisRefs) > 0 {
		primaryRef = problemBasisRefs[0]
	}
	if primaryRef == "" {
		return "", ""
	}
	problem, err := store.Get(ctx, primaryRef)
	if err != nil {
		return "", ""
	}
	return problem.Body, problem.StructuredData
}

func canonicalDecisionLinks(values []Link) ([]Link, error) {
	result := make([]Link, 0, len(values))
	for _, value := range values {
		ref := strings.TrimSpace(value.Ref)
		linkType := strings.TrimSpace(value.Type)
		if ref == "" || linkType == "" || containsUnsupportedControl(ref) ||
			containsUnsupportedControl(linkType) {
			return nil, fmt.Errorf("decision link requires canonical ref and type")
		}
		result = append(result, Link{Ref: ref, Type: linkType})
	}
	slices.SortFunc(result, func(left Link, right Link) int {
		if left.Type != right.Type {
			return strings.Compare(left.Type, right.Type)
		}
		return strings.Compare(left.Ref, right.Ref)
	})
	result = slices.CompactFunc(result, func(left Link, right Link) bool {
		return left.Ref == right.Ref && left.Type == right.Type
	})
	return result, nil
}

func canonicalDecisionAffectedPaths(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		canonical, err := canonicalDecisionAffectedPath(value)
		if err != nil {
			return nil, err
		}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return slices.Compact(result), nil
}

func canonicalDecisionAffectedPath(value string) (string, error) {
	canonical, err := projectpath.Parse(value)
	if err != nil {
		return "", fmt.Errorf(
			"decision affected file %q must be a canonical project-relative path",
			value,
		)
	}
	return canonical.String(), nil
}

func decisionAffectedFiles(paths []string) []AffectedFile {
	result := make([]AffectedFile, 0, len(paths))
	for _, filePath := range paths {
		result = append(result, AffectedFile{Path: filePath})
	}
	return result
}

func canonicalDecisionAffectedFiles(values []AffectedFile) []AffectedFile {
	paths := make([]string, 0, len(values))
	for _, value := range values {
		if value.Hash != "" {
			return nil
		}
		paths = append(paths, value.Path)
	}
	canonical, err := canonicalDecisionAffectedPaths(paths)
	if err != nil {
		return nil
	}
	return decisionAffectedFiles(canonical)
}

func validPreparedDecisionProjectRoot(value string) bool {
	return value != "" && filepath.IsAbs(value) && filepath.Clean(value) == value &&
		!containsUnsupportedControl(value)
}

func containsUnsupportedControl(value string) bool {
	return strings.ContainsFunc(value, unicode.IsControl)
}

func cloneArtifactValue(value Artifact) Artifact {
	clone := value
	clone.Meta.Links = slices.Clone(value.Meta.Links)
	return clone
}

func digestPreparedDecision(value []byte) PreparedDecisionDigest {
	hash := sha256.Sum256(value)
	return PreparedDecisionDigest{value: "sha256:" + hex.EncodeToString(hash[:])}
}

func canonicalSHA256Digest(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != 71 {
		return false
	}
	_, err := hex.DecodeString(strings.TrimPrefix(value, "sha256:"))
	return err == nil && strings.ToLower(value) == value
}

type decisionPreparationReadStore struct {
	ArtifactStore
	pins map[string]preparedDecisionSourcePinJSONV1
	err  error
}

func newDecisionPreparationReadStore(store ArtifactStore) *decisionPreparationReadStore {
	return &decisionPreparationReadStore{
		ArtifactStore: store,
		pins:          map[string]preparedDecisionSourcePinJSONV1{},
	}
}

func (store *decisionPreparationReadStore) Get(
	ctx context.Context,
	id string,
) (*Artifact, error) {
	artifact, err := store.ArtifactStore.Get(ctx, id)
	if err != nil {
		store.recordPin(preparedDecisionSourcePinJSONV1{
			Operation: "get",
			Ref:       id,
			Outcome:   "unavailable",
		})
		return nil, err
	}
	clone := cloneArtifactValue(*artifact)
	digest, digestErr := digestSourceArtifact(clone)
	if digestErr != nil {
		store.err = digestErr
		return nil, digestErr
	}
	store.recordPin(preparedDecisionSourcePinJSONV1{
		Operation: "get",
		Ref:       id,
		Outcome:   "found",
		Version:   clone.Meta.Version,
		Digest:    digest,
	})
	return &clone, nil
}

func (store *decisionPreparationReadStore) ListByKind(
	ctx context.Context,
	kind Kind,
	limit int,
) ([]*Artifact, error) {
	items, err := store.ArtifactStore.ListByKind(ctx, kind, limit)
	if err != nil {
		return nil, err
	}
	clones := make([]*Artifact, 0, len(items))
	members := make([]preparedDecisionSourceMemberJSONV1, 0, len(items))
	for _, item := range items {
		clone := cloneArtifactValue(*item)
		digest, digestErr := digestSourceArtifact(clone)
		if digestErr != nil {
			store.err = digestErr
			return nil, digestErr
		}
		clones = append(clones, &clone)
		members = append(members, preparedDecisionSourceMemberJSONV1{
			Ref:     clone.Meta.ID,
			Version: clone.Meta.Version,
			Digest:  digest,
		})
	}
	slices.SortFunc(members, func(left preparedDecisionSourceMemberJSONV1, right preparedDecisionSourceMemberJSONV1) int {
		return strings.Compare(left.Ref, right.Ref)
	})
	membersJSON, err := json.Marshal(members)
	if err != nil {
		return nil, err
	}
	setDigest := digestPreparedDecision(membersJSON).String()
	store.recordPin(preparedDecisionSourcePinJSONV1{
		Operation: "list_by_kind",
		Ref:       fmt.Sprintf("kind:%s;limit:%d", kind, limit),
		Outcome:   "observed",
		Digest:    setDigest,
		Members:   members,
	})
	return clones, nil
}

func (store *decisionPreparationReadStore) recordPin(
	pin preparedDecisionSourcePinJSONV1,
) {
	key := pin.Operation + "\x00" + pin.Ref
	previous, exists := store.pins[key]
	if exists && !reflect.DeepEqual(previous, pin) {
		store.err = fmt.Errorf("decision source %s changed during preparation", pin.Ref)
		return
	}
	store.pins[key] = pin
}

func (store *decisionPreparationReadStore) Err() error {
	return store.err
}

func (store *decisionPreparationReadStore) SourcePins() []preparedDecisionSourcePinJSONV1 {
	result := make([]preparedDecisionSourcePinJSONV1, 0, len(store.pins))
	for _, pin := range store.pins {
		result = append(result, cloneDecisionSourcePin(pin))
	}
	slices.SortFunc(result, func(left preparedDecisionSourcePinJSONV1, right preparedDecisionSourcePinJSONV1) int {
		if left.Operation != right.Operation {
			return strings.Compare(left.Operation, right.Operation)
		}
		return strings.Compare(left.Ref, right.Ref)
	})
	return result
}

func digestSourceArtifact(value Artifact) (string, error) {
	links, err := canonicalDecisionLinks(value.Meta.Links)
	if err != nil {
		return "", err
	}
	value.Meta.Links = links
	canonical, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("encode decision source pin: %w", err)
	}
	return digestPreparedDecision(canonical).String(), nil
}

func cloneDecisionSourcePins(
	values []preparedDecisionSourcePinJSONV1,
) []preparedDecisionSourcePinJSONV1 {
	result := make([]preparedDecisionSourcePinJSONV1, 0, len(values))
	for _, value := range values {
		result = append(result, cloneDecisionSourcePin(value))
	}
	return result
}

func canonicalDecisionSourcePins(
	values []preparedDecisionSourcePinJSONV1,
) []preparedDecisionSourcePinJSONV1 {
	result := cloneDecisionSourcePins(values)
	for index := range result {
		slices.SortFunc(
			result[index].Members,
			func(
				left preparedDecisionSourceMemberJSONV1,
				right preparedDecisionSourceMemberJSONV1,
			) int {
				return strings.Compare(left.Ref, right.Ref)
			},
		)
	}
	slices.SortFunc(
		result,
		func(
			left preparedDecisionSourcePinJSONV1,
			right preparedDecisionSourcePinJSONV1,
		) int {
			if left.Operation != right.Operation {
				return strings.Compare(left.Operation, right.Operation)
			}
			return strings.Compare(left.Ref, right.Ref)
		},
	)
	return slices.CompactFunc(
		result,
		func(
			left preparedDecisionSourcePinJSONV1,
			right preparedDecisionSourcePinJSONV1,
		) bool {
			return reflect.DeepEqual(left, right)
		},
	)
}

func cloneDecisionSourcePin(
	value preparedDecisionSourcePinJSONV1,
) preparedDecisionSourcePinJSONV1 {
	clone := value
	clone.Members = slices.Clone(value.Members)
	return clone
}
