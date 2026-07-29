package specmigrationv2_test

import (
	"context"
	"testing"

	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/specmigrationv2"
)

func TestPublicMigrationEffectBoundaryRejectsUnadmittedRequests(t *testing.T) {
	apply := specmigrationv2.ApplyMigration(
		context.Background(),
		profileadmissionsqlite.Service{},
		specmigrationv2.ApplyRequest{},
	)
	if _, ok := apply.(specmigrationv2.ApplyRejected); !ok {
		t.Fatalf("ApplyMigration result = %T, want ApplyRejected", apply)
	}

	recover := specmigrationv2.RecoverMigration(
		context.Background(),
		profileadmissionsqlite.Service{},
		specmigrationv2.ReviewAdmissionService{},
		specmigrationv2.RecoveryRequest{},
	)
	if _, ok := recover.(specmigrationv2.ApplyRejected); !ok {
		t.Fatalf("RecoverMigration result = %T, want ApplyRejected", recover)
	}
}
