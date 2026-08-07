package existingrecordprojection_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/m0n0x41d/haft/internal/artifact"
	"github.com/m0n0x41d/haft/internal/projectmemory/existingrecordprojection"
)

func TestSourceArgumentsRecoverCanonicalNoteClaims(t *testing.T) {
	t.Parallel()

	record := artifact.BuildNoteArtifact(
		"note-source-owned",
		time.Date(2026, 7, 26, 9, 0, 0, 0, time.UTC),
		artifact.NoteInput{
			Title:        "Source-owned note",
			Observations: []string{"Typed memory preserves exact source claims."},
			Rationale:    "The carrier remains the source.",
			Evidence:     "test:canonical-note",
		},
	)
	route := singleExistingRecordRoute(t, record)
	arguments, err := existingrecordprojection.SourceArguments(
		route,
		record,
		existingrecordprojection.ConcernCoordinates{
			RefKindID:        "U.EntityRef",
			ReferenceID:      "entity:typed-memory",
			BoundedContextID: "haft-project",
		},
	)
	if err != nil {
		t.Fatalf("SourceArguments() error = %v", err)
	}

	if got, want := arguments["observations"], []string{
		"Typed memory preserves exact source claims.",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("observations = %#v, want %#v", got, want)
	}
	if arguments["rationale"] != "The carrier remains the source." {
		t.Fatalf("rationale = %#v", arguments["rationale"])
	}
	if arguments["evidence"] != "test:canonical-note" {
		t.Fatalf("evidence = %#v", arguments["evidence"])
	}
}

func TestSourceArgumentsRejectNoteWithoutRecoverableClaims(t *testing.T) {
	t.Parallel()

	record := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      "note-unresolved-source",
			Kind:    artifact.KindNote,
			Version: 1,
			Title:   "Unstructured historical note",
		},
		Body: "# Unstructured historical note\n\nNo canonical claim section.\n",
	}
	route := singleExistingRecordRoute(t, record)
	_, err := existingrecordprojection.SourceArguments(
		route,
		record,
		existingrecordprojection.ConcernCoordinates{
			RefKindID:        "U.EntityRef",
			ReferenceID:      "entity:typed-memory",
			BoundedContextID: "haft-project",
		},
	)
	if err == nil {
		t.Fatal("SourceArguments() error = nil, want unresolved source")
	}
}

func TestSourceArgumentsRecoverProblemStructuredData(t *testing.T) {
	t.Parallel()

	fields := artifact.ProblemFields{
		ProblemType: artifact.ProblemTypeDiagnosis,
		Signal:      "A selected record is absent from typed recall.",
		Profile: &artifact.ProblemCardProfile{
			Level:           "deep",
			SourceKind:      "observed_problem",
			Scope:           "existing record projection",
			AcceptanceProbe: "dry-run validates without writes",
		},
		Constraints: []string{"Do not guess identity."},
		Acceptance:  "The exact carrier projects through its source adapter.",
	}
	encoded, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	record := &artifact.Artifact{
		Meta: artifact.Meta{
			ID:      "prob-source-owned",
			Kind:    artifact.KindProblemCard,
			Version: 1,
			Title:   "Recover structured problem",
		},
		StructuredData: string(encoded),
	}
	route := singleExistingRecordRoute(t, record)
	arguments, err := existingrecordprojection.SourceArguments(
		route,
		record,
		existingrecordprojection.ConcernCoordinates{
			RefKindID:        "U.EntityRef",
			ReferenceID:      "entity:typed-memory",
			BoundedContextID: "haft-project",
		},
	)
	if err != nil {
		t.Fatalf("SourceArguments() error = %v", err)
	}
	if arguments["signal"] != fields.Signal ||
		arguments["acceptance_probe"] != fields.Profile.AcceptanceProbe {
		t.Fatalf("problem arguments = %#v", arguments)
	}
}

func singleExistingRecordRoute(
	t *testing.T,
	record *artifact.Artifact,
) existingrecordprojection.Route {
	t.Helper()

	plan, err := existingrecordprojection.Build(
		[]*artifact.Artifact{record},
	)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	routes := plan.Routes()
	if len(routes) != 1 {
		t.Fatalf("routes = %#v, want exactly one", routes)
	}
	return routes[0]
}
