package cli

func init() {
	initCmd.Flags().BoolVar(
		&initGrok,
		"grok",
		false,
		"Configure for Grok CLI — project .grok/config.toml MCP + .grok/skills",
	)
}
