package cli

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/m0n0x41d/haft/internal/artifact"
)

func TestLegacyCLIStoresCannotBypassProjectMigrationCoordinator(
	t *testing.T,
) {
	tests := []struct {
		name string
		run  func(*testing.T) error
	}{
		{
			name: "artifact helper",
			run: func(*testing.T) error {
				_, _, _, err := openArtifactCLIStore()
				return err
			},
		},
		{
			name: "overseer helper",
			run: func(*testing.T) error {
				_, _, _, err := openOverseerProjectStore()
				return err
			},
		},
		{
			name: "commission helper",
			run: func(t *testing.T) error {
				return withCommissionProject(func(
					context.Context,
					*artifact.Store,
					string,
				) error {
					t.Fatal("commission callback ran on an old schema")
					return nil
				})
			},
		},
		{
			name: "sync command",
			run: func(*testing.T) error {
				return runSync(&cobra.Command{}, nil)
			},
		},
	}
	for index, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			projectID := fmt.Sprintf("qnt_7fac7fa%d", index)
			fixture := newReadOnlyProjectValidationFixture(t, projectID)
			executeReadOnlyProjectValidationFixtureSQL(
				t,
				fixture.database,
				"DELETE FROM schema_version WHERE version = 58",
			)
			beforeSchema := readOnlyProjectValidationSchema(
				t,
				fixture.database,
			)
			beforeFiles := readOnlyProjectValidationFiles(
				t,
				fixture.databaseDirectory,
			)
			restore := enterTestProjectRoot(t, fixture.binding.ProjectRoot)
			err := test.run(t)
			restore()
			if err == nil {
				t.Fatal("legacy CLI store accepted an old schema")
			}
			for _, fragment := range []string{
				"kernel schema is not current",
				"haft project migrate",
				"no migration was attempted",
			} {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf("legacy CLI error %q does not contain %q", err, fragment)
				}
			}
			afterSchema := readOnlyProjectValidationSchema(
				t,
				fixture.database,
			)
			afterFiles := readOnlyProjectValidationFiles(
				t,
				fixture.databaseDirectory,
			)
			if !reflect.DeepEqual(afterSchema, beforeSchema) {
				t.Fatal("legacy CLI store changed the old schema")
			}
			if !reflect.DeepEqual(afterFiles, beforeFiles) {
				t.Fatal("legacy CLI store changed project-ledger files")
			}
		})
	}
}
