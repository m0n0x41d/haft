package fpf

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/m0n0x41d/quint-code/db"
	"github.com/m0n0x41d/quint-code/logger"

	"github.com/google/uuid"
)

func (t *Tools) GetFPFDir() string {
	return filepath.Join(t.RootDir, ".quint")
}

func (t *Tools) AuditLog(toolName, operation, actor, targetID, result string, input interface{}, details string) {
	if t.DB == nil {
		return
	}

	if actor == "" || actor == "agent" {
		actor = string(GetRoleForTool(toolName))
	}

	var inputHash string
	if input != nil {
		data, err := json.Marshal(input)
		if err == nil {
			hash := sha256.Sum256(data)
			inputHash = hex.EncodeToString(hash[:8])
		}
	}

	id := uuid.New().String()
	ctx := context.Background()
	if err := t.DB.InsertAuditLog(ctx, id, toolName, operation, actor, targetID, inputHash, result, details, "default"); err != nil {
		logger.Warn().Err(err).Msg("failed to insert audit log")
	}
}

func (t *Tools) Slugify(title string) string {
	slug := slugifyRegex.ReplaceAllString(strings.ToLower(title), "-")
	return strings.Trim(slug, "-")
}

func (t *Tools) MoveHypothesis(hypothesisID, sourceLevel, destLevel string) error {
	ctx := context.Background()

	if t.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	holon, err := t.DB.GetHolon(ctx, hypothesisID)
	if err != nil {
		t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "ERROR",
			map[string]string{"from": sourceLevel, "to": destLevel}, "not found in database")
		return fmt.Errorf("hypothesis %s not found", hypothesisID)
	}

	if holon.Layer != sourceLevel {
		return fmt.Errorf("hypothesis %s is in %s, not %s", hypothesisID, holon.Layer, sourceLevel)
	}

	if err := t.DB.UpdateHolonLayer(ctx, hypothesisID, destLevel); err != nil {
		t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "ERROR",
			map[string]string{"from": sourceLevel, "to": destLevel}, err.Error())
		return fmt.Errorf("failed to update layer in database: %w", err)
	}

	t.AuditLog("quint_move", "move_hypothesis", "agent", hypothesisID, "SUCCESS",
		map[string]string{"from": sourceLevel, "to": destLevel}, "")
	return nil
}

func (t *Tools) GetAgentContext(role string) (string, error) {
	filename := strings.ToLower(role) + ".md"
	path := filepath.Join(t.GetFPFDir(), "agents", filename)

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("agent profile for %s not found at %s", role, path)
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(content), nil
}

func (t *Tools) RecordWork(methodName string, start time.Time) {
	if t.DB == nil {
		return
	}
	end := time.Now()
	id := fmt.Sprintf("work-%d", start.UnixNano())

	performer := string(t.FSM.State.ActiveRole.Role)
	if performer == "" {
		performer = "System"
	}

	ledger := fmt.Sprintf(`{"duration_ms": %d}`, end.Sub(start).Milliseconds())
	if err := t.DB.RecordWork(context.Background(), id, methodName, performer, start, end, ledger); err != nil {
		logger.Warn().Err(err).Msg("failed to record work in DB")
	}
}

func (t *Tools) GetHolon(id string) (db.Holon, error) {
	if t.DB == nil {
		return db.Holon{}, fmt.Errorf("DB not initialized")
	}
	return t.DB.GetHolon(context.Background(), id)
}
