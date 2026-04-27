package autonomyenvelope

import (
	"strings"
	"testing"
	"time"
)

func TestEvaluateBlocksOutOfEnvelopeAction(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	snapshot := envelopeFixture(t, now)

	report := snapshot.Evaluate(CommissionRequest{
		RepoRef:        "local:haft",
		AllowedPaths:   []string{"internal/cli/serve_commission.go"},
		AllowedActions: []string{"edit_files", "tag_release"},
	}, now)

	if report.Decision != DecisionBlocked {
		t.Fatalf("decision = %s, want blocked", report.Decision)
	}
	if !reportHasCode(report, "one_way_door_action_forbidden") {
		t.Fatalf("findings = %#v, want one_way_door_action_forbidden", report.Findings)
	}
}

func TestEnvelopeCannotSkipRequiredGates(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)
	payload := envelopePayload(now)
	payload["skip_gates"] = []any{"freshness", "scope", "evidence"}

	_, err := SnapshotFromMap(payload)
	if err == nil {
		t.Fatal("expected skip_gates to be rejected")
	}
	if !strings.Contains(err.Error(), "autonomy_envelope_gate_skip_forbidden") {
		t.Fatalf("err = %v, want gate skip rejection", err)
	}
}

func TestExpiredAndRevokedEnvelopeBlocksExecution(t *testing.T) {
	now := time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name   string
		mutate func(map[string]any)
		code   string
	}{
		{
			name: "expired",
			mutate: func(payload map[string]any) {
				payload["valid_until"] = now.Add(-time.Hour).Format(time.RFC3339)
			},
			code: "autonomy_envelope_expired",
		},
		{
			name: "revoked",
			mutate: func(payload map[string]any) {
				payload["state"] = "revoked"
				payload["revoked_at"] = now.Add(-time.Hour).Format(time.RFC3339)
			},
			code: "autonomy_envelope_revoked",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			payload := envelopePayload(now)
			test.mutate(payload)

			snapshot, err := SnapshotFromMap(payload)
			if err != nil {
				t.Fatal(err)
			}

			report := snapshot.Evaluate(CommissionRequest{
				RepoRef:        "local:haft",
				AllowedPaths:   []string{"internal/cli/serve_commission.go"},
				AllowedActions: []string{"edit_files"},
			}, now)

			if report.Decision != DecisionBlocked {
				t.Fatalf("decision = %s, want blocked", report.Decision)
			}
			if !reportHasCode(report, test.code) {
				t.Fatalf("findings = %#v, want %s", report.Findings, test.code)
			}
		})
	}
}

func TestNoEnvelopeRequiresCheckpointByDefault(t *testing.T) {
	report := NoEnvelopeReport()

	if report.Decision != DecisionCheckpointRequired {
		t.Fatalf("decision = %s, want checkpoint_required", report.Decision)
	}
	if !reportHasCode(report, "autonomy_envelope_missing") {
		t.Fatalf("findings = %#v, want autonomy_envelope_missing", report.Findings)
	}
}

func envelopeFixture(t *testing.T, now time.Time) Snapshot {
	t.Helper()

	snapshot, err := SnapshotFromMap(envelopePayload(now))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func envelopePayload(now time.Time) map[string]any {
	return map[string]any{
		"ref":                            "ae-20260426-001",
		"revision":                       "ae-r1",
		"state":                          "active",
		"allowed_repos":                  []any{"local:haft"},
		"allowed_paths":                  []any{"internal/cli/**"},
		"forbidden_paths":                []any{"internal/cli/migrations/**"},
		"allowed_actions":                []any{"edit_files", "run_tests"},
		"allowed_modules":                []any{"internal/cli"},
		"forbidden_actions":              []any{"delete_data"},
		"forbidden_one_way_door_actions": []any{"tag_release", "merge_pr"},
		"max_concurrency":                float64(2),
		"commission_budget":              float64(3),
		"on_failure":                     "block_node",
		"on_stale":                       "block_node",
		"valid_until":                    now.Add(time.Hour).Format(time.RFC3339),
		"required_gates":                 []any{"freshness", "scope", "evidence"},
	}
}

func reportHasCode(report Report, code string) bool {
	for _, finding := range report.Findings {
		if finding.Code == code {
			return true
		}
	}
	return false
}
