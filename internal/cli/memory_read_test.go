package cli

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemorywire"
	"github.com/spf13/cobra"
)

func TestMemoryReadCommandsArePublicAndUseInputFiles(
	t *testing.T,
) {
	registered := map[string]bool{}
	for _, command := range memoryCmd.Commands() {
		registered[command.Name()] = true
	}
	for _, command := range []*cobra.Command{
		memoryResolveCmd,
		memoryNeighborhoodCmd,
		memoryRecallCmd,
	} {
		if !registered[command.Name()] {
			t.Fatalf("memory %s is not publicly registered", command.Name())
		}
		flag := command.Flags().Lookup("input-file")
		if flag == nil || flag.DefValue != "" {
			t.Fatalf("%s --input-file flag = %#v", command.Name(), flag)
		}
	}
}

func TestMemoryReadCommandRejectsCrossActionBeforeOpeningProject(
	t *testing.T,
) {
	openCalls := 0
	previousOpener := openProjectMemoryReadSession
	openProjectMemoryReadSession = func(
		context.Context,
	) (projectMemoryReadSession, error) {
		openCalls++
		return &fixedProjectMemoryReadSession{}, nil
	}
	t.Cleanup(func() {
		openProjectMemoryReadSession = previousOpener
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetIn(strings.NewReader(memoryResolveReadPayload()))

	err := runProjectMemoryRead(
		command,
		"-",
		typedmemorywire.ActionRecall,
	)
	if err == nil || !strings.Contains(err.Error(), "requires action") {
		t.Fatalf("cross-action read error = %v", err)
	}
	if openCalls != 0 {
		t.Fatalf("cross-action request opened project %d time(s)", openCalls)
	}
}

func TestRunMemoryResolveUsesOneReadOnlySessionAndClosesIt(
	t *testing.T,
) {
	want := []byte(
		`{"contract_version":"haft.memory.v1","action":"resolve","result_kind":"known_absent","result":{}}` + "\n",
	)
	session := &fixedProjectMemoryReadSession{result: want}
	previousOpener := openProjectMemoryReadSession
	openProjectMemoryReadSession = func(
		context.Context,
	) (projectMemoryReadSession, error) {
		return session, nil
	}
	t.Cleanup(func() {
		openProjectMemoryReadSession = previousOpener
	})
	previousInput := memoryResolveInputFile
	memoryResolveInputFile = "-"
	t.Cleanup(func() {
		memoryResolveInputFile = previousInput
	})
	command := &cobra.Command{}
	command.SetContext(context.Background())
	command.SetIn(strings.NewReader(memoryResolveReadPayload()))
	output := bytes.Buffer{}
	command.SetOut(&output)

	if err := runMemoryResolve(command, nil); err != nil {
		t.Fatalf("runMemoryResolve() error = %v", err)
	}
	if !bytes.Equal(output.Bytes(), want) {
		t.Fatalf("runMemoryResolve() output = %q, want %q", output.Bytes(), want)
	}
	if session.calls != 1 ||
		session.action != typedmemorywire.ActionResolve ||
		!session.closed {
		t.Fatalf("read session = %#v", session)
	}
}

func TestBoundProjectMemoryReadOpenLeavesCurrentStoreUnchanged(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_8eadbeef")
	configureBoundProjectMemoryAdmissionFixture(t, fixture)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)

	session, err := openBoundProjectMemoryReadRuntime(context.Background())
	if err != nil {
		t.Fatalf("openBoundProjectMemoryReadRuntime() error = %v", err)
	}
	runtimeType := reflect.TypeOf(session.runtime)
	if _, found := runtimeType.MethodByName("Admit"); found {
		_ = session.Close()
		t.Fatal("bound project-memory read runtime exposes admission")
	}
	if err := session.Close(); err != nil {
		t.Fatal(err)
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatal("read-only runtime open changed SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatal("read-only runtime open changed project-store files")
	}
}

type fixedProjectMemoryReadSession struct {
	result []byte
	err    error
	calls  int
	action string
	closed bool
}

func (session *fixedProjectMemoryReadSession) Execute(
	_ context.Context,
	request typedmemorywire.Request,
) ([]byte, error) {
	session.calls++
	session.action = request.Action()
	return append([]byte(nil), session.result...), session.err
}

func (session *fixedProjectMemoryReadSession) Close() error {
	session.closed = true
	return nil
}

func memoryResolveReadPayload() string {
	return `{
  "contract_version":"haft.memory.v1",
  "action":"resolve",
  "basis":{
    "kind":"exact_project",
    "type_env_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
    "graph_revision":1
  },
  "query":"authorization service",
  "max_candidates":8
}`
}
