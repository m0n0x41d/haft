package codebase

import "testing"

func TestSymbolAnchorStableAcrossLineAndBodyChanges(t *testing.T) {
	first := SymbolSnapshot{
		FilePath:      "src/service.ts",
		SymbolName:    "run",
		SymbolKind:    "method",
		Receiver:      "Service",
		QualifiedName: "Service.run",
		SignatureHash: signatureHashFromDeclaration("method", "Service.run", []byte("run(input: string): number { return 1 }")),
		Line:          10,
		EndLine:       12,
		Hash:          "body-one",
	}
	second := first
	second.Line = 200
	second.EndLine = 204
	second.Hash = "body-two"

	firstAnchor := BuildSymbolAnchor(first, "typescript")
	secondAnchor := BuildSymbolAnchor(second, "typescript")
	if firstAnchor.ID != secondAnchor.ID {
		t.Fatalf("line/body edit changed anchor: %s != %s", firstAnchor.ID, secondAnchor.ID)
	}
}

func TestSymbolAnchorSeparatesReceiversAndOverloads(t *testing.T) {
	makeAnchor := func(receiver, declaration string) SymbolAnchor {
		qualified := qualifiedSymbolName(receiver, "run")
		snapshot := SymbolSnapshot{
			FilePath:      "src/service.ts",
			SymbolName:    "run",
			SymbolKind:    "method",
			Receiver:      receiver,
			QualifiedName: qualified,
			SignatureHash: signatureHashFromDeclaration("method", qualified, []byte(declaration)),
		}
		return BuildSymbolAnchor(snapshot, "typescript")
	}

	stringRun := makeAnchor("Service", "run(input: string): number { return 1 }")
	numberRun := makeAnchor("Service", "run(input: number): number { return 2 }")
	otherReceiver := makeAnchor("Worker", "run(input: string): number { return 1 }")
	if stringRun.ID == numberRun.ID {
		t.Fatal("overload signatures collapsed to one anchor")
	}
	if stringRun.ID == otherReceiver.ID {
		t.Fatal("same-name methods on different receivers collapsed to one anchor")
	}
}

func TestSignatureHashExcludesCallableBody(t *testing.T) {
	first := signatureHashFromDeclaration("func", "parse", []byte("function parse(input: string): number { return 1 }"))
	second := signatureHashFromDeclaration("func", "parse", []byte("function parse(input: string): number { return 999 }"))
	if first != second {
		t.Fatalf("implementation-only edit changed signature hash: %s != %s", first, second)
	}
}

func TestCompareSymbolSnapshotsSeparatesSameNameReceivers(t *testing.T) {
	baseline := []SymbolSnapshot{
		{FilePath: "src/service.ts", SymbolName: "run", SymbolKind: "method", Receiver: "A", Line: 10, Hash: "a"},
		{FilePath: "src/service.ts", SymbolName: "run", SymbolKind: "method", Receiver: "B", Line: 20, Hash: "b"},
	}
	current := []SymbolSnapshot{
		{FilePath: "src/service.ts", SymbolName: "run", SymbolKind: "method", Receiver: "A", Line: 100, Hash: "changed"},
		{FilePath: "src/service.ts", SymbolName: "run", SymbolKind: "method", Receiver: "B", Line: 110, Hash: "b"},
	}

	drifts := CompareSymbolSnapshots(baseline, current)
	if len(drifts) != 1 {
		t.Fatalf("drifts = %+v, want one modified A.run", drifts)
	}
	if drifts[0].Status != "modified" || drifts[0].OldLine != 10 || drifts[0].NewLine != 100 {
		t.Fatalf("drift = %+v, want A.run line 10->100 modified", drifts[0])
	}
}

func TestCompareSymbolSnapshotsBridgesLegacyMissingSignature(t *testing.T) {
	baseline := []SymbolSnapshot{{
		FilePath:   "src/service.ts",
		SymbolName: "run",
		SymbolKind: "method",
		Receiver:   "Service",
		Line:       10,
		Hash:       "same-body",
	}}
	current := []SymbolSnapshot{{
		FilePath:      "src/service.ts",
		SymbolName:    "run",
		SymbolKind:    "method",
		QualifiedName: "Service.run",
		SignatureHash: signatureHashFromDeclaration("method", "Service.run", []byte("run(input: string): void {}")),
		Receiver:      "Service",
		Line:          200,
		Hash:          "same-body",
	}}

	drifts := CompareSymbolSnapshots(baseline, current)
	if len(drifts) != 0 {
		t.Fatalf("legacy baseline should bridge to one current signature without false remove/add: %+v", drifts)
	}
}
