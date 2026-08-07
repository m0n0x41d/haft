package typedmemoryevaluation

import "github.com/m0n0x41d/haft/internal/typedmemory"

// NewPureEvaluatorForTest exposes the sealed constructor only to this
// package's external test binary. It is absent from production builds.
func NewPureEvaluatorForTest[Input, Output any](
	evaluate func(Input) (Output, error),
) (PureEvaluator[Input, Output], error) {
	return newPureEvaluator(evaluate)
}

// NewRegistrationForTest exposes the sealed constructor only to this
// package's external test binary. It is absent from production builds.
func NewRegistrationForTest[Input, Output any](
	rule typedmemory.RuleRef,
	identity MechanismIdentity,
	evaluator PureEvaluator[Input, Output],
) (Registration[Input, Output], error) {
	return newRegistration(rule, identity, evaluator)
}
