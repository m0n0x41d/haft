package check

import "reflect"

// DeclarationMismatch checks facts reconstructed from the pinned claim and
// selected oracle. Changing raw code/oracle/dependency bytes is currentness drift,
// not grounds to erase a historical observation. Scope, failure markers and seed
// remain caller declarations, not independently attested runner facts.
func DeclarationMismatch(expected, declared Contract) string {
	if expected.Ref != declared.Ref || expected.Selector != declared.Selector {
		return "supplied oracle reference or exact package/test selector differs from the declared check"
	}
	if expected.Basis.Claim != declared.Basis.Claim || !reflect.DeepEqual(expected.Basis.Conditions, declared.Basis.Conditions) {
		return "supplied claim or binding conditions differ from the exact declared claim"
	}
	if expected.Scope != declared.Scope || expected.FailureContract != declared.FailureContract || expected.FailurePattern != declared.FailurePattern || !reflect.DeepEqual(expected.Basis.Seed, declared.Basis.Seed) {
		return "supplied scope, failure contract or seed differs from the reconstructed declaration"
	}
	return ""
}
