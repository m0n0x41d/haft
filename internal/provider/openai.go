package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/responses"
	"github.com/openai/openai-go/v3/shared"

	"github.com/m0n0x41d/haft/internal/agent"
)

// Codex CLI uses /backend-api/codex/responses for ChatGPT-authenticated requests.
const codexAPIEndpoint = "https://chatgpt.com/backend-api/codex"
const codexOriginator = "haft"

// OpenAIProvider implements LLMProvider using the OpenAI chat completions API.
type OpenAIProvider struct {
	client    openai.Client
	model     string
	accountID string // ChatGPT account ID for Codex auth
	authType  string // "api_key" | "codex" | "codex_cli"
}

var _ LLMProvider = (*OpenAIProvider)(nil)

// NewOpenAI creates an OpenAI provider.
// Auth resolution: env var -> haft auth (codex oauth / api key) -> codex CLI token.
func NewOpenAI(model string) (*OpenAIProvider, error) {
	resolved := resolveAuth()
	if resolved.key == "" {
		return nil, fmt.Errorf("no OpenAI auth found: run 'haft login' or set OPENAI_API_KEY")
	}

	opts := []option.RequestOption{option.WithAPIKey(resolved.key)}

	// ChatGPT/Codex auth uses the ChatGPT backend and requires workspace scoping.
	if resolved.authType == "codex" || resolved.authType == "codex_cli" {
		opts = append(opts,
			option.WithBaseURL(codexAPIEndpoint),
			option.WithHeader("originator", codexOriginator),
		)
		if resolved.accountID != "" {
			opts = append(opts, option.WithHeader("chatgpt-account-id", resolved.accountID))
		}
	}

	client := openai.NewClient(opts...)

	return &OpenAIProvider{
		client:    client,
		model:     model,
		accountID: resolved.accountID,
		authType:  resolved.authType,
	}, nil
}

func (p *OpenAIProvider) ModelID() string { return p.model }

// Stream sends conversation to OpenAI and streams the response.
func (p *OpenAIProvider) Stream(
	ctx context.Context,
	messages []agent.Message,
	tools []agent.ToolSchema,
	handler func(StreamDelta),
) (*agent.Message, error) {
	sessionID := extractSessionID(messages)
	params := buildResponseParams(p.model, sessionID, p.authType, messages, tools)
	requestDebug := p.debugRequestPayload(sessionID, params)
	stream := p.client.Responses.NewStreaming(ctx, params, p.requestOptionsFor(sessionID)...)

	var finalResp *responses.Response
	var streamErr error
	var accumulatedText strings.Builder
	var eventTypes []string
	for stream.Next() {
		event := stream.Current()
		eventTypes = append(eventTypes, event.Type)
		switch event.Type {
		case "response.output_text.delta":
			if event.Delta != "" {
				accumulatedText.WriteString(event.Delta)
				handler(StreamDelta{Text: event.Delta})
			}
		case "response.reasoning_summary_text.delta":
			if event.Delta != "" {
				handler(StreamDelta{Thinking: event.Delta})
			}
		case "response.content_part.delta":
			if event.Delta != "" {
				accumulatedText.WriteString(event.Delta)
				handler(StreamDelta{Text: event.Delta})
			}
		case "response.completed":
			resp := event.Response
			finalResp = &resp
		case "error":
			streamErr = fmt.Errorf("openai stream event error: %s", event.Message)
		}
	}
	// Log event types for debugging empty responses
	if accumulatedText.Len() == 0 {
		uniqueTypes := make(map[string]int)
		for _, t := range eventTypes {
			uniqueTypes[t]++
		}
		typeList := make([]string, 0, len(uniqueTypes))
		for t, c := range uniqueTypes {
			typeList = append(typeList, fmt.Sprintf("%s:%d", t, c))
		}
		fmt.Fprintf(os.Stderr, "haft: stream produced no text. Event types: %v\n", typeList)
	}
	if err := stream.Err(); err != nil {
		return nil, formatOpenAIError(err, requestDebug)
	}
	if streamErr != nil {
		return nil, streamErr
	}
	if finalResp == nil {
		return nil, fmt.Errorf("openai responses stream ended without final response")
	}

	msg := responseToMessage(*finalResp)

	// Some models return empty OutputText in the final response.completed
	// event even though text deltas were streamed. Recover the streamed text.
	if msg.Text() == "" && accumulatedText.Len() > 0 {
		msg.Parts = append([]agent.Part{agent.TextPart{Text: accumulatedText.String()}}, msg.Parts...)
	}

	// Debug: log response output details when text is empty
	if msg.Text() == "" && len(msg.ToolCalls()) == 0 {
		outputTypes := make([]string, 0, len(finalResp.Output))
		for _, item := range finalResp.Output {
			detail := item.Type
			if item.Type == "message" {
				detail += fmt.Sprintf("(role=%s,content_len=%d)", item.Role, len(item.Content))
			}
			outputTypes = append(outputTypes, detail)
		}
		fmt.Fprintf(os.Stderr, "haft: empty response. Output items: %v, OutputText: %q, Status: %s\n",
			outputTypes, truncStr(finalResp.OutputText(), 200), finalResp.Status)
	}

	return msg, nil
}

func truncStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// ---------------------------------------------------------------------------
// Converters
// ---------------------------------------------------------------------------

func buildResponseParams(
	model string,
	sessionID string,
	authType string,
	msgs []agent.Message,
	tools []agent.ToolSchema,
) responses.ResponseNewParams {
	instructions, input := splitInstructionsFromMessages(msgs)
	params := responses.ResponseNewParams{
		Model: model,
		Input: responses.ResponseNewParamsInputUnion{
			OfInputItemList: input,
		},
		ParallelToolCalls: openai.Bool(true),
		Store:             openai.Bool(false),
		ToolChoice: responses.ResponseNewParamsToolChoiceUnion{
			OfToolChoiceMode: openai.Opt(responses.ToolChoiceOptionsAuto),
		},
	}
	// MaxOutputTokens not supported by Codex backend — only set for direct API
	if authType == "api_key" {
		maxOut := 16384
		if m, ok := DefaultRegistry().Lookup(model); ok && m.DefaultMaxOut > 0 {
			maxOut = m.DefaultMaxOut
		}
		params.MaxOutputTokens = openai.Int(int64(maxOut))
	}
	if instructions != "" {
		params.Instructions = openai.String(instructions)
	}
	if sessionID != "" {
		params.PromptCacheKey = openai.String(sessionID)
	}
	if len(tools) > 0 {
		params.Tools = convertTools(tools)
	}
	if reasoning, ok := defaultReasoningForModel(model, authType); ok {
		params.Reasoning = reasoning
		params.Include = []responses.ResponseIncludable{
			responses.ResponseIncludableReasoningEncryptedContent,
		}
	}
	return params
}

func splitInstructionsFromMessages(
	msgs []agent.Message,
) (string, []responses.ResponseInputItemUnionParam) {
	var instructionParts []string
	var input []responses.ResponseInputItemUnionParam
	for _, msg := range msgs {
		switch msg.Role {
		case agent.RoleSystem:
			if text := msg.Text(); text != "" {
				instructionParts = append(instructionParts, text)
			}
		default:
			input = append(input, convertMessage(msg)...)
		}
	}
	return strings.Join(instructionParts, "\n\n"), input
}

func convertMessage(msg agent.Message) []responses.ResponseInputItemUnionParam {
	var out []responses.ResponseInputItemUnionParam
	switch msg.Role {
	case agent.RoleUser:
		if content := userInputContent(msg); len(content) > 0 {
			out = append(out, responses.ResponseInputItemParamOfInputMessage(
				content,
				"user",
			))
		}
	case agent.RoleAssistant:
		out = append(out, convertAssistantMessage(msg)...)
	case agent.RoleTool:
		for _, p := range msg.Parts {
			if tr, ok := p.(agent.ToolResultPart); ok {
				out = append(out, responses.ResponseInputItemParamOfFunctionCallOutput(
					tr.ToolCallID,
					tr.Content,
				))
			}
		}
	}
	return out
}

func convertAssistantMessage(msg agent.Message) []responses.ResponseInputItemUnionParam {
	var out []responses.ResponseInputItemUnionParam
	if text := msg.Text(); text != "" {
		// Assistant messages replayed in input must use output_text, not input_text
		content := []responses.ResponseOutputMessageContentUnionParam{
			{OfOutputText: &responses.ResponseOutputTextParam{Text: text}},
		}
		out = append(out, responses.ResponseInputItemParamOfOutputMessage(
			content,
			msg.ID,
			responses.ResponseOutputMessageStatusCompleted,
		))
	}
	for _, tc := range msg.ToolCalls() {
		out = append(out, responses.ResponseInputItemParamOfFunctionCall(
			tc.Arguments,
			tc.ToolCallID,
			tc.ToolName,
		))
	}
	return out
}

func userInputContent(msg agent.Message) responses.ResponseInputMessageContentListParam {
	var content responses.ResponseInputMessageContentListParam
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case agent.TextPart:
			if p.Text != "" {
				content = append(content, responses.ResponseInputContentParamOfInputText(p.Text))
			}
		case agent.ImagePart:
			if len(p.Data) == 0 {
				continue
			}
			dataURL := fmt.Sprintf("data:%s;base64,%s", p.MIMEType, base64.StdEncoding.EncodeToString(p.Data))
			image := responses.ResponseInputContentParamOfInputImage(responses.ResponseInputImageDetailAuto)
			image.OfInputImage.ImageURL = openai.String(dataURL)
			content = append(content, image)
		}
	}
	return content
}

func convertTools(schemas []agent.ToolSchema) []responses.ToolUnionParam {
	out := make([]responses.ToolUnionParam, len(schemas))
	for i, s := range schemas {
		tool := responses.FunctionToolParam{
			Name:       s.Name,
			Parameters: s.Parameters,
			Strict:     openai.Bool(false),
		}
		if s.Description != "" {
			tool.Description = openai.String(s.Description)
		}
		out[i] = responses.ToolUnionParam{OfFunction: &tool}
	}
	return out
}

func responseToMessage(resp responses.Response) *agent.Message {
	msg := &agent.Message{
		Role:      agent.RoleAssistant,
		Model:     resp.Model,
		CreatedAt: time.Now().UTC(),
	}
	// OutputText() concatenates all text from output items — use it as the
	// single source of truth. Don't also extract from "message" items or
	// text gets doubled.
	if text := resp.OutputText(); text != "" {
		msg.Parts = append(msg.Parts, agent.TextPart{Text: text})
	}
	for _, item := range resp.Output {
		if item.Type == "function_call" {
			call := item.AsFunctionCall()
			msg.Parts = append(msg.Parts, agent.ToolCallPart{
				ToolCallID: call.CallID,
				ToolName:   call.Name,
				Arguments:  call.Arguments,
			})
		}
	}
	msg.Tokens = int(resp.Usage.TotalTokens)

	// Debug: log if response has output items but no parts extracted
	if len(msg.Parts) == 0 && len(resp.Output) > 0 {
		types := make([]string, 0, len(resp.Output))
		for _, item := range resp.Output {
			types = append(types, item.Type)
		}
		fmt.Fprintf(os.Stderr, "haft: warning: %d output items but no parts extracted (types: %v)\n", len(resp.Output), types)
	}

	return msg
}

func extractSessionID(msgs []agent.Message) string {
	for _, msg := range msgs {
		if msg.SessionID != "" {
			return msg.SessionID
		}
	}
	return ""
}

func (p *OpenAIProvider) requestOptionsFor(sessionID string) []option.RequestOption {
	opts := []option.RequestOption{
		option.WithHeader("Accept", "text/event-stream"),
	}
	if sessionID != "" {
		opts = append(opts,
			option.WithHeader("session_id", sessionID),
			option.WithHeader("x-client-request-id", sessionID),
		)
	}
	if p.authType != "codex" && p.authType != "codex_cli" {
		return opts
	}

	opts = append(opts,
		option.WithHeader("User-Agent", codexUserAgent()),
		option.WithHeaderDel("X-Stainless-Lang"),
		option.WithHeaderDel("X-Stainless-Package-Version"),
		option.WithHeaderDel("X-Stainless-OS"),
		option.WithHeaderDel("X-Stainless-Arch"),
		option.WithHeaderDel("X-Stainless-Runtime"),
		option.WithHeaderDel("X-Stainless-Runtime-Version"),
		option.WithHeaderDel("X-Stainless-Retry-Count"),
		option.WithHeaderDel("X-Stainless-Timeout"),
	)
	return opts
}

func formatOpenAIError(err error, requestDebug string) error {
	// Write full debug to file for inspection
	writeDebugLog(err, requestDebug)

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		raw := strings.TrimSpace(apiErr.RawJSON())
		if raw != "" {
			return fmt.Errorf("openai error (%d): %s", apiErr.StatusCode, truncateDebugString(raw))
		}
		return fmt.Errorf("openai error (%d): %s", apiErr.StatusCode, truncateDebugString(apiErr.Error()))
	}
	return fmt.Errorf("openai stream: %w", err)
}

func writeDebugLog(err error, requestDebug string) {
	home, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return
	}
	logDir := filepath.Join(home, ".config", "haft")
	_ = os.MkdirAll(logDir, 0o700)
	logPath := filepath.Join(logDir, "agent-debug.log")

	var buf strings.Builder
	fmt.Fprintf(&buf, "=== %s ===\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(&buf, "error: %s\n", err.Error())

	var apiErr *openai.Error
	if errors.As(err, &apiErr) {
		fmt.Fprintf(&buf, "status: %d\n", apiErr.StatusCode)
		fmt.Fprintf(&buf, "raw_json: %s\n", apiErr.RawJSON())
		fmt.Fprintf(&buf, "response_dump:\n%s\n", string(apiErr.DumpResponse(true)))
	}
	fmt.Fprintf(&buf, "request:\n%s\n\n", requestDebug)

	f, fErr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if fErr != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(buf.String())
}

func defaultReasoningForModel(model, _ string) (shared.ReasoningParam, bool) {
	if !isReasoningModel(model) {
		return shared.ReasoningParam{}, false
	}

	effort := shared.ReasoningEffortMedium
	if strings.HasPrefix(model, "gpt-5-pro") {
		effort = shared.ReasoningEffortHigh
	}

	// Always request concise summary. "auto" allows OpenAI to skip summaries
	// entirely, which produces zero streaming deltas — the model thinks for
	// thousands of tokens but nothing is visible to the user.
	summary := shared.ReasoningSummaryConcise

	return shared.ReasoningParam{
		Effort:  effort,
		Summary: summary,
	}, true
}

func isReasoningModel(model string) bool {
	// Prefix heuristic first (fast, no network, deterministic)
	for _, prefix := range []string{"gpt-5", "o1", "o3", "o4"} {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}
	// Then check registry for non-obvious models
	if m, ok := DefaultRegistry().Lookup(model); ok {
		return m.CanReason
	}
	return false
}

func (p *OpenAIProvider) debugRequestPayload(
	sessionID string,
	params responses.ResponseNewParams,
) string {
	body := map[string]any{}
	bodyBytes, err := json.Marshal(params)
	if err == nil {
		_ = json.Unmarshal(bodyBytes, &body)
	}
	body["stream"] = true

	payload := map[string]any{
		"url": p.requestURL(),
		"headers": map[string]any{
			"accept":              "text/event-stream",
			"originator":          codexOriginator,
			"user-agent":          codexUserAgent(),
			"session_id":          sessionID,
			"x-client-request-id": sessionID,
		},
		"body": sanitizeDebugValue(body),
	}
	if p.accountID != "" {
		headers := payload["headers"].(map[string]any)
		headers["chatgpt-account-id"] = "[redacted]"
	}
	if p.authType != "codex" && p.authType != "codex_cli" {
		headers := payload["headers"].(map[string]any)
		delete(headers, "originator")
		delete(headers, "user-agent")
		delete(headers, "chatgpt-account-id")
	}

	out, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return `{"error":"failed to encode debug request"}`
	}
	return string(out)
}

func (p *OpenAIProvider) requestURL() string {
	if p.authType == "codex" || p.authType == "codex_cli" {
		return codexAPIEndpoint + "/responses"
	}
	return "https://api.openai.com/v1/responses"
}

func sanitizeDebugValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "chatgpt-account-id" {
				out[key] = "[redacted]"
				continue
			}
			out[key] = sanitizeDebugValue(child)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, sanitizeDebugValue(child))
		}
		return out
	case string:
		return truncateDebugString(v)
	default:
		return value
	}
}

func truncateDebugString(s string) string {
	const maxLen = 240
	s = strings.TrimSpace(s)
	if len(s) <= maxLen {
		return s
	}
	return fmt.Sprintf("%s...(%d chars)", s[:maxLen], len(s))
}

func codexUserAgent() string {
	version := "0.0.0"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	return fmt.Sprintf(
		"%s/%s (%s unknown; %s) haft/%s",
		codexOriginator,
		version,
		normalizedCodexOS(),
		runtime.GOARCH,
		version,
	)
}

func normalizedCodexOS() string {
	switch runtime.GOOS {
	case "darwin":
		return "MacOS"
	case "linux":
		return "Linux"
	case "windows":
		return "Windows"
	default:
		return runtime.GOOS
	}
}

// ---------------------------------------------------------------------------
// Auth resolution
// ---------------------------------------------------------------------------

type resolvedAuth struct {
	key       string
	authType  string // "api_key" | "codex" | "codex_cli"
	accountID string
}

// ResolveOpenAIAPIKey returns a direct OpenAI API key suitable for platform
// endpoints such as embeddings. Codex/ChatGPT OAuth tokens are intentionally
// excluded because they are scoped to the responses backend, not the platform
// embeddings API.
func ResolveOpenAIAPIKey() (string, error) {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key, nil
	}

	if auth := loadHaftAuthFile(); auth != nil && auth.Key != "" {
		return auth.Key, nil
	}

	return "", fmt.Errorf("no OpenAI API key found: set OPENAI_API_KEY or run 'haft login' with an API key")
}

func resolveAuth() resolvedAuth {
	// 1. Env var
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return resolvedAuth{key: key, authType: "api_key"}
	}

	// 2. Quint's own auth file (from `haft login`)
	if auth := loadHaftAuthFile(); auth != nil {
		// Prefer Codex OAuth (free with ChatGPT sub)
		if auth.CodexAccess != "" {
			// Check if token needs refresh
			if auth.CodexExpires > 0 && time.Now().Unix() > auth.CodexExpires && auth.CodexRefresh != "" {
				refreshed := refreshCodexToken(auth.CodexRefresh)
				if refreshed != nil {
					return resolvedAuth{
						key:       refreshed.key,
						authType:  "codex",
						accountID: auth.AccountID,
					}
				}
			}
			return resolvedAuth{
				key:       auth.CodexAccess,
				authType:  "codex",
				accountID: auth.AccountID,
			}
		}
		if auth.Key != "" {
			return resolvedAuth{key: auth.Key, authType: "api_key"}
		}
	}

	// 3. Codex CLI token (~/.codex/auth.json)
	if cliAuth := readCodexCLIAuth(); cliAuth.key != "" {
		return cliAuth
	}

	return resolvedAuth{}
}

type haftAuthFile struct {
	Key          string `json:"api_key,omitempty"`
	CodexAccess  string `json:"codex_access_token,omitempty"`
	CodexRefresh string `json:"codex_refresh_token,omitempty"`
	CodexExpires int64  `json:"codex_expires_at,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
}

func loadHaftAuthFile() *haftAuthFile {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".config", "haft", "auth.json"))
	if err != nil {
		return nil
	}
	var auth haftAuthFile
	if json.Unmarshal(data, &auth) != nil {
		return nil
	}
	return &auth
}

func readCodexCLIAuth() resolvedAuth {
	home, err := os.UserHomeDir()
	if err != nil {
		return resolvedAuth{}
	}
	data, err := os.ReadFile(filepath.Join(home, ".codex", "auth.json"))
	if err != nil {
		return resolvedAuth{}
	}
	var auth struct {
		AuthMode string `json:"auth_mode"`
		Tokens   struct {
			AccessToken  string `json:"access_token"`
			RefreshToken string `json:"refresh_token"`
			AccountID    string `json:"account_id"`
			IDToken      string `json:"id_token"`
		} `json:"tokens"`
	}
	if json.Unmarshal(data, &auth) != nil {
		return resolvedAuth{}
	}
	if auth.Tokens.AccessToken == "" {
		return resolvedAuth{}
	}
	accountID := auth.Tokens.AccountID
	if accountID == "" {
		accountID = extractAccountIDFromJWT(auth.Tokens.IDToken)
	}
	return resolvedAuth{
		key:       auth.Tokens.AccessToken,
		authType:  "codex_cli",
		accountID: accountID,
	}
}

func refreshCodexToken(refreshToken string) *resolvedAuth {
	data := strings.NewReader(fmt.Sprintf(
		"grant_type=refresh_token&refresh_token=%s&client_id=app_EMoamEEZ73f0CkXaXp7hrann",
		refreshToken,
	))
	resp, err := http.Post("https://auth.openai.com/oauth/token", "application/x-www-form-urlencoded", data)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	var tokens struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if json.NewDecoder(resp.Body).Decode(&tokens) != nil {
		return nil
	}

	// Update stored tokens
	auth := loadHaftAuthFile()
	if auth != nil {
		auth.CodexAccess = tokens.AccessToken
		if tokens.RefreshToken != "" {
			auth.CodexRefresh = tokens.RefreshToken
		}
		if auth.AccountID == "" {
			auth.AccountID = extractAccountIDFromJWT(tokens.AccessToken)
		}
		expiresIn := tokens.ExpiresIn
		if expiresIn == 0 {
			expiresIn = 3600
		}
		auth.CodexExpires = time.Now().Unix() + int64(expiresIn)
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".config", "haft", "auth.json")
		updated, _ := json.MarshalIndent(auth, "", "  ")
		_ = os.WriteFile(path, updated, 0o600)
	}

	return &resolvedAuth{
		key:       tokens.AccessToken,
		authType:  "codex",
		accountID: extractAccountIDFromJWT(tokens.AccessToken),
	}
}

func extractAccountIDFromJWT(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
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

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	if json.Unmarshal(payload, &claims) != nil {
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
