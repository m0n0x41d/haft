package sqlite

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/operatorrequest"
	profileadmissionsqlite "github.com/m0n0x41d/haft/internal/profileadmission/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis"
	profilebasissqlite "github.com/m0n0x41d/haft/internal/projecttypeenvprofilebasis/sqlite"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselection"
	"github.com/m0n0x41d/haft/internal/projecttypeenvselectionauthority"
)

func declaredProjectProfileBasis(
	admission profileadmissionsqlite.CanonicalProfileAdmission,
) (projecttypeenvprofilebasis.DeclaredCanonicalProjectProfile, error) {
	return profilebasissqlite.FromCanonicalAdmission(admission)
}

func hostRoutedIngressForTest(
	t testing.TB,
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) GenesisAuthorityIngress {
	t.Helper()
	payload, err := projecttypeenvselectionauthority.HostRoutedSelectionPayload(
		request,
		content,
	)
	if err != nil {
		t.Fatalf("seal host-routed selection payload: %v", err)
	}
	operatorRequest, err := operatorrequest.New(
		operatorrequest.ProjectTypeEnvHeadSelect,
		request.Ref().String(),
		payload,
	)
	if err != nil {
		t.Fatalf("seal host-routed operator request: %v", err)
	}
	ingress, err := NewHostRoutedOperatorRequest(operatorRequest)
	if err != nil {
		t.Fatalf("seal host-routed authority ingress: %v", err)
	}
	return ingress
}

func hostRoutedIngressForFixture(
	request projecttypeenvselection.ProjectTypeEnvHeadSelectionRequest,
	content projecttypeenvselectionauthority.ProjectTypeEnvHeadSelectionAuthorizationContent,
) GenesisAuthorityIngress {
	payload, err := projecttypeenvselectionauthority.HostRoutedSelectionPayload(
		request,
		content,
	)
	if err != nil {
		panic(err)
	}
	operatorRequest, err := operatorrequest.New(
		operatorrequest.ProjectTypeEnvHeadSelect,
		request.Ref().String(),
		payload,
	)
	if err != nil {
		panic(err)
	}
	ingress, err := NewHostRoutedOperatorRequest(operatorRequest)
	if err != nil {
		panic(err)
	}
	return ingress
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
