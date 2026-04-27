package workcommission

import "testing"

func TestProjectionValidationRejectsInventedAuthorityClaims(t *testing.T) {
	intent := ProjectionIntent{
		RequiredLinks: []string{"haft://commission/wc-1"},
	}

	for _, kind := range AuthorityProjectionClaimKinds() {
		validation := ValidateProjectionDraft(intent, ProjectionDraft{
			Claims: []ProjectionClaim{
				{Kind: kind, Value: "invented"},
			},
			Links: []string{"haft://commission/wc-1"},
		})

		if validation.Verdict != ProjectionValidationReject {
			t.Fatalf("%s verdict = %s, want reject", kind, validation.Verdict)
		}
		if len(validation.Issues) == 0 {
			t.Fatalf("%s validation issues empty, want invented claim", kind)
		}
		if validation.Issues[0].Code != ProjectionValidationInventedClaim {
			t.Fatalf("%s issue = %#v, want invented claim", kind, validation.Issues[0])
		}
	}
}

func TestProjectionValidationPassesClosedIntentFactsAndLinks(t *testing.T) {
	intent := ProjectionIntent{
		RequiredClaims: []ProjectionClaim{
			{Kind: ProjectionClaimStatus, Value: "blocked_policy"},
		},
		OptionalClaims: []ProjectionClaim{
			{Kind: ProjectionClaimCompletion, Value: "not completed"},
		},
		RequiredLinks: []string{"haft://commission/wc-1"},
	}

	validation := ValidateProjectionDraft(intent, ProjectionDraft{
		Claims: []ProjectionClaim{
			{Kind: ProjectionClaimStatus, Value: "blocked_policy"},
			{Kind: ProjectionClaimCompletion, Value: "not completed"},
		},
		Links: []string{"haft://commission/wc-1"},
	})

	if validation.Verdict != ProjectionValidationPass {
		t.Fatalf("validation = %#v, want pass", validation)
	}
}

func TestProjectionValidationRejectsMissingRequiredFactsAndLinks(t *testing.T) {
	intent := ProjectionIntent{
		RequiredClaims: []ProjectionClaim{
			{Kind: ProjectionClaimStatus, Value: "running"},
		},
		RequiredLinks: []string{"haft://decision/dec-1"},
	}

	validation := ValidateProjectionDraft(intent, ProjectionDraft{})

	if validation.Verdict != ProjectionValidationReject {
		t.Fatalf("verdict = %s, want reject", validation.Verdict)
	}
	if len(validation.Issues) != 2 {
		t.Fatalf("issues = %#v, want missing claim and missing link", validation.Issues)
	}
}

func TestCompletionAfterLocalEvidenceSeparatesProjectionDebt(t *testing.T) {
	localOnly := CompletionAfterLocalEvidence(
		ProjectionPolicyLocalOnly,
		ProjectionPublication{State: ProjectionPublicationMissing},
	)
	if localOnly.State != StateCompleted || localOnly.Debt != nil {
		t.Fatalf("localOnly = %#v, want completed without debt", localOnly)
	}

	requiredMissing := CompletionAfterLocalEvidence(
		ProjectionPolicyExternalRequired,
		ProjectionPublication{State: ProjectionPublicationMissing},
	)
	if requiredMissing.State != StateCompletedWithProjectionDebt {
		t.Fatalf("requiredMissing state = %s, want projection debt", requiredMissing.State)
	}
	if requiredMissing.Debt == nil {
		t.Fatal("requiredMissing debt nil, want explicit debt")
	}
	if requiredMissing.Debt.LastError != "external publication has not synced" {
		t.Fatalf("last_error = %q", requiredMissing.Debt.LastError)
	}

	requiredSynced := CompletionAfterLocalEvidence(
		ProjectionPolicyExternalRequired,
		ProjectionPublication{State: ProjectionPublicationSynced},
	)
	if requiredSynced.State != StateCompleted || requiredSynced.Debt != nil {
		t.Fatalf("requiredSynced = %#v, want completed without debt", requiredSynced)
	}
}
