package teamleader

// MCPEnvConfig remains only as a compatibility shell for archived MCP wiring.
// Default runtime paths no longer populate or consume MCP server settings.
type MCPEnvConfig struct {
	DBPath     string
	DevMode    bool
	SourceRoot string
	ServerAddr string
	AuthToken  string
}
