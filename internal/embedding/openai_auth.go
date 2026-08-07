package embedding

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// resolveOpenAIAPIKey returns a direct OpenAI API key suitable for platform
// endpoints such as embeddings. Codex/ChatGPT OAuth tokens are intentionally
// excluded because they are scoped to the responses backend.
func resolveOpenAIAPIKey() (string, error) {
	key := os.Getenv("OPENAI_API_KEY")
	if key != "" {
		return key, nil
	}

	key, exists := loadLegacyHaftOpenAIAPIKey()
	if exists {
		return key, nil
	}

	return "", fmt.Errorf(
		"no OpenAI API key found: set OPENAI_API_KEY",
	)
}

func loadLegacyHaftOpenAIAPIKey() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", false
	}

	path := filepath.Join(home, ".config", "haft", "auth.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}

	var auth struct {
		Key string `json:"api_key,omitempty"`
	}
	err = json.Unmarshal(data, &auth)
	if err != nil || auth.Key == "" {
		return "", false
	}

	return auth.Key, true
}
