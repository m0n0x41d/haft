package artifact

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestPrepareDecisionDoesNotPersistOrWriteMarkdown(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	reservation, err := ReserveDecisionIdentity("read-only preparation")
	if err != nil {
		t.Fatal(err)
	}

	prepared, err := PrepareDecision(
		ctx,
		store,
		haftDir,
		reservation,
		completeDecision(DecideInput{
			SelectedTitle: "Keep preparation read-only",
			WhySelected:   "Human review must precede every institutional decision effect.",
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	decisions, err := store.ListByKind(ctx, KindDecisionRecord, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 0 {
		t.Fatalf("PrepareDecision persisted decisions: %#v", decisions)
	}
	decisionDir := filepath.Join(haftDir, KindDecisionRecord.Dir())
	if _, err := os.Stat(decisionDir); !os.IsNotExist(err) {
		t.Fatalf("PrepareDecision created Markdown directory %q: %v", decisionDir, err)
	}
	canonical, ok := prepared.CanonicalBytes()
	if !ok {
		t.Fatal("prepared decision did not expose canonical bytes")
	}
	if bytes.Contains(canonical, []byte(`"created_at"`)) ||
		bytes.Contains(canonical, []byte(`"updated_at"`)) {
		t.Fatalf("time-free prepared decision contains institutional timestamps: %s", canonical)
	}
	if !prepared.state.semanticArtifact.Meta.CreatedAt.IsZero() ||
		!prepared.state.semanticArtifact.Meta.UpdatedAt.IsZero() {
		t.Fatal("prepared semantic artifact predates the verified decision occurrence")
	}
}

func TestPreparedDecisionArtifactAtUsesVerifiedOccurrenceAndReturnsCopies(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	problem := &Artifact{
		Meta: Meta{
			ID:      "prob-20260715-verified-occurrence-a1b2c3d4",
			Kind:    KindProblemCard,
			Status:  StatusActive,
			Context: "authority",
			Mode:    ModeDeep,
			Title:   "Decision occurrence must be verified",
		},
		Body: "The binding occurrence must remain distinct from preparation.",
	}
	if err := store.Create(ctx, problem); err != nil {
		t.Fatal(err)
	}
	reservation, err := ReserveDecisionIdentity("verified occurrence")
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := PrepareDecision(
		ctx,
		store,
		filepath.Join(t.TempDir(), ".haft"),
		reservation,
		completeDecision(DecideInput{
			ProblemRef:    problem.Meta.ID,
			SelectedTitle: "Bind only after the SpeechAct",
			WhySelected:   "The occurrence, rather than preparation, institutes the decision.",
			SectionRefs:   []string{"TS.authority.001"},
		}),
	)
	if err != nil {
		t.Fatal(err)
	}

	localZone := time.FixedZone("UTC+04", 4*60*60)
	occurrence := time.Date(2026, time.July, 15, 12, 34, 56, 789, localZone)
	wantOccurrence := occurrence.UTC().Round(0)
	first, err := prepared.artifactAt(occurrence)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Meta.CreatedAt.Equal(wantOccurrence) ||
		!first.Meta.UpdatedAt.Equal(wantOccurrence) {
		t.Fatalf(
			"artifact times = (%s, %s), want verified occurrence %s",
			first.Meta.CreatedAt,
			first.Meta.UpdatedAt,
			wantOccurrence,
		)
	}
	if first.Meta.CreatedAt.Location() != time.UTC || first.Meta.UpdatedAt.Location() != time.UTC {
		t.Fatalf("artifact occurrence was not normalized to UTC: %+v", first.Meta)
	}

	first.Body = "tampered body"
	first.Meta.Links[0].Ref = "tampered-ref"
	second, err := prepared.artifactAt(occurrence)
	if err != nil {
		t.Fatal(err)
	}
	if second.Body == first.Body || second.Meta.Links[0].Ref == first.Meta.Links[0].Ref {
		t.Fatal("ArtifactAt leaked a mutable view of the prepared semantic snapshot")
	}
	if _, err := prepared.artifactAt(time.Time{}); err == nil {
		t.Fatal("ArtifactAt accepted a missing verified SpeechAct occurrence")
	}
}

func TestPreparedDecisionDigestIsDeterministicAndRevalidationDetectsSourceDrift(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	problem := &Artifact{
		Meta: Meta{
			ID:      "prob-20260715-source-a1b2c3d4",
			Kind:    KindProblemCard,
			Status:  StatusActive,
			Context: "authority",
			Mode:    ModeDeep,
			Title:   "Manual decision authorization is unclear",
		},
		Body: "## Signal\nAgents can submit decision-shaped data without a verified human act.\n\n" +
			"## Constraints\n- Preserve an exact reviewable intent.\n\n" +
			"## Acceptance\nA source change invalidates prepared authorization content.\n",
	}
	if err := store.Create(ctx, problem); err != nil {
		t.Fatal(err)
	}
	reservation, err := ReserveDecisionIdentity("source-pinned decision")
	if err != nil {
		t.Fatal(err)
	}
	input := completeDecision(DecideInput{
		ProblemRef:    problem.Meta.ID,
		SelectedTitle: "Use source-pinned prepared decisions",
		WhySelected:   "Authorization must bind the fully resolved semantic snapshot.",
	})

	haftDir := filepath.Join(t.TempDir(), ".haft")
	first, err := PrepareDecision(ctx, store, haftDir, reservation, input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := PrepareDecision(ctx, store, haftDir, reservation, input)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, ok := first.Digest()
	if !ok {
		t.Fatal("first prepared decision has no digest")
	}
	secondDigest, ok := second.Digest()
	if !ok {
		t.Fatal("second prepared decision has no digest")
	}
	if firstDigest.String() != secondDigest.String() {
		t.Fatalf("same proposal and sources produced different digests: %s != %s", firstDigest.String(), secondDigest.String())
	}
	firstBytes, _ := first.CanonicalBytes()
	secondBytes, _ := second.CanonicalBytes()
	if !bytes.Equal(firstBytes, secondBytes) {
		t.Fatal("same proposal and sources produced different canonical snapshots")
	}
	if err := RevalidatePreparedDecision(ctx, store, haftDir, first); err != nil {
		t.Fatalf("unchanged prepared decision failed revalidation: %v", err)
	}

	currentProblem, err := store.Get(ctx, problem.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	currentProblem.Body = strings.Replace(
		currentProblem.Body,
		"Preserve an exact reviewable intent.",
		"Preserve an exact reviewable intent and its source provenance.",
		1,
	)
	if err := store.Update(ctx, currentProblem); err != nil {
		t.Fatal(err)
	}
	if err := RevalidatePreparedDecision(ctx, store, haftDir, first); err == nil {
		t.Fatal("source drift did not invalidate the prepared decision")
	} else if !strings.Contains(err.Error(), "stale") {
		t.Fatalf("source drift returned an opaque error: %v", err)
	}
}

func TestPreparedDecisionCanonicalCollectionsAndAccessorsAreImmutable(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	reservation, err := ReserveDecisionIdentity("canonical collections")
	if err != nil {
		t.Fatal(err)
	}
	input := completeDecision(DecideInput{
		SelectedTitle: "Canonicalize the reviewed decision scope",
		WhySelected:   "Equivalent scopes must produce one authorization content digest.",
		SectionRefs:   []string{" TS.memory.002 ", "TS.memory.001", "TS.memory.002"},
		AffectedFiles: []string{"z.go", " a.go ", "z.go", "a.go"},
	})
	prepared, err := PrepareDecision(
		ctx,
		store,
		filepath.Join(t.TempDir(), ".haft"),
		reservation,
		input,
	)
	if err != nil {
		t.Fatal(err)
	}

	links, ok := prepared.Links()
	if !ok || len(links) != 0 {
		t.Fatalf(
			"canonical Artifact links = %#v, want none for typed SpecSection refs",
			links,
		)
	}
	wantSectionRefs := []string{"TS.memory.001", "TS.memory.002"}
	wantFiles := []AffectedFile{{Path: "a.go"}, {Path: "z.go"}}
	affected, ok := prepared.AffectedFiles()
	if !ok || !reflect.DeepEqual(affected, wantFiles) {
		t.Fatalf("canonical affected files = %#v, want %#v", affected, wantFiles)
	}

	affected[0].Path = "tampered-file"
	canonical, ok := prepared.CanonicalBytes()
	if !ok {
		t.Fatal("prepared decision did not expose canonical bytes")
	}
	canonical[0] ^= 0xff
	resolved, ok := prepared.ResolvedInput()
	if !ok {
		t.Fatal("prepared decision did not expose resolved input")
	}
	resolved.AffectedFiles[0] = "tampered-input"
	resolved.SectionRefs[0] = "tampered-section"

	linksAgain, _ := prepared.Links()
	affectedAgain, _ := prepared.AffectedFiles()
	canonicalAgain, _ := prepared.CanonicalBytes()
	resolvedAgain, _ := prepared.ResolvedInput()
	if len(linksAgain) != 0 {
		t.Fatalf("Links returned mutable state: %#v", linksAgain)
	}
	if !reflect.DeepEqual(affectedAgain, wantFiles) {
		t.Fatalf("AffectedFiles returned mutable state: %#v", affectedAgain)
	}
	if !bytes.Equal(canonicalAgain, prepared.state.canonicalJSON) {
		t.Fatal("CanonicalBytes returned mutable state")
	}
	if !reflect.DeepEqual(resolvedAgain.AffectedFiles, []string{"a.go", "z.go"}) {
		t.Fatalf("ResolvedInput returned mutable state: %#v", resolvedAgain.AffectedFiles)
	}
	if !reflect.DeepEqual(resolvedAgain.SectionRefs, wantSectionRefs) {
		t.Fatalf(
			"ResolvedInput returned mutable section refs: %#v",
			resolvedAgain.SectionRefs,
		)
	}
}

func TestPrepareDecisionRejectsAffectedPathsOutsideProject(t *testing.T) {
	store := setupTestDB(t)
	ctx := context.Background()
	haftDir := filepath.Join(t.TempDir(), ".haft")
	invalid := []string{
		"/tmp/outside.go",
		"../outside.go",
		"nested/../../outside.go",
		".",
	}
	for _, affectedPath := range invalid {
		t.Run(affectedPath, func(t *testing.T) {
			reservation, err := ReserveDecisionIdentity("invalid affected path")
			if err != nil {
				t.Fatal(err)
			}
			_, err = PrepareDecision(
				ctx,
				store,
				haftDir,
				reservation,
				completeDecision(DecideInput{
					SelectedTitle: "Reject project-escaping file scope",
					WhySelected:   "A reviewed decision must not bind an ambiguous file outside its project.",
					AffectedFiles: []string{affectedPath},
				}),
			)
			if err == nil {
				t.Fatal("PrepareDecision accepted an invalid affected path")
			}
		})
	}
}

func TestDecideCompatibilityMatchesPreparedDecisionSemantics(t *testing.T) {
	ctx := context.Background()
	projectRoot := t.TempDir()
	haftDir := filepath.Join(projectRoot, ".haft")
	input := completeDecision(DecideInput{
		SelectedTitle: "Keep the Decide compatibility surface",
		WhySelected:   "Existing callers should retain the same persisted DecisionRecord semantics.",
		AffectedFiles: []string{"internal/authority/binding.go"},
	})

	legacyStore := setupTestDB(t)
	decision, filePath, err := Decide(ctx, legacyStore, haftDir, input)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); err != nil {
		t.Fatalf("Decide did not retain its Markdown compatibility effect: %v", err)
	}
	if _, err := legacyStore.Get(ctx, decision.Meta.ID); err != nil {
		t.Fatalf("Decide did not retain its persistence compatibility effect: %v", err)
	}

	reservation, err := NewDecisionReservation(decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	preparationStore := setupTestDB(t)
	prepared, err := PrepareDecision(ctx, preparationStore, haftDir, reservation, input)
	if err != nil {
		t.Fatal(err)
	}
	want, err := prepared.artifactAt(decision.Meta.CreatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Meta.ID != want.Meta.ID ||
		decision.Meta.Kind != want.Meta.Kind ||
		decision.Meta.Version != want.Meta.Version ||
		decision.Meta.Status != want.Meta.Status ||
		decision.Meta.Context != want.Meta.Context ||
		decision.Meta.Mode != want.Meta.Mode ||
		decision.Meta.Title != want.Meta.Title ||
		decision.Meta.ValidUntil != want.Meta.ValidUntil ||
		decision.Body != want.Body ||
		decision.SearchKeywords != want.SearchKeywords ||
		decision.StructuredData != want.StructuredData ||
		!reflect.DeepEqual(decision.Meta.Links, want.Meta.Links) {
		t.Fatalf("Decide semantics diverged from PrepareDecision + ArtifactAt:\n got: %#v\nwant: %#v", decision, want)
	}

	gotFiles, err := legacyStore.GetAffectedFiles(ctx, decision.Meta.ID)
	if err != nil {
		t.Fatal(err)
	}
	wantFiles, ok := prepared.AffectedFiles()
	if !ok {
		t.Fatal("prepared decision did not expose affected files")
	}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Fatalf("Decide affected files = %#v, want prepared scope %#v", gotFiles, wantFiles)
	}
}
