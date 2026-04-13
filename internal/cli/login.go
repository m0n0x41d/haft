package cli

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	codexClientID = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexIssuer   = "https://auth.openai.com"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Configure LLM provider authentication",
	Long: `Set up authentication for the agent's LLM provider.

Methods:
  haft login              # ChatGPT Plus/Pro device flow (recommended)
  haft login --api-key    # enter OpenAI API key manually
  haft login --status     # check current auth status`,
	RunE: runLogin,
}

var (
	loginStatus bool
	loginAPIKey bool
)

func init() {
	loginCmd.Flags().BoolVar(&loginStatus, "status", false, "Show current auth status")
	loginCmd.Flags().BoolVar(&loginAPIKey, "api-key", false, "Enter API key interactively")
	// login is hidden — use 'haft setup' instead. Kept for OAuth flow internals.
	loginCmd.Hidden = true
	rootCmd.AddCommand(loginCmd)
}

func runLogin(_ *cobra.Command, _ []string) error {
	if loginStatus {
		return showAuthStatus()
	}
	if loginAPIKey {
		return promptAPIKey()
	}
	return codexDeviceLogin()
}

// ---------------------------------------------------------------------------
// Codex device flow (headless — no browser callback server needed)
// ---------------------------------------------------------------------------

type deviceAuthResponse struct {
	DeviceAuthID string `json:"device_auth_id"`
	UserCode     string `json:"user_code"`
	Interval     string `json:"interval"`
}

type deviceTokenResponse struct {
	AuthorizationCode string `json:"authorization_code"`
	CodeVerifier      string `json:"code_verifier"`
}

type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func codexDeviceLogin() error {
	fmt.Println("Authenticating with ChatGPT Plus/Pro...")
	fmt.Println()

	// 1. Request device code
	resp, err := http.Post(
		codexIssuer+"/api/accounts/deviceauth/usercode",
		"application/json",
		strings.NewReader(fmt.Sprintf(`{"client_id":"%s"}`, codexClientID)),
	)
	if err != nil {
		return fmt.Errorf("device auth request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device auth failed (%d): %s", resp.StatusCode, string(body))
	}

	var deviceAuth deviceAuthResponse
	if err := json.NewDecoder(resp.Body).Decode(&deviceAuth); err != nil {
		return fmt.Errorf("parse device auth: %w", err)
	}

	// 2. Show user the code
	fmt.Printf("  Open:  %s/codex/device\n", codexIssuer)
	fmt.Printf("  Code:  %s\n", deviceAuth.UserCode)
	fmt.Println()
	fmt.Println("Waiting for authorization...")

	// 3. Poll for token
	interval := 5 * time.Second
	for {
		time.Sleep(interval)

		tokenResp, err := http.Post(
			codexIssuer+"/api/accounts/deviceauth/token",
			"application/json",
			strings.NewReader(fmt.Sprintf(`{"device_auth_id":"%s","user_code":"%s"}`,
				deviceAuth.DeviceAuthID, deviceAuth.UserCode)),
		)
		if err != nil {
			continue
		}

		if tokenResp.StatusCode == http.StatusOK {
			var deviceToken deviceTokenResponse
			_ = json.NewDecoder(tokenResp.Body).Decode(&deviceToken)
			_ = tokenResp.Body.Close()

			// 4. Exchange for OAuth tokens
			tokens, err := exchangeDeviceToken(deviceToken)
			if err != nil {
				return fmt.Errorf("token exchange: %w", err)
			}

			// 5. Extract account ID from JWT
			accountID := extractAccountID(tokens)

			// 6. Store tokens
			if err := storeCodexAuth(tokens, accountID); err != nil {
				return fmt.Errorf("store tokens: %w", err)
			}

			fmt.Println()
			fmt.Println("Authenticated successfully.")
			fmt.Println("Run: haft agent")
			return nil
		}

		_ = tokenResp.Body.Close()

		if tokenResp.StatusCode != http.StatusForbidden && tokenResp.StatusCode != http.StatusNotFound {
			return fmt.Errorf("unexpected polling status: %d", tokenResp.StatusCode)
		}
	}
}

func exchangeDeviceToken(device deviceTokenResponse) (*oauthTokenResponse, error) {
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {device.AuthorizationCode},
		"redirect_uri":  {codexIssuer + "/deviceauth/callback"},
		"client_id":     {codexClientID},
		"code_verifier": {device.CodeVerifier},
	}

	resp, err := http.PostForm(codexIssuer+"/oauth/token", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokens oauthTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, err
	}
	return &tokens, nil
}

func extractAccountID(tokens *oauthTokenResponse) string {
	if id := extractAccountIDFromJWT(tokens.IDToken); id != "" {
		return id
	}
	return extractAccountIDFromJWT(tokens.AccessToken)
}

func extractAccountIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	decoded, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var claims struct {
		ChatGPTAccountID string `json:"chatgpt_account_id"`
		Auth             *struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
		Organizations []struct {
			ID string `json:"id"`
		} `json:"organizations"`
	}
	if json.Unmarshal(decoded, &claims) != nil {
		return ""
	}
	if claims.ChatGPTAccountID != "" {
		return claims.ChatGPTAccountID
	}
	if claims.Auth != nil && claims.Auth.ChatGPTAccountID != "" {
		return claims.Auth.ChatGPTAccountID
	}
	if len(claims.Organizations) > 0 {
		return claims.Organizations[0].ID
	}
	return ""
}

// ---------------------------------------------------------------------------
// API key entry
// ---------------------------------------------------------------------------

func promptAPIKey() error {
	fmt.Print("Enter your OpenAI API key: ")
	reader := bufio.NewReader(os.Stdin)
	key, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read input: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("no key provided")
	}

	if err := storeAPIKeyAuth(key); err != nil {
		return err
	}
	fmt.Println("API key stored.")
	fmt.Println("Run: haft agent")
	return nil
}

// ---------------------------------------------------------------------------
// Status
// ---------------------------------------------------------------------------

func showAuthStatus() error {
	fmt.Println("Authentication status:")
	fmt.Println()

	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		fmt.Printf("  OPENAI_API_KEY    %s (env var)\n", maskToken(key))
	} else {
		fmt.Println("  OPENAI_API_KEY    not set")
	}

	auth := loadHaftAuth()
	if auth != nil {
		if auth.CodexAccess != "" {
			expired := ""
			if auth.CodexExpires > 0 && time.Now().Unix() > auth.CodexExpires {
				expired = " (expired — run 'haft login' to refresh)"
			}
			fmt.Printf("  Codex OAuth       %s%s\n", maskToken(auth.CodexAccess), expired)
		}
		if auth.Key != "" {
			fmt.Printf("  API key           %s (stored)\n", maskToken(auth.Key))
		}
	}

	if auth == nil || (auth.CodexAccess == "" && auth.Key == "") {
		fmt.Println("  Haft auth        not configured — run 'haft login'")
	}

	return nil
}

func maskToken(s string) string {
	if len(s) < 12 {
		return s[:min(4, len(s))] + "..."
	}
	return s[:8] + "..." + s[len(s)-4:]
}

// ---------------------------------------------------------------------------
// Auth storage — ~/.config/haft/auth.json
// ---------------------------------------------------------------------------

type haftAuth struct {
	Key          string `json:"api_key,omitempty"`
	CodexAccess  string `json:"codex_access_token,omitempty"`
	CodexRefresh string `json:"codex_refresh_token,omitempty"`
	CodexExpires int64  `json:"codex_expires_at,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
}

func haftAuthPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "haft", "auth.json")
}

func loadHaftAuth() *haftAuth {
	data, err := os.ReadFile(haftAuthPath())
	if err != nil {
		return nil
	}
	var auth haftAuth
	if json.Unmarshal(data, &auth) != nil {
		return nil
	}
	return &auth
}

func saveHaftAuth(auth *haftAuth) error {
	path := haftAuthPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, _ := json.MarshalIndent(auth, "", "  ")
	return os.WriteFile(path, data, 0o600)
}

func storeCodexAuth(tokens *oauthTokenResponse, accountID string) error {
	auth := loadHaftAuth()
	if auth == nil {
		auth = &haftAuth{}
	}
	auth.CodexAccess = tokens.AccessToken
	auth.CodexRefresh = tokens.RefreshToken
	auth.AccountID = accountID
	expiresIn := tokens.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600
	}
	auth.CodexExpires = time.Now().Unix() + int64(expiresIn)
	return saveHaftAuth(auth)
}

func storeAPIKeyAuth(key string) error {
	auth := loadHaftAuth()
	if auth == nil {
		auth = &haftAuth{}
	}
	auth.Key = key
	return saveHaftAuth(auth)
}
