package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestSpecReadCommandsNeverUpgradeAnOldProjectLedger(
	t *testing.T,
) {
	fixture := newReadOnlyProjectValidationFixture(t, "qnt_5fecbeef")
	executeReadOnlyProjectValidationFixtureSQL(
		t,
		fixture.database,
		"DELETE FROM schema_version WHERE version >= 49",
	)
	beforeSchema := readOnlyProjectValidationSchema(t, fixture.database)
	beforeFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	restoreCWD := chdirForTest(t, fixture.binding.ProjectRoot)
	defer restoreCWD()

	commands := []struct {
		name string
		run  func(*cobra.Command, []string) error
	}{
		{name: "check", run: runSpecCheck},
		{name: "status", run: runSpecStatus},
		{name: "next", run: runSpecNext},
	}
	for _, command := range commands {
		t.Run(command.name, func(t *testing.T) {
			cobraCommand := &cobra.Command{}
			cobraCommand.SetOut(&bytes.Buffer{})
			err := command.run(cobraCommand, nil)
			if err == nil {
				t.Fatalf("haft spec %s accepted an old kernel schema", command.name)
			}
			for _, fragment := range []string{
				"kernel schema is not current",
				"haft project migrate",
				"no migration was attempted",
			} {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf(
						"haft spec %s error %q does not contain %q",
						command.name,
						err,
						fragment,
					)
				}
			}
		})
	}

	afterSchema := readOnlyProjectValidationSchema(t, fixture.database)
	afterFiles := readOnlyProjectValidationFiles(
		t,
		fixture.databaseDirectory,
	)
	if !reflect.DeepEqual(afterSchema, beforeSchema) {
		t.Fatalf("spec reads changed the old SQLite schema")
	}
	if !reflect.DeepEqual(afterFiles, beforeFiles) {
		t.Fatalf("spec reads changed old project-ledger files")
	}
}
