package typedmemory

import "testing"

func TestValidateMemoryChangeSetUsesCurrentClassificationWithoutMemberOf(t *testing.T) {
	fixture := newCurrentClassificationValidationFixture(t)
	fixture.snapshot.memberOfJudgements = map[string]MemberOfJudgement{}
	fixture.snapshot.memberOfRequestJudgements = map[string]MemberOfJudgement{}
	fixture.snapshot.memberOfEvaluator = func(
		MemberOfEvaluationRequest,
	) MemberOfJudgement {
		t.Fatal("current C.3.2 validation called the historical MemberOf evaluator")
		return nil
	}
	fixture.snapshot.kindClassificationEvaluator = func(
		request KindClassificationRequest,
	) KindClassificationJudgement {
		if request.LocalKind().ValueKind() == fixture.typeEnv.entityValueKind {
			return validationTestSettledClassification(
				t,
				fixture.environment,
				request,
				KindClassificationTrue,
			)
		}
		return validationTestSettledClassification(
			t,
			fixture.environment,
			request,
			KindClassificationFalse,
		)
	}

	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	valid, ok := verdict.(Valid)
	if !ok {
		t.Fatalf("verdict = %T (%s); want Valid", verdict, verdict.Kind())
	}
	basis, ok := valid.AdmissionBatch().Basis().(ContextSliceClassificationBasis)
	if !ok {
		t.Fatalf(
			"admission basis = %T; want ContextSliceClassificationBasis",
			valid.AdmissionBatch().Basis(),
		)
	}
	uses := basis.ClassificationReferenceFillerAdmissionUses()
	if len(uses) != 1 ||
		uses[0].RequiredClassification().Kind() != KindClassificationTrue ||
		len(uses[0].DisjointClassifications()) != 1 ||
		uses[0].DisjointClassifications()[0].Judgement().Kind() != KindClassificationFalse {
		t.Fatal("current admission certificate lost its direct true/false classifications")
	}
	if _, historical := valid.AdmissionBatch().Basis().(ContextSliceMembershipBasis); historical {
		t.Fatal("current validation emitted a historical MemberOf admission basis")
	}
	if len(fixture.environment.EntitySetDefinitions()) != 0 ||
		len(fixture.environment.KindSignatureDefinitions()) != 0 {
		t.Fatal("current validation environment requires historical EntitySet/MemberOf declarations")
	}
}

func TestValidateMemoryChangeSetKeepsCurrentFalseAndUnknownDistinct(t *testing.T) {
	t.Run("false is invalid", func(t *testing.T) {
		fixture := newCurrentClassificationValidationFixture(t)
		fixture.snapshot.kindClassificationEvaluator = func(
			request KindClassificationRequest,
		) KindClassificationJudgement {
			return validationTestSettledClassification(
				t,
				fixture.environment,
				request,
				KindClassificationFalse,
			)
		}
		verdict := ValidateMemoryChangeSet(
			fixture.environment,
			fixture.registry,
			fixture.snapshot,
			fixture.changeSet,
		)
		assertValidationDiagnostic(
			t,
			verdict,
			ValidationInvalid,
			DiagnosticEntityKindMismatch,
		)
	})

	t.Run("unknown is underdetermined", func(t *testing.T) {
		fixture := newCurrentClassificationValidationFixture(t)
		fixture.snapshot.kindClassificationEvaluator = func(
			request KindClassificationRequest,
		) KindClassificationJudgement {
			return validationTestUnknownClassification(
				t,
				request,
				KindUnknownDependencyUnavailable,
				"restore-kind-standard-dependency",
			)
		}
		verdict := ValidateMemoryChangeSet(
			fixture.environment,
			fixture.registry,
			fixture.snapshot,
			fixture.changeSet,
		)
		assertValidationDiagnostic(
			t,
			verdict,
			ValidationUnderdetermined,
			DiagnosticTypeRuleUnavailable,
		)
	})
}

func TestValidateMemoryChangeSetRequiresDirectFalseForCurrentDisjointKind(t *testing.T) {
	t.Run("unknown counter remains underdetermined", func(t *testing.T) {
		fixture := newCurrentClassificationValidationFixture(t)
		fixture.snapshot.kindClassificationEvaluator = func(
			request KindClassificationRequest,
		) KindClassificationJudgement {
			if request.LocalKind().ValueKind() == fixture.typeEnv.entityValueKind {
				return validationTestSettledClassification(
					t,
					fixture.environment,
					request,
					KindClassificationTrue,
				)
			}
			return validationTestUnknownClassification(
				t,
				request,
				KindUnknownMissingGovernedFeature,
				"deliver-disjoint-kind-features",
			)
		}
		verdict := ValidateMemoryChangeSet(
			fixture.environment,
			fixture.registry,
			fixture.snapshot,
			fixture.changeSet,
		)
		assertValidationDiagnostic(
			t,
			verdict,
			ValidationUnderdetermined,
			DiagnosticTypeRuleUnavailable,
		)
	})

	t.Run("true counter is a contradiction", func(t *testing.T) {
		fixture := newCurrentClassificationValidationFixture(t)
		fixture.snapshot.kindClassificationEvaluator = func(
			request KindClassificationRequest,
		) KindClassificationJudgement {
			return validationTestSettledClassification(
				t,
				fixture.environment,
				request,
				KindClassificationTrue,
			)
		}
		verdict := ValidateMemoryChangeSet(
			fixture.environment,
			fixture.registry,
			fixture.snapshot,
			fixture.changeSet,
		)
		assertValidationDiagnostic(
			t,
			verdict,
			ValidationInvalid,
			DiagnosticEntityKindMismatch,
		)
	})
}

func TestValidateMemoryChangeSetRejectsUncorrelatedCurrentClassification(t *testing.T) {
	fixture := newCurrentClassificationValidationFixture(t)
	other := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.Entity",
		SignatureF4,
		true,
	)
	fixture.snapshot.kindClassificationEvaluator = func(
		KindClassificationRequest,
	) KindClassificationJudgement {
		return validationTestSettledClassification(
			t,
			fixture.environment,
			other.request,
			KindClassificationTrue,
		)
	}
	verdict := ValidateMemoryChangeSet(
		fixture.environment,
		fixture.registry,
		fixture.snapshot,
		fixture.changeSet,
	)
	assertValidationDiagnostic(
		t,
		verdict,
		ValidationInvalid,
		DiagnosticTypeRuleUnavailable,
	)
}

func newCurrentClassificationValidationFixture(t *testing.T) validationFixture {
	t.Helper()
	fixture := newValidationFixture(t)
	required := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.Entity",
		SignatureF4,
		true,
	)
	counter := newKindClassificationFixture(
		t,
		"ctx:haft",
		"U.ClaimGraph",
		SignatureF4,
		true,
	)
	environment, err := fixture.typeEnv.builderWithoutBridge().
		AddKindClassificationSignatureDefinition(required.signature).
		AddKindClassificationSignatureDefinition(counter.signature).
		Build()
	if err != nil {
		t.Fatalf("Build(current classification environment) error = %v", err)
	}
	fixture.environment = environment
	fixture.snapshot.typeEnv = environment.Ref()
	fixture.snapshot.kindClassificationJudgements = map[string]KindClassificationJudgement{}
	return fixture
}

func validationTestSettledClassification(
	t *testing.T,
	environment TypeEnv,
	request KindClassificationRequest,
	kind KindClassificationJudgementKind,
) KindClassificationJudgement {
	t.Helper()
	signature, found := environment.KindClassificationSignatureDefinition(
		request.LocalKind(),
	)
	if !found {
		t.Fatalf("current signature not found for %s", request.LocalKind().String())
	}
	featureValue := kindClassificationVerifiedValue(
		t,
		typeEnvTestValueKindRef(
			t,
			environment.Ref(),
			typeEnvTestKindID(t, "U.ClaimGraph"),
		),
	)
	feature, err := NewGovernedCandidateFeature(GovernedCandidateFeatureInput{
		Key:          mustKindFeatureKey(t, "validation.registry-status"),
		Value:        featureValue,
		Governor:     signature.Criterion(),
		Source:       mustKindCarrierRef(t, "record:validation/current-classification"),
		SourceDigest: mustKindDigest(t, 0xd1),
	})
	if err != nil {
		t.Fatalf("NewGovernedCandidateFeature() error = %v", err)
	}
	features, err := NewGovernedCandidateFeatureSet(
		request,
		[]GovernedCandidateFeature{feature},
	)
	if err != nil {
		t.Fatalf("NewGovernedCandidateFeatureSet() error = %v", err)
	}
	basis, err := NewKindClassificationEvaluationBasis(
		request,
		signature,
		features,
	)
	if err != nil {
		t.Fatalf("NewKindClassificationEvaluationBasis() error = %v", err)
	}
	switch kind {
	case KindClassificationTrue:
		judgement, judgementErr := NewTrueKindClassification(request, basis)
		if judgementErr != nil {
			t.Fatalf("NewTrueKindClassification() error = %v", judgementErr)
		}
		return judgement
	case KindClassificationFalse:
		judgement, judgementErr := NewFalseKindClassification(request, basis)
		if judgementErr != nil {
			t.Fatalf("NewFalseKindClassification() error = %v", judgementErr)
		}
		return judgement
	default:
		t.Fatalf("unsupported settled classification kind %q", kind.String())
		return nil
	}
}

func validationTestUnknownClassification(
	t *testing.T,
	request KindClassificationRequest,
	kind KindClassificationUnknownReasonKind,
	repairRaw string,
) UnknownKindClassification {
	t.Helper()
	repair, err := NewRepairPointer(repairRaw)
	if err != nil {
		t.Fatalf("NewRepairPointer() error = %v", err)
	}
	reason, err := NewKindClassificationUnknownReason(kind, repair)
	if err != nil {
		t.Fatalf("NewKindClassificationUnknownReason() error = %v", err)
	}
	judgement, err := NewUnknownKindClassification(
		request,
		[]KindClassificationUnknownReason{reason},
	)
	if err != nil {
		t.Fatalf("NewUnknownKindClassification() error = %v", err)
	}
	return judgement
}
