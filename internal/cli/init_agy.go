package cli

func init() {
	initCmd.Flags().BoolVar(&initAgy, "agy", false, "Configure Google Antigravity shared MCP config")
}
