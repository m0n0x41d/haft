package cmd

import (
	"testing"
)

func TestParseStringArrayFromArgs(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		key  string
		want []string
	}{
		{
			name: "parsed array",
			args: map[string]interface{}{
				"files": []interface{}{"a.go", "b.go"},
			},
			key:  "files",
			want: []string{"a.go", "b.go"},
		},
		{
			name: "JSON string array",
			args: map[string]interface{}{
				"files": `["a.go","b.go"]`,
			},
			key:  "files",
			want: []string{"a.go", "b.go"},
		},
		{
			name: "missing key",
			args: map[string]interface{}{},
			key:  "files",
			want: nil,
		},
		{
			name: "non-array string",
			args: map[string]interface{}{
				"files": "not an array",
			},
			key:  "files",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseStringArrayFromArgs(tt.args, tt.key)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseVariants(t *testing.T) {
	tests := []struct {
		name      string
		args      map[string]interface{}
		wantCount int
		wantTitle string // first variant title
	}{
		{
			name: "parsed array of objects",
			args: map[string]interface{}{
				"variants": []interface{}{
					map[string]interface{}{
						"title":        "Iterative solver",
						"weakest_link": "convergence",
						"description":  "Krylov method",
					},
					map[string]interface{}{
						"title":        "Direct factorization",
						"weakest_link": "memory",
					},
				},
			},
			wantCount: 2,
			wantTitle: "Iterative solver",
		},
		{
			name: "JSON string of variant objects",
			args: map[string]interface{}{
				"variants": `[{"title":"Iterative solver","weakest_link":"convergence","description":"Krylov method"},{"title":"Direct factorization","weakest_link":"memory"}]`,
			},
			wantCount: 2,
			wantTitle: "Iterative solver",
		},
		{
			name: "JSON string with nested arrays",
			args: map[string]interface{}{
				"variants": `[{"title":"V1","weakest_link":"w1","strengths":["fast","cheap"]},{"title":"V2","weakest_link":"w2","risks":["fragile"]}]`,
			},
			wantCount: 2,
			wantTitle: "V1",
		},
		{
			name:      "missing key",
			args:      map[string]interface{}{},
			wantCount: 0,
		},
		{
			name: "non-array string",
			args: map[string]interface{}{
				"variants": "not an array",
			},
			wantCount: 0,
		},
		{
			name: "empty array",
			args: map[string]interface{}{
				"variants": []interface{}{},
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseVariants(tt.args)
			if len(got) != tt.wantCount {
				t.Fatalf("len = %d, want %d", len(got), tt.wantCount)
			}
			if tt.wantTitle != "" && len(got) > 0 && got[0].Title != tt.wantTitle {
				t.Errorf("first title = %q, want %q", got[0].Title, tt.wantTitle)
			}
		})
	}
}

func TestParseVariants_JSONStringPreservesFields(t *testing.T) {
	args := map[string]interface{}{
		"variants": `[
			{
				"title": "Iterative solver with preconditioner caching",
				"description": "Uses Krylov subspace methods with cached preconditioners for saddle-point systems",
				"weakest_link": "Convergence depends on preconditioner quality",
				"strengths": ["Memory efficient", "Scalable to large systems"],
				"risks": ["Slow convergence for ill-conditioned systems"],
				"stepping_stone": true,
				"rollback_notes": "Can fall back to direct solver"
			},
			{
				"title": "Direct factorization with Schur complement",
				"description": "Block LU factorization exploiting saddle-point structure",
				"weakest_link": "O(n^3) memory for dense blocks",
				"strengths": ["Guaranteed solution", "No convergence issues"],
				"risks": ["Memory explosion for large systems"]
			}
		]`,
	}

	got := parseVariants(args)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}

	v1 := got[0]
	if v1.Title != "Iterative solver with preconditioner caching" {
		t.Errorf("title = %q", v1.Title)
	}
	if v1.WeakestLink != "Convergence depends on preconditioner quality" {
		t.Errorf("weakest_link = %q", v1.WeakestLink)
	}
	if len(v1.Strengths) != 2 {
		t.Errorf("strengths len = %d, want 2", len(v1.Strengths))
	}
	if len(v1.Risks) != 1 {
		t.Errorf("risks len = %d, want 1", len(v1.Risks))
	}
	if !v1.SteppingStone {
		t.Error("stepping_stone should be true")
	}
	if v1.RollbackNotes != "Can fall back to direct solver" {
		t.Errorf("rollback_notes = %q", v1.RollbackNotes)
	}

	v2 := got[1]
	if v2.Title != "Direct factorization with Schur complement" {
		t.Errorf("title = %q", v2.Title)
	}
	if v2.SteppingStone {
		t.Error("stepping_stone should be false")
	}
}
