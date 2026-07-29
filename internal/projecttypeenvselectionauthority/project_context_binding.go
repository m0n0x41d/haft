package projecttypeenvselectionauthority

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/m0n0x41d/haft/internal/authority"
	"github.com/m0n0x41d/haft/internal/projectidentity"
)

const projectContextBindingDomain = "haft.project-typeenv.authority-project-context-binding/v1"

// ProjectAuthorityContextBinding is an explicit observable relation between
// Haft's ProjectID and the generic SpeechAct source's ProjectRoot. Unlike raw
// string comparison it keeps the unlike identities distinct and binds the
// exact context plus an observable carrier. Its truth/currentness remains the
// responsibility of the future project-ledger effect adapter.
type ProjectAuthorityContextBinding struct {
	project       projectidentity.ProjectID
	root          authority.ProjectRoot
	context       authority.BoundedContextRef
	carrier       authority.ObservableCarrierBinding
	digest        authority.Digest
	canonicalJSON []byte
}

type ProjectAuthorityContextBindingInput struct {
	Project projectidentity.ProjectID
	Root    authority.ProjectRoot
	Context authority.BoundedContextRef
	Carrier authority.ObservableCarrierBinding
}

func SealProjectAuthorityContextBinding(
	input ProjectAuthorityContextBindingInput,
) (ProjectAuthorityContextBinding, error) {
	project, err := projectidentity.ParseProjectID(input.Project.String())
	if err != nil || project != input.Project {
		return ProjectAuthorityContextBinding{}, fmt.Errorf("project context binding ProjectID is invalid")
	}
	root, err := authority.NewProjectRoot(input.Root.String())
	if err != nil || root != input.Root {
		return ProjectAuthorityContextBinding{}, fmt.Errorf("project context binding ProjectRoot is invalid")
	}
	context, err := authority.NewBoundedContextRef(input.Context.String())
	if err != nil || context != input.Context {
		return ProjectAuthorityContextBinding{}, fmt.Errorf("project context binding context is invalid")
	}
	carrierRef := input.Carrier.Ref()
	carrierDigest := input.Carrier.Digest()
	carrier, err := authority.NewObservableCarrierBinding(carrierRef, carrierDigest)
	if err != nil {
		return ProjectAuthorityContextBinding{}, fmt.Errorf("project context binding carrier: %w", err)
	}
	projection := struct {
		Schema        string `json:"schema"`
		Project       string `json:"project_id"`
		Root          string `json:"project_root"`
		Context       string `json:"bounded_context_ref"`
		CarrierRef    string `json:"carrier_ref"`
		CarrierDigest string `json:"carrier_digest"`
	}{
		Schema:        "haft.project-typeenv.authority-project-context-binding/v1",
		Project:       project.String(),
		Root:          root.String(),
		Context:       context.String(),
		CarrierRef:    carrier.Ref().String(),
		CarrierDigest: carrier.Digest().String(),
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return ProjectAuthorityContextBinding{}, err
	}
	digest, err := digestCanonical(projectContextBindingDomain, canonical)
	if err != nil {
		return ProjectAuthorityContextBinding{}, err
	}
	return ProjectAuthorityContextBinding{
		project: project, root: root, context: context, carrier: carrier,
		digest: digest, canonicalJSON: canonical,
	}, nil
}

func (binding ProjectAuthorityContextBinding) ExactFor(
	project projectidentity.ProjectID,
	root authority.ProjectRoot,
	context authority.BoundedContextRef,
) bool {
	rebuilt, err := SealProjectAuthorityContextBinding(ProjectAuthorityContextBindingInput{
		Project: binding.project, Root: binding.root, Context: binding.context, Carrier: binding.carrier,
	})
	return err == nil && rebuilt.digest == binding.digest &&
		bytes.Equal(rebuilt.canonicalJSON, binding.canonicalJSON) &&
		binding.project == project && binding.root == root && binding.context == context
}

func DecodeProjectAuthorityContextBinding(
	input ProjectAuthorityContextBindingInput,
	canonical []byte,
	digest authority.Digest,
) (ProjectAuthorityContextBinding, error) {
	if len(canonical) == 0 || len(canonical) > 64*1024 {
		return ProjectAuthorityContextBinding{}, fmt.Errorf(
			"project context binding has invalid canonical size",
		)
	}
	projection := struct {
		Schema        string `json:"schema"`
		Project       string `json:"project_id"`
		Root          string `json:"project_root"`
		Context       string `json:"bounded_context_ref"`
		CarrierRef    string `json:"carrier_ref"`
		CarrierDigest string `json:"carrier_digest"`
	}{}
	if err := decodeStrictJSON(canonical, &projection); err != nil {
		return ProjectAuthorityContextBinding{}, err
	}
	rebuilt, err := SealProjectAuthorityContextBinding(input)
	if err != nil {
		return ProjectAuthorityContextBinding{}, err
	}
	if rebuilt.digest != digest || !bytes.Equal(rebuilt.canonicalJSON, canonical) {
		return ProjectAuthorityContextBinding{}, fmt.Errorf(
			"project context binding is not exact canonical material",
		)
	}
	return rebuilt, nil
}

func (binding ProjectAuthorityContextBinding) Project() projectidentity.ProjectID {
	return binding.project
}
func (binding ProjectAuthorityContextBinding) Root() authority.ProjectRoot { return binding.root }
func (binding ProjectAuthorityContextBinding) Context() authority.BoundedContextRef {
	return binding.context
}
func (binding ProjectAuthorityContextBinding) Carrier() authority.ObservableCarrierBinding {
	return binding.carrier
}
func (binding ProjectAuthorityContextBinding) Digest() authority.Digest { return binding.digest }
func (binding ProjectAuthorityContextBinding) CanonicalJSON() []byte {
	return append([]byte(nil), binding.canonicalJSON...)
}
