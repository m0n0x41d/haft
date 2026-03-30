package agent

// BuildTitlePrompt creates a prompt for session title generation.
// The coordinator calls this after the first assistant response and sends
// it to the LLM as a lightweight async request.
// Pure function.
func BuildTitlePrompt(userMessage string) string {
	return `Generate a short title (under 50 characters) for a coding session that started with this message:

"` + truncateForTitle(userMessage, 500) + `"

Rules:
- Under 50 characters
- No quotes or colons
- Match the language of the message
- Describe the task, not the greeting
- Single line, no explanation

Title:`
}

func truncateForTitle(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
