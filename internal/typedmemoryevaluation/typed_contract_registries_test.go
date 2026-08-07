package typedmemoryevaluation_test

import (
	"testing"

	"github.com/m0n0x41d/haft/internal/typedmemory"
	"github.com/m0n0x41d/haft/internal/typedmemoryevaluation"
	"github.com/m0n0x41d/haft/internal/typedmemorykindruntime"
)

func TestEntitySetEnumerationRegistryBindsExactCallableIdentity(t *testing.T) {
	t.Parallel()
	rule := typedContractRule(t, "haft.entity-set.enumeration.test/v1")
	identity := typedContractIdentity(t, "artifact:entity-set-enumeration", 0x31)
	registry := typedContractMust(
		typedmemoryevaluation.NewEntitySetEnumerationRegistry(rule, identity),
	)
	lookup := typedContractMust(registry.Lookup(rule, identity))
	if _, ok := lookup.(typedmemoryevaluation.Found[
		typedmemorykindruntime.EntitySetEnumerationRequest,
		typedmemorykindruntime.EntitySetEnumerationResult,
	]); !ok {
		t.Fatalf("Lookup() = %T, want exact EntitySetEnumeration Found", lookup)
	}
}

func TestCandidateVisibilityRegistryBindsExactCallableIdentity(t *testing.T) {
	t.Parallel()
	rule := typedContractRule(t, "haft.candidate.visibility.test/v1")
	identity := typedContractIdentity(t, "artifact:candidate-visibility", 0x41)
	registry := typedContractMust(
		typedmemoryevaluation.NewCandidateVisibilityRegistry(rule, identity),
	)
	lookup := typedContractMust(registry.Lookup(rule, identity))
	if _, ok := lookup.(typedmemoryevaluation.Found[
		typedmemorykindruntime.CandidateVisibilityRequest,
		typedmemorykindruntime.CandidateVisibilityResult,
	]); !ok {
		t.Fatalf("Lookup() = %T, want exact CandidateVisibility Found", lookup)
	}
}

func TestKindDefinednessRegistryBindsExactCallableIdentity(t *testing.T) {
	t.Parallel()
	rule := typedContractRule(t, "haft.kind.definedness.test/v1")
	identity := typedContractIdentity(t, "artifact:kind-definedness", 0x51)
	registry := typedContractMust(
		typedmemoryevaluation.NewKindDefinednessRegistry(rule, identity),
	)
	lookup := typedContractMust(registry.Lookup(rule, identity))
	if _, ok := lookup.(typedmemoryevaluation.Found[
		typedmemorykindruntime.KindDefinednessRequest,
		typedmemorykindruntime.KindDefinednessResult,
	]); !ok {
		t.Fatalf("Lookup() = %T, want exact KindDefinedness Found", lookup)
	}
}

func typedContractRule(t *testing.T, raw string) typedmemory.RuleRef {
	t.Helper()
	return typedContractMust(typedmemory.NewRuleRef(raw))
}

func typedContractIdentity(
	t *testing.T,
	artifactRaw string,
	fill byte,
) typedmemoryevaluation.MechanismIdentity {
	t.Helper()
	artifact := typedContractMust(typedmemory.NewCarrierRef(artifactRaw))
	edition := typedContractMust(typedmemory.NewCarrierEdition("1.0.0"))
	digest := typedContractDigest(t, fill)
	return typedContractMust(typedmemoryevaluation.NewMechanismIdentity(
		artifact,
		edition,
		digest,
		typedmemoryevaluation.EvaluatorRole,
	))
}

func typedContractDigest(t *testing.T, fill byte) typedmemory.SHA256Digest {
	t.Helper()
	raw := make([]byte, len("sha256:")+64)
	copy(raw, "sha256:")
	const alphabet = "0123456789abcdef"
	for index := len("sha256:"); index < len(raw); index++ {
		raw[index] = alphabet[int(fill+byte(index))%len(alphabet)]
	}
	return typedContractMust(typedmemory.NewSHA256Digest(string(raw)))
}

func typedContractMust[T any](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}
