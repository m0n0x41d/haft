package protocol

import (
	"encoding/json"
	"testing"
)

func TestModeUpdateUnmarshalUsesTypedMode(t *testing.T) {
	t.Helper()

	var update ModeUpdate
	err := json.Unmarshal([]byte(`{"mode":"autonomous","yolo":true}`), &update)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if update.Mode != "autonomous" {
		t.Fatalf("Mode = %q, want autonomous", update.Mode)
	}
	if !update.Yolo {
		t.Fatal("Yolo = false, want true")
	}
}

func TestModeUpdateUnmarshalCanonicalizesLegacyModeLabels(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "legacy symbiotic label",
			json: `{"mode":"symbiotic"}`,
			want: "checkpointed",
		},
		{
			name: "legacy collaborative label",
			json: `{"mode":"collaborative"}`,
			want: "checkpointed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var update ModeUpdate
			err := json.Unmarshal([]byte(tt.json), &update)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if update.Mode != tt.want {
				t.Fatalf("Mode = %q, want %q", update.Mode, tt.want)
			}
		})
	}
}

func TestModeUpdateUnmarshalCanonicalizesLegacyInteractionField(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "legacy symbiotic interaction",
			json: `{"interaction":"symbiotic"}`,
			want: "checkpointed",
		},
		{
			name: "legacy autonomous interaction",
			json: `{"interaction":"autonomous"}`,
			want: "autonomous",
		},
		{
			name: "mode wins over interaction alias",
			json: `{"mode":"checkpointed","interaction":"autonomous"}`,
			want: "checkpointed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var update ModeUpdate
			err := json.Unmarshal([]byte(tt.json), &update)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if update.Mode != tt.want {
				t.Fatalf("Mode = %q, want %q", update.Mode, tt.want)
			}
		})
	}
}

func TestModeUpdateUnmarshalBridgesLegacyAutonomousBool(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		json string
		want string
	}{
		{
			name: "legacy true",
			json: `{"autonomous":true}`,
			want: "autonomous",
		},
		{
			name: "legacy false",
			json: `{"autonomous":false}`,
			want: "checkpointed",
		},
		{
			name: "mode wins over legacy bool",
			json: `{"mode":"checkpointed","autonomous":true}`,
			want: "checkpointed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var update ModeUpdate
			err := json.Unmarshal([]byte(tt.json), &update)
			if err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if update.Mode != tt.want {
				t.Fatalf("Mode = %q, want %q", update.Mode, tt.want)
			}
		})
	}
}

func TestModeUpdateUnmarshalLeavesModeEmptyWhenUnset(t *testing.T) {
	t.Helper()

	var update ModeUpdate
	err := json.Unmarshal([]byte(`{"yolo":true}`), &update)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if update.Mode != "" {
		t.Fatalf("Mode = %q, want empty", update.Mode)
	}
	if !update.Yolo {
		t.Fatal("Yolo = false, want true")
	}
}
