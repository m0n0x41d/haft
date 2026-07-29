package sqlite

import (
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
)

func declaredProjectProfileBasis(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile, error) {
	return profilebasissqlite.FromCanonicalAdmission(admission)
}

func currentProjectAuthorityContextBinding(
	frame currentGenesisFrame,
	input GenesisSelectionInput,
) (projecttypeenvselectionauthority.ProjectAuthorityContextBinding, error) {
	return projectAuthorityContextBindingFromCurrentProfile(
		frame.projectRoot,
		frame.currentProfile,
		input.Request,
		input.Content,
	)
}
