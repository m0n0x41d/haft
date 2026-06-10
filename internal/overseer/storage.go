package overseer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	latestRunFile         = "latest-run.json"
	latestMaintenanceFile = "latest-maintenance.json"
)

type StoredRun struct {
	Run    ReviewRun `json:"run"`
	Packet Packet    `json:"packet"`
}

type Reminder struct {
	HasReminder bool   `json:"has_reminder"`
	Message     string `json:"message"`
	ReviewRunID string `json:"review_run_id,omitempty"`
	PacketID    string `json:"packet_id,omitempty"`
	RiskLevel   string `json:"risk_level,omitempty"`
	Command     string `json:"command,omitempty"`
}

func NewDeterministicReviewRun(packet Packet, createdAt string) ReviewRun {
	return ReviewRun{
		SchemaVersion: ReviewResultSchemaVersion,
		ReviewRunID:   "rrun-" + packetHashSuffix(packet),
		PacketID:      packet.PacketID,
		PacketHash:    packet.PacketHash,
		CreatedAt:     strings.TrimSpace(createdAt),
		Mode:          "deterministic_packet",
		Budget:        packet.ContextBudget,
		Reviewer: Reviewer{
			Agent:                   DefaultToolName,
			ModelOrRuntime:          "deterministic_cli",
			SessionRelationToAuthor: "not_applicable",
			InputSources:            []string{"git_commit", "artifact_graph", "workflow_policy"},
		},
		Authority: DefaultReviewAuthority(),
		Verdict:   "packet_generated",
		ScopeCoverage: ScopeCoverage{
			ModesReviewed: []string{},
			FilesReviewed: packetPaths(packet),
			FetchesUsed:   []string{},
			Abstentions:   []string{"llm_review_disabled"},
		},
		Findings:    []ReviewFinding{},
		NonFindings: []NonFinding{},
	}
}

func packetHashSuffix(packet Packet) string {
	hash := strings.TrimPrefix(packet.PacketHash, "sha256:")
	if len(hash) >= 16 {
		return hash[:16]
	}
	if hash != "" {
		return hash
	}
	return strings.TrimPrefix(packet.PacketID, "rpkt-")
}

func StoreRun(projectRoot string, packet Packet, run ReviewRun) error {
	baseDir := OverseerDir(projectRoot)
	packetPath := filepath.Join(baseDir, "packets", packet.PacketID+".json")
	runPath := filepath.Join(baseDir, "runs", run.ReviewRunID+".json")
	latestPath := filepath.Join(baseDir, latestRunFile)

	if err := os.MkdirAll(filepath.Dir(packetPath), 0o755); err != nil {
		return fmt.Errorf("create packet dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(runPath), 0o755); err != nil {
		return fmt.Errorf("create run dir: %w", err)
	}

	if err := writeJSONFile(packetPath, packet); err != nil {
		return fmt.Errorf("write packet: %w", err)
	}
	if err := writeJSONFile(runPath, run); err != nil {
		return fmt.Errorf("write review run: %w", err)
	}
	if err := writeJSONFile(latestPath, StoredRun{Run: run, Packet: packet}); err != nil {
		return fmt.Errorf("write latest review run: %w", err)
	}
	return nil
}

func StoreMaintenanceRun(projectRoot string, run MaintenanceRun) error {
	baseDir := OverseerDir(projectRoot)
	runPath := filepath.Join(baseDir, "maintenance", run.MaintenanceID+".json")
	latestPath := filepath.Join(baseDir, latestMaintenanceFile)

	if err := os.MkdirAll(filepath.Dir(runPath), 0o755); err != nil {
		return fmt.Errorf("create maintenance dir: %w", err)
	}
	if err := writeJSONFile(runPath, run); err != nil {
		return fmt.Errorf("write maintenance run: %w", err)
	}
	if err := writeJSONFile(latestPath, run); err != nil {
		return fmt.Errorf("write latest maintenance run: %w", err)
	}
	return nil
}

func LoadRun(projectRoot string, runID string) (StoredRun, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" || runID == "latest" {
		return LoadLatestRun(projectRoot)
	}

	var run ReviewRun
	runPath := filepath.Join(OverseerDir(projectRoot), "runs", runID+".json")
	if err := readJSONFile(runPath, &run); err != nil {
		return StoredRun{}, fmt.Errorf("read review run %s: %w", runID, err)
	}

	var packet Packet
	packetPath := filepath.Join(OverseerDir(projectRoot), "packets", run.PacketID+".json")
	if err := readJSONFile(packetPath, &packet); err != nil {
		return StoredRun{}, fmt.Errorf("read packet %s: %w", run.PacketID, err)
	}

	return StoredRun{Run: run, Packet: packet}, nil
}

func LoadLatestRun(projectRoot string) (StoredRun, error) {
	var stored StoredRun
	path := filepath.Join(OverseerDir(projectRoot), latestRunFile)
	if err := readJSONFile(path, &stored); err != nil {
		return StoredRun{}, err
	}
	return stored, nil
}

func LoadMaintenanceRun(projectRoot string, maintenanceID string) (MaintenanceRun, error) {
	maintenanceID = strings.TrimSpace(maintenanceID)
	if maintenanceID == "" || maintenanceID == "latest" {
		return LoadLatestMaintenanceRun(projectRoot)
	}

	var run MaintenanceRun
	path := filepath.Join(OverseerDir(projectRoot), "maintenance", maintenanceID+".json")
	if err := readJSONFile(path, &run); err != nil {
		return MaintenanceRun{}, fmt.Errorf("read maintenance run %s: %w", maintenanceID, err)
	}
	return run, nil
}

func LoadLatestMaintenanceRun(projectRoot string) (MaintenanceRun, error) {
	var run MaintenanceRun
	path := filepath.Join(OverseerDir(projectRoot), latestMaintenanceFile)
	if err := readJSONFile(path, &run); err != nil {
		return MaintenanceRun{}, err
	}
	return run, nil
}

func LoadStatusSummary(projectRoot string) (StatusSummary, error) {
	stored, storedErr := LoadLatestRun(projectRoot)
	hasStored := storedErr == nil
	if storedErr != nil && !os.IsNotExist(storedErr) {
		return StatusSummary{}, storedErr
	}

	maintenance, maintenanceErr := LoadLatestMaintenanceRun(projectRoot)
	hasMaintenance := maintenanceErr == nil
	if maintenanceErr != nil && !os.IsNotExist(maintenanceErr) {
		return StatusSummary{}, maintenanceErr
	}

	return BuildStatusSummary(stored, hasStored, maintenance, hasMaintenance), nil
}

func BuildReminder(stored StoredRun) Reminder {
	if stored.Run.ReviewRunID == "" {
		return Reminder{
			HasReminder: false,
			Message:     "No overseer review run found.",
		}
	}

	unresolved := UnresolvedFindings(stored.Run)
	if len(unresolved) > 0 {
		command := "haft overseer show " + stored.Run.ReviewRunID
		return Reminder{
			HasReminder: true,
			Message: fmt.Sprintf(
				"Overseer has %d unresolved review finding(s) in %s. Inspect them with `%s` before claiming the change is clean.",
				len(unresolved),
				stored.Run.ReviewRunID,
				command,
			),
			ReviewRunID: stored.Run.ReviewRunID,
			PacketID:    stored.Packet.PacketID,
			RiskLevel:   stored.Packet.Risk.Level,
			Command:     command,
		}
	}

	if stored.Run.Verdict != "packet_generated" {
		return Reminder{
			HasReminder: false,
			Message:     "Latest overseer review has no unresolved findings.",
			ReviewRunID: stored.Run.ReviewRunID,
			PacketID:    stored.Packet.PacketID,
			RiskLevel:   stored.Packet.Risk.Level,
		}
	}

	if stored.Packet.Risk.Level == "low" {
		return Reminder{
			HasReminder: false,
			Message:     "Latest overseer packet is low risk.",
			ReviewRunID: stored.Run.ReviewRunID,
			PacketID:    stored.Packet.PacketID,
			RiskLevel:   stored.Packet.Risk.Level,
		}
	}

	command := "haft overseer show " + stored.Run.ReviewRunID
	return Reminder{
		HasReminder: true,
		Message: fmt.Sprintf(
			"Overseer stored a %s risk packet %s. Inspect it with `%s` before claiming the change is clean.",
			stored.Packet.Risk.Level,
			stored.Packet.PacketID,
			command,
		),
		ReviewRunID: stored.Run.ReviewRunID,
		PacketID:    stored.Packet.PacketID,
		RiskLevel:   stored.Packet.Risk.Level,
		Command:     command,
	}
}

func writeJSONFile(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, value)
}

func packetPaths(packet Packet) []string {
	paths := make([]string, 0, len(packet.ChangedFiles))
	for _, file := range packet.ChangedFiles {
		paths = append(paths, file.Path)
	}
	return paths
}
