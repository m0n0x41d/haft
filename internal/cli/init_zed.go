package cli

const zedContextServerName = "Haft"

func init() {
	initCmd.Flags().BoolVar(&initZed, "zed", false, "Configure Zed MCP context server")
}
