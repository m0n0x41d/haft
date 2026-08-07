package legacyimporteffect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/projectidentity"
	"github.com/m0n0x41d/haft/internal/projectmemory/legacyimport"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectioneffect"
	"github.com/m0n0x41d/haft/internal/typedmemory"
)

var _ ImportApplyStore = (*memoryImportApplyStore)(nil)
var _ ImportApplyTransaction = (*memoryImportApplyTransaction)(nil)

func TestApplyRequiresSelectedProjectTypeEnvBeforeOpaqueHistoryWrite(
	t *testing.T,
) {
	request := mustImportApplyRequest(t, "requires-head", "import-run:requires-head")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	store.selectionErr = errors.New("no ProjectTypeEnvHead exists")

	result, err := NewApplyService().Apply(context.Background(), store, request)

	if !errors.Is(err, ErrSelectedProjectTypeEnvUnavailable) {
		t.Fatalf("Apply() error = %v, want ErrSelectedProjectTypeEnvUnavailable", err)
	}
	if result != nil {
		t.Fatalf("Apply() result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 1, 1, 0, 0)
	if strings.Join(store.events, ",") != "probe,resolve,rollback" {
		t.Fatalf("effect order = %v, want probe, resolve, rollback", store.events)
	}
}

func TestApplyPersistsOneAtomicOpaqueBatchAndReplaysWithoutCurrentHead(
	t *testing.T,
) {
	request := mustImportApplyRequest(t, "apply-replay", "import-run:apply-replay")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())

	first, err := NewApplyService().Apply(context.Background(), store, request)
	if err != nil {
		t.Fatalf("first Apply() error = %v", err)
	}
	applied, exists := first.(ImportApplied)
	if !exists {
		t.Fatalf("first Apply() result = %T, want ImportApplied", first)
	}
	if applied.Receipt().OpaqueCarrierCount() != 1 {
		t.Fatalf(
			"OpaqueCarrierCount() = %d, want 1",
			applied.Receipt().OpaqueCarrierCount(),
		)
	}
	if applied.Receipt().SubjectDispositionCount() != 1 {
		t.Fatalf(
			"SubjectDispositionCount() = %d, want 1",
			applied.Receipt().SubjectDispositionCount(),
		)
	}
	assertImportStoreCounters(t, store, 1, 1, 1, 1)
	if strings.Join(store.events, ",") != "probe,resolve,append,commit" {
		t.Fatalf(
			"first effect order = %v, want probe, resolve, append, commit",
			store.events,
		)
	}

	store.events = nil
	store.selectionErr = errors.New("current head must not be read during exact replay")
	second, err := NewApplyService().Apply(context.Background(), store, request)
	if err != nil {
		t.Fatalf("replay Apply() error = %v", err)
	}
	replayed, exists := second.(ImportReplayed)
	if !exists {
		t.Fatalf("replay Apply() result = %T, want ImportReplayed", second)
	}
	if !bytes.Equal(
		applied.Receipt().CanonicalBytes(),
		replayed.Receipt().CanonicalBytes(),
	) {
		t.Fatal("exact replay returned a different receipt")
	}
	assertImportStoreCounters(t, store, 2, 1, 1, 2)
	if strings.Join(store.events, ",") != "probe,commit" {
		t.Fatalf("replay effect order = %v, want probe, commit", store.events)
	}
}

func TestApplyRejectsIdempotencyConflictBeforeHeadResolutionOrWrite(
	t *testing.T,
) {
	firstRequest := mustImportApplyRequest(t, "conflict-a", "import-run:conflict")
	secondRequest := mustImportApplyRequest(t, "conflict-b", "import-run:conflict")
	store := newMemoryImportApplyStore(t, firstRequest.Plan().ProjectID())
	if _, err := NewApplyService().Apply(
		context.Background(),
		store,
		firstRequest,
	); err != nil {
		t.Fatalf("seed Apply() error = %v", err)
	}
	store.events = nil

	result, err := NewApplyService().Apply(
		context.Background(),
		store,
		secondRequest,
	)

	if !errors.Is(err, ErrImportReplayConflict) {
		t.Fatalf("conflicting Apply() error = %v, want ErrImportReplayConflict", err)
	}
	if result != nil {
		t.Fatalf("conflicting Apply() result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 2, 1, 1, 1)
	if strings.Join(store.events, ",") != "probe,rollback" {
		t.Fatalf("conflict effect order = %v, want probe, rollback", store.events)
	}
}

func TestApplyRollsBackFailedAppendAndRetryUsesSameReceipt(t *testing.T) {
	request := mustImportApplyRequest(t, "append-fault", "import-run:append-fault")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	store.appendErr = errors.New("injected append fault")

	failed, err := NewApplyService().Apply(context.Background(), store, request)
	if !errors.Is(err, ErrOpaqueHistoryWrite) {
		t.Fatalf("faulted Apply() error = %v, want ErrOpaqueHistoryWrite", err)
	}
	if failed != nil {
		t.Fatalf("faulted Apply() result = %#v, want nil", failed)
	}
	assertImportStoreCounters(t, store, 1, 1, 1, 0)
	if len(store.committed) != 0 {
		t.Fatal("faulted append left a committed import")
	}

	store.events = nil
	store.appendErr = nil
	retried, err := NewApplyService().Apply(context.Background(), store, request)
	if err != nil {
		t.Fatalf("retry Apply() error = %v", err)
	}
	if _, exists := retried.(ImportApplied); !exists {
		t.Fatalf("retry Apply() result = %T, want ImportApplied", retried)
	}
	assertImportStoreCounters(t, store, 2, 2, 2, 1)
	if strings.Join(store.events, ",") != "probe,resolve,append,commit" {
		t.Fatalf("retry effect order = %v", store.events)
	}
}

func TestApplyReturnsCancelledWithoutStartingOrCommittingTransaction(
	t *testing.T,
) {
	request := mustImportApplyRequest(t, "pre-cancelled", "import-run:pre-cancelled")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := NewApplyService().Apply(ctx, store, request)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply() error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("Apply() result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 0, 0, 0, 0)
	if len(store.committed) != 0 {
		t.Fatal("pre-cancelled transaction left a committed import")
	}
	if strings.Join(store.events, ",") != "rollback" {
		t.Fatalf("effect order = %v, want rollback", store.events)
	}
}

func TestApplyRollsBackCancellationAtCommitBoundary(t *testing.T) {
	request := mustImportApplyRequest(t, "commit-cancel", "import-run:commit-cancel")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	ctx, cancel := context.WithCancel(context.Background())
	store.beforeCommit = cancel

	result, err := NewApplyService().Apply(ctx, store, request)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Apply() error = %v, want context.Canceled", err)
	}
	if result != nil {
		t.Fatalf("Apply() result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 1, 1, 1, 0)
	if len(store.committed) != 0 {
		t.Fatal("commit-boundary cancellation left a committed import")
	}
	if strings.Join(store.events, ",") != "probe,resolve,append,rollback" {
		t.Fatalf("effect order = %v, want probe, resolve, append, rollback", store.events)
	}
}

func TestApplyRollsBackCommitFailureAndRetry(t *testing.T) {
	request := mustImportApplyRequest(t, "commit-fault", "import-run:commit-fault")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	commitFault := errors.New("injected commit fault")
	store.commitErr = commitFault

	result, err := NewApplyService().Apply(context.Background(), store, request)

	if !errors.Is(err, commitFault) {
		t.Fatalf("Apply() error = %v, want injected commit fault", err)
	}
	if result != nil {
		t.Fatalf("Apply() result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 1, 1, 1, 0)
	if len(store.committed) != 0 {
		t.Fatal("commit failure left a committed import")
	}
	if strings.Join(store.events, ",") != "probe,resolve,append,rollback" {
		t.Fatalf("effect order = %v, want probe, resolve, append, rollback", store.events)
	}

	store.events = nil
	store.commitErr = nil
	retried, err := NewApplyService().Apply(context.Background(), store, request)
	if err != nil {
		t.Fatalf("retry Apply() error = %v", err)
	}
	if _, exists := retried.(ImportApplied); !exists {
		t.Fatalf("retry Apply() result = %T, want ImportApplied", retried)
	}
	assertImportStoreCounters(t, store, 2, 2, 2, 1)
	if strings.Join(store.events, ",") != "probe,resolve,append,commit" {
		t.Fatalf("retry effect order = %v", store.events)
	}
}

func TestApplyRejectsCrossProjectSelectedBasisWithoutWrite(t *testing.T) {
	request := mustImportApplyRequest(t, "cross-project", "import-run:cross-project")
	otherProject, err := projectidentity.ParseProjectID("qnt_deadbeef")
	if err != nil {
		t.Fatalf("ParseProjectID(other) error = %v", err)
	}
	store := newMemoryImportApplyStore(t, otherProject)

	result, err := NewApplyService().Apply(context.Background(), store, request)

	if !errors.Is(err, ErrSelectedProjectTypeEnvUnavailable) {
		t.Fatalf("Apply() error = %v, want ErrSelectedProjectTypeEnvUnavailable", err)
	}
	if result != nil {
		t.Fatalf("Apply() result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 1, 1, 0, 0)
}

func TestApplyRejectsForgedSelectedProjectTypeEnvCoordinates(t *testing.T) {
	request := mustImportApplyRequest(t, "forged-basis", "import-run:forged-basis")
	otherProject, err := projectidentity.ParseProjectID("qnt_deadbeef")
	if err != nil {
		t.Fatalf("ParseProjectID(other) error = %v", err)
	}
	otherHeadRef, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(
		otherProject,
	)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject(other) error = %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*SelectedProjectTypeEnvBasis)
	}{
		{
			name: "head ref",
			mutate: func(basis *SelectedProjectTypeEnvBasis) {
				basis.headRef = otherHeadRef
			},
		},
		{
			name: "head revision",
			mutate: func(basis *SelectedProjectTypeEnvBasis) {
				basis.headRevision = projecttypeenvselection.HeadRevision{}
			},
		},
		{
			name: "TypeEnv ref",
			mutate: func(basis *SelectedProjectTypeEnvBasis) {
				basis.typeEnvRef = typedmemory.TypeEnvRef{}
			},
		},
		{
			name: "graph revision",
			mutate: func(basis *SelectedProjectTypeEnvBasis) {
				basis.graphRevision = typedmemory.NewGraphRevision(0)
			},
		},
		{
			name: "selection receipt",
			mutate: func(basis *SelectedProjectTypeEnvBasis) {
				basis.selectionReceiptRef = projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionReceiptRef{}
			},
		},
		{
			name: "selection closure",
			mutate: func(basis *SelectedProjectTypeEnvBasis) {
				basis.selectionClosureRef = projecttypeenvselectioneffect.ProjectTypeEnvHeadSelectionClosureRef{}
			},
		},
	}
	for _, current := range cases {
		t.Run(current.name, func(t *testing.T) {
			store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
			current.mutate(&store.selected)

			result, err := NewApplyService().Apply(
				context.Background(),
				store,
				request,
			)

			if !errors.Is(err, ErrSelectedProjectTypeEnvUnavailable) {
				t.Fatalf(
					"Apply() error = %v, want ErrSelectedProjectTypeEnvUnavailable",
					err,
				)
			}
			if result != nil {
				t.Fatalf("Apply() result = %#v, want nil", result)
			}
			assertImportStoreCounters(t, store, 1, 1, 0, 0)
		})
	}
}

func TestApplyRejectsCorruptExactReplayWithoutCurrentHeadRead(t *testing.T) {
	request := mustImportApplyRequest(t, "corrupt-replay", "import-run:corrupt-replay")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	_, err := NewApplyService().Apply(context.Background(), store, request)
	if err != nil {
		t.Fatalf("seed Apply() error = %v", err)
	}
	key := storeKey(coordinateOf(request))
	stored := store.committed[key]
	stored.receiptBytes[0] ^= 0xff
	store.committed[key] = stored
	store.events = nil
	store.selectionErr = errors.New("must not resolve current head")

	result, err := NewApplyService().Apply(context.Background(), store, request)

	if !errors.Is(err, ErrImportReplayCorrupt) {
		t.Fatalf("corrupt replay error = %v, want ErrImportReplayCorrupt", err)
	}
	if result != nil {
		t.Fatalf("corrupt replay result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 2, 1, 1, 1)
	if strings.Join(store.events, ",") != "probe,rollback" {
		t.Fatalf("corrupt replay effect order = %v, want probe, rollback", store.events)
	}
}

func TestApplyRejectsCanonicallyAlteredRehydratedReplay(t *testing.T) {
	request := mustImportApplyRequest(t, "altered-replay", "import-run:altered-replay")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	if _, err := NewApplyService().Apply(
		context.Background(),
		store,
		request,
	); err != nil {
		t.Fatalf("seed Apply() error = %v", err)
	}
	key := storeKey(coordinateOf(request))
	stored := store.committed[key]
	var body importReceiptBodyDTO
	if err := json.Unmarshal(stored.receiptBytes, &body); err != nil {
		t.Fatalf("decode stored receipt fixture: %v", err)
	}
	body.SelectionClosureRef = "project-typeenv-head-selection-closure:sha256:" +
		strings.Repeat("d", 64)
	altered, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode altered receipt fixture: %v", err)
	}
	if _, err := DecodeImportReceipt(altered); err != nil {
		t.Fatalf("altered receipt is not independently canonical: %v", err)
	}
	stored.receiptBytes = altered
	store.committed[key] = stored
	store.events = nil
	store.selectionErr = errors.New("must not resolve current head")

	result, err := NewApplyService().Apply(context.Background(), store, request)

	if !errors.Is(err, ErrImportReplayCorrupt) {
		t.Fatalf("altered replay error = %v, want ErrImportReplayCorrupt", err)
	}
	if result != nil {
		t.Fatalf("altered replay result = %#v, want nil", result)
	}
	assertImportStoreCounters(t, store, 2, 1, 1, 1)
	if strings.Join(store.events, ",") != "probe,rollback" {
		t.Fatalf("altered replay effect order = %v, want probe, rollback", store.events)
	}
}

func TestImportReceiptStrictRoundTripAndReference(t *testing.T) {
	request := mustImportApplyRequest(t, "receipt-codec", "import-run:receipt-codec")
	store := newMemoryImportApplyStore(t, request.Plan().ProjectID())
	result, err := NewApplyService().Apply(context.Background(), store, request)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	receipt := result.Receipt()

	decoded, err := DecodeImportReceipt(receipt.CanonicalBytes())
	if err != nil {
		t.Fatalf("DecodeImportReceipt() error = %v", err)
	}
	if decoded.Ref() != receipt.Ref() {
		t.Fatalf(
			"decoded receipt ref = %s, want %s",
			decoded.Ref().String(),
			receipt.Ref().String(),
		)
	}
	if !bytes.Equal(decoded.CanonicalBytes(), receipt.CanonicalBytes()) {
		t.Fatal("decoded receipt changed canonical bytes")
	}
	parsedRef, err := ParseImportReceiptRef(receipt.Ref().String())
	if err != nil {
		t.Fatalf("ParseImportReceiptRef() error = %v", err)
	}
	if parsedRef != receipt.Ref() {
		t.Fatalf(
			"parsed receipt ref = %s, want %s",
			parsedRef.String(),
			receipt.Ref().String(),
		)
	}

	withWhitespace := append(receipt.CanonicalBytes(), ' ')
	if _, err := DecodeImportReceipt(withWhitespace); err == nil {
		t.Fatal("DecodeImportReceipt() accepted non-canonical trailing whitespace")
	}
	var unknown map[string]any
	if err := json.Unmarshal(receipt.CanonicalBytes(), &unknown); err != nil {
		t.Fatalf("decode receipt fixture: %v", err)
	}
	unknown["fabricated_member_of"] = true
	unknownBytes, err := json.Marshal(unknown)
	if err != nil {
		t.Fatalf("encode receipt fixture: %v", err)
	}
	if _, err := DecodeImportReceipt(unknownBytes); err == nil {
		t.Fatal("DecodeImportReceipt() accepted an unknown semantic field")
	}
}

func mustImportApplyRequest(
	t *testing.T,
	suffix string,
	keyRaw string,
) ImportApplyRequest {
	t.Helper()
	carrier := mustLegacyCarrier(t, suffix)
	subject := mustLegacySubject(t, "legacy-subject:"+suffix)
	observation := mustLegacyCarrierObservation(t, subject, carrier)
	classification, err := legacyimport.NewCarrierOnly(
		subject,
		[]legacyimport.CarrierObservation{observation},
	)
	if err != nil {
		t.Fatalf("NewCarrierOnly() error = %v", err)
	}
	report := mustLegacyDryRunReport(
		t,
		[]legacyimport.CarrierSnapshot{carrier},
		[]legacyimport.SubjectObservation{observation},
		[]legacyimport.SubjectClassification{classification},
	)
	plan, err := legacyimport.NewImportPlan(report)
	if err != nil {
		t.Fatalf("NewImportPlan() error = %v", err)
	}
	key, err := NewImportIdempotencyKey(keyRaw)
	if err != nil {
		t.Fatalf("NewImportIdempotencyKey() error = %v", err)
	}
	request, err := NewImportApplyRequest(plan, key)
	if err != nil {
		t.Fatalf("NewImportApplyRequest() error = %v", err)
	}
	return request
}

func mustLegacyCarrier(
	t *testing.T,
	suffix string,
) legacyimport.CarrierSnapshot {
	t.Helper()
	coordinate, err := legacyimport.NewSourceCoordinate("source:" + suffix)
	if err != nil {
		t.Fatalf("NewSourceCoordinate() error = %v", err)
	}
	ref, err := typedmemory.NewCarrierRef("carrier:" + suffix)
	if err != nil {
		t.Fatalf("NewCarrierRef() error = %v", err)
	}
	edition, err := typedmemory.NewCarrierEdition("edition:1")
	if err != nil {
		t.Fatalf("NewCarrierEdition() error = %v", err)
	}
	format, err := legacyimport.NewCarrierFormat("application/octet-stream")
	if err != nil {
		t.Fatalf("NewCarrierFormat() error = %v", err)
	}
	legacyRef, err := legacyimport.NewLegacyIdentityRef("legacy:" + suffix)
	if err != nil {
		t.Fatalf("NewLegacyIdentityRef() error = %v", err)
	}
	identity, err := legacyimport.NewIdentifiedLegacyCarrier(legacyRef)
	if err != nil {
		t.Fatalf("NewIdentifiedLegacyCarrier() error = %v", err)
	}
	carrier, err := legacyimport.NewCarrierSnapshot(
		coordinate,
		ref,
		edition,
		format,
		[]byte("carrier:"+suffix),
		identity,
	)
	if err != nil {
		t.Fatalf("NewCarrierSnapshot() error = %v", err)
	}
	return carrier
}

func mustLegacySubject(
	t *testing.T,
	raw string,
) legacyimport.SemanticSubjectRef {
	t.Helper()
	subject, err := legacyimport.NewSemanticSubjectRef(raw)
	if err != nil {
		t.Fatalf("NewSemanticSubjectRef() error = %v", err)
	}
	return subject
}

func mustLegacyCarrierObservation(
	t *testing.T,
	subject legacyimport.SemanticSubjectRef,
	carrier legacyimport.CarrierSnapshot,
) legacyimport.CarrierObservation {
	t.Helper()
	observation, err := legacyimport.NewCarrierObservation(subject, carrier)
	if err != nil {
		t.Fatalf("NewCarrierObservation() error = %v", err)
	}
	return observation
}

func mustLegacyDryRunReport(
	t *testing.T,
	carriers []legacyimport.CarrierSnapshot,
	observations []legacyimport.SubjectObservation,
	classifications []legacyimport.SubjectClassification,
) legacyimport.DryRunReport {
	t.Helper()
	catalog, err := legacyimport.NewCarrierCatalog(carriers)
	if err != nil {
		t.Fatalf("NewCarrierCatalog() error = %v", err)
	}
	observationSet, err := legacyimport.NewObservationSet(observations)
	if err != nil {
		t.Fatalf("NewObservationSet() error = %v", err)
	}
	source, err := legacyimport.NewLegacySourceSnapshot(catalog, observationSet)
	if err != nil {
		t.Fatalf("NewLegacySourceSnapshot() error = %v", err)
	}
	project, err := projectidentity.ParseProjectID("qnt_e3149c17")
	if err != nil {
		t.Fatalf("ParseProjectID() error = %v", err)
	}
	classifier, err := legacyimport.NewClassifierVersion(
		"legacy-import-classifier.v1",
	)
	if err != nil {
		t.Fatalf("NewClassifierVersion() error = %v", err)
	}
	report, err := legacyimport.NewDryRunReport(
		project,
		classifier,
		source,
		classifications,
	)
	if err != nil {
		t.Fatalf("NewDryRunReport() error = %v", err)
	}
	return report
}

func mustSelectedProjectTypeEnvBasis(
	t *testing.T,
	project projectidentity.ProjectID,
) SelectedProjectTypeEnvBasis {
	t.Helper()
	digest, err := typedmemory.NewSHA256Digest(
		"sha256:" + strings.Repeat("a", 64),
	)
	if err != nil {
		t.Fatalf("NewSHA256Digest() error = %v", err)
	}
	typeEnvRef, err := typedmemory.NewTypeEnvRef(digest)
	if err != nil {
		t.Fatalf("NewTypeEnvRef() error = %v", err)
	}
	headRef, err := projecttypeenvselection.ProjectTypeEnvHeadRefForProject(
		project,
	)
	if err != nil {
		t.Fatalf("ProjectTypeEnvHeadRefForProject() error = %v", err)
	}
	headRevision, err := projecttypeenvselection.NewHeadRevision(1)
	if err != nil {
		t.Fatalf("NewHeadRevision() error = %v", err)
	}
	selectionReceiptRef, err := projecttypeenvselectioneffect.ParseProjectTypeEnvHeadSelectionReceiptRef(
		"project-typeenv-head-selection-receipt:sha256:" + strings.Repeat("b", 64),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvHeadSelectionReceiptRef() error = %v", err)
	}
	selectionClosureRef, err := projecttypeenvselectioneffect.ParseProjectTypeEnvHeadSelectionClosureRef(
		"project-typeenv-head-selection-closure:sha256:" + strings.Repeat("c", 64),
	)
	if err != nil {
		t.Fatalf("ParseProjectTypeEnvHeadSelectionClosureRef() error = %v", err)
	}
	basis, err := newSelectedProjectTypeEnvBasis(
		selectedProjectTypeEnvBasisInput{
			project:             project,
			headRef:             headRef,
			headRevision:        headRevision,
			typeEnvRef:          typeEnvRef,
			graphRevision:       typedmemory.NewGraphRevision(7),
			selectionReceiptRef: selectionReceiptRef,
			selectionClosureRef: selectionClosureRef,
		},
	)
	if err != nil {
		t.Fatalf("newSelectedProjectTypeEnvBasis() error = %v", err)
	}
	return basis
}

type committedImport struct {
	planDigest   typedmemory.SHA256Digest
	receiptRef   ImportReceiptRef
	receiptBytes []byte
}

type memoryImportApplyStore struct {
	selected     SelectedProjectTypeEnvBasis
	selectionErr error
	appendErr    error
	commitErr    error
	beforeCommit func()
	committed    map[string]committedImport
	events       []string
	probeCalls   int
	resolveCalls int
	appendCalls  int
	commitCalls  int
}

func newMemoryImportApplyStore(
	t *testing.T,
	selectedProject projectidentity.ProjectID,
) *memoryImportApplyStore {
	t.Helper()
	return &memoryImportApplyStore{
		selected:  mustSelectedProjectTypeEnvBasis(t, selectedProject),
		committed: map[string]committedImport{},
	}
}

func (*memoryImportApplyStore) legacyImportApplyStore() {}

func (store *memoryImportApplyStore) RunImportTransaction(
	ctx context.Context,
	operation func(ImportApplyTransaction) error,
) error {
	if err := ctx.Err(); err != nil {
		store.events = append(store.events, "rollback")
		return err
	}
	transaction := &memoryImportApplyTransaction{store: store}
	err := operation(transaction)
	if err != nil {
		store.events = append(store.events, "rollback")
		return err
	}
	if store.beforeCommit != nil {
		store.beforeCommit()
	}
	if err := ctx.Err(); err != nil {
		store.events = append(store.events, "rollback")
		return err
	}
	if store.commitErr != nil {
		store.events = append(store.events, "rollback")
		return store.commitErr
	}
	if transaction.pending != nil {
		coordinate := ImportApplyCoordinate{
			project:    transaction.pending.plan.ProjectID(),
			key:        transaction.pending.receipt.IdempotencyKey(),
			planDigest: transaction.pending.plan.Digest(),
		}
		store.committed[storeKey(coordinate)] = committedImport{
			planDigest:   transaction.pending.plan.Digest(),
			receiptRef:   transaction.pending.receipt.Ref(),
			receiptBytes: transaction.pending.receipt.CanonicalBytes(),
		}
	}
	store.events = append(store.events, "commit")
	store.commitCalls++
	return nil
}

type memoryImportApplyTransaction struct {
	store   *memoryImportApplyStore
	pending *OpaqueImportBatch
}

func (*memoryImportApplyTransaction) legacyImportApplyTransaction() {}

func (transaction *memoryImportApplyTransaction) ProbeImportReplay(
	_ context.Context,
	coordinate ImportApplyCoordinate,
) (ImportReplayProbe, error) {
	transaction.store.events = append(transaction.store.events, "probe")
	transaction.store.probeCalls++
	stored, exists := transaction.store.committed[storeKey(coordinate)]
	if !exists {
		return ImportReplayAbsent{}, nil
	}
	if stored.planDigest != coordinate.PlanDigest() {
		return newImportReplayConflict(stored.planDigest), nil
	}
	receipt, err := DecodeImportReceipt(stored.receiptBytes)
	if err != nil {
		return nil, fmt.Errorf("%w: decode persisted receipt: %v", ErrImportReplayCorrupt, err)
	}
	replay, err := newImportReplayExact(stored.receiptRef, receipt)
	if err != nil {
		return nil, err
	}
	return replay, nil
}

func (transaction *memoryImportApplyTransaction) ResolveSelectedProjectTypeEnv(
	_ context.Context,
	_ projectidentity.ProjectID,
) (SelectedProjectTypeEnvBasis, error) {
	transaction.store.events = append(transaction.store.events, "resolve")
	transaction.store.resolveCalls++
	if transaction.store.selectionErr != nil {
		return SelectedProjectTypeEnvBasis{}, transaction.store.selectionErr
	}
	return transaction.store.selected, nil
}

func (transaction *memoryImportApplyTransaction) AppendOpaqueImport(
	_ context.Context,
	batch OpaqueImportBatch,
) error {
	transaction.store.events = append(transaction.store.events, "append")
	transaction.store.appendCalls++
	if transaction.store.appendErr != nil {
		return transaction.store.appendErr
	}
	if transaction.pending != nil {
		return fmt.Errorf("test transaction received more than one append")
	}
	owned := batch
	transaction.pending = &owned
	return nil
}

func storeKey(coordinate ImportApplyCoordinate) string {
	return coordinate.ProjectID().String() + "\x00" + coordinate.IdempotencyKey().String()
}

func assertImportStoreCounters(
	t *testing.T,
	store *memoryImportApplyStore,
	probe int,
	resolve int,
	appendCount int,
	commit int,
) {
	t.Helper()
	if store.probeCalls != probe {
		t.Fatalf("probe calls = %d, want %d", store.probeCalls, probe)
	}
	if store.resolveCalls != resolve {
		t.Fatalf("resolve calls = %d, want %d", store.resolveCalls, resolve)
	}
	if store.appendCalls != appendCount {
		t.Fatalf("append calls = %d, want %d", store.appendCalls, appendCount)
	}
	if store.commitCalls != commit {
		t.Fatalf("commit calls = %d, want %d", store.commitCalls, commit)
	}
}
