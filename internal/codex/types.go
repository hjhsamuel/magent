package codex

const sockDir = "/run/codex"

type Config struct {
	McpServers map[string]McpServer `toml:"mcp_servers"`
}

type McpServer struct {
	Url         string            `toml:"url"`
	HttpHeaders map[string]string `toml:"http_headers"`
}
