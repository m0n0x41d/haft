package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

func readCommissionPlanPayload(stdin io.Reader, path string) (map[string]any, error) {
	data, err := readCommissionPlanBytes(stdin, path)
	if err != nil {
		return nil, err
	}

	payload := map[string]any{}
	if err := yaml.Unmarshal(data, &payload); err != nil {
		return nil, fmt.Errorf("parse ImplementationPlan %s: %w", path, err)
	}

	return normalizeYAMLMap(payload), nil
}

func readCommissionPlanBytes(stdin io.Reader, path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return data, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

func normalizeYAMLMap(value map[string]any) map[string]any {
	encoded, _ := json.Marshal(value)

	normalized := map[string]any{}
	_ = json.Unmarshal(encoded, &normalized)

	return normalized
}
