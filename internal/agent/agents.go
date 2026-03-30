package agent

// DefaultAgents returns the built-in agent definitions.
func DefaultAgents() map[string]AgentDef {
	return map[string]AgentDef{
		"haft": HaftAgent(),
		"code": CodeAgent(),
	}
}

// HaftAgent returns the lemniscate-enabled agent definition.
//
// v2: Single react loop with FPF discipline enforced by tool guardrails.
// No phase pipeline. One unified prompt built by BuildSystemPrompt().
// All tools available. Tools refuse when preconditions aren't met.
func HaftAgent() AgentDef {
	return AgentDef{
		Name:       "haft",
		Lemniscate: true,
		// SystemPrompt is empty — BuildSystemPrompt(PromptConfig{Lemniscate: true})
		// produces the complete prompt. No more split across two files.
	}
}

// CodeAgent returns the plain ReAct agent (no lemniscate).
func CodeAgent() AgentDef {
	return AgentDef{
		Name:       "code",
		Lemniscate: false,
	}
}
