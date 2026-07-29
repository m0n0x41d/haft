package profileauthority_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/m0n0x41d/haft/internal/profileauthority"
)

func TestAdmittedUseHasNoExternalStateOrWireDecoder(t *testing.T) {
	useType := reflect.TypeOf(profileauthority.AdmittedUse{})
	if useType.NumField() != 1 || useType.Field(0).IsExported() {
		t.Fatal("AdmittedUse exposes externally constructible state")
	}
	var use profileauthority.AdmittedUse
	if _, ok := any(&use).(json.Unmarshaler); ok {
		t.Fatal("AdmittedUse unexpectedly accepts a wire representation")
	}
	if _, err := json.Marshal(use); err == nil {
		t.Fatal("AdmittedUse unexpectedly has a persistent JSON form")
	}
	if _, _, ok := use.Resolution(); ok {
		t.Fatal("zero AdmittedUse is valid")
	}
}

func TestAuthorityResolutionRecordHasNoExternalBuilderState(t *testing.T) {
	recordType := reflect.TypeOf(profileauthority.AuthorityResolutionRecord{})
	if recordType.NumField() != 1 || recordType.Field(0).IsExported() {
		t.Fatal("AuthorityResolutionRecord exposes externally constructible state")
	}
	var record profileauthority.AuthorityResolutionRecord
	if _, ok := record.Digest(); ok {
		t.Fatal("zero AuthorityResolutionRecord is valid")
	}
}

func TestNewResolutionDoesNotExposeConsumableUse(t *testing.T) {
	resultType := reflect.TypeOf(profileauthority.NewResolution{})
	if _, ok := resultType.MethodByName("AdmittedUse"); ok {
		t.Fatal("NewResolution exposes a use before post-Work replay")
	}
}

func TestProfileDeclarationAuthorityResolutionRefRequiresOwnedPrefixAndPayload(
	t *testing.T,
) {
	invalid := []string{
		"",
		"authority-resolution:profile",
		"profile-authority-resolution:",
	}
	for _, raw := range invalid {
		if _, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(raw); err == nil {
			t.Fatalf("resolution ref %q unexpectedly parsed", raw)
		}
	}
	ref, err := profileauthority.NewProfileDeclarationAuthorityResolutionRef(
		"profile-authority-resolution:profile",
	)
	if err != nil || ref.String() != "profile-authority-resolution:profile" {
		t.Fatalf("canonical resolution ref = %q, err=%v", ref.String(), err)
	}
}

func TestAuthorityUseRecordHasNoExternalBuilderState(t *testing.T) {
	recordType := reflect.TypeOf(profileauthority.AuthorityUseRecord{})
	if recordType.NumField() != 1 || recordType.Field(0).IsExported() {
		t.Fatal("AuthorityUseRecord exposes externally constructible state")
	}
	var record profileauthority.AuthorityUseRecord
	if _, ok := record.Digest(); ok {
		t.Fatal("zero AuthorityUseRecord is valid")
	}
	if _, err := profileauthority.NewProfileDeclarationAuthorityUseRef(
		"profile-authority-use:",
	); err == nil {
		t.Fatal("empty profile-authority-use ref payload parsed")
	}
}
