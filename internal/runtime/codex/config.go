package codex

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

type Config struct {
	McpServers map[string]McpServerConfig `toml:"mcp_servers,omitempty"`
}

func (c *Config) MarshalTOML() ([]byte, error) {
	for name, m := range c.McpServers {
		if name == "" || (m.Command == "") == (m.URL == "") {
			return nil, fmt.Errorf("codex: MCP server %q requires exactly one of command or URL", name)
		}
		if m.URL != "" && (len(m.Args) > 0 || len(m.Env) > 0 || len(m.EnvVars) > 0 || m.Cwd != "") {
			return nil, fmt.Errorf("codex: HTTP MCP server %q contains stdio settings", name)
		}
		if m.Command != "" && (m.BearerTokenEnvVar != "" || len(m.HTTPHeaders) > 0 || len(m.EnvHTTPHeaders) > 0) {
			return nil, fmt.Errorf("codex: stdio MCP server %q contains HTTP settings", name)
		}
		if (m.StartupTimeoutSec != nil && *m.StartupTimeoutSec < 0) || (m.ToolTimeoutSec != nil && *m.ToolTimeoutSec < 0) {
			return nil, fmt.Errorf("codex: MCP server %q has a negative timeout", name)
		}
	}

	type obj *Config
	return toml.Marshal(obj(c))
}

func (c *Config) WriteTo(workdir string) error {
	content, err := c.MarshalTOML()
	if err != nil {
		return err
	}

	codexDir := filepath.Join(workdir, ".codex")
	_ = os.MkdirAll(codexDir, 0755)

	return os.WriteFile(filepath.Join(codexDir, "config.toml"), content, 0644)
}

type McpServerConfig struct {
	Command           string            `toml:"command,omitempty"`
	Args              []string          `toml:"args,omitempty"`
	Env               map[string]string `toml:"env,omitempty"`
	EnvVars           []string          `toml:"env_vars,omitempty"`
	Cwd               string            `toml:"cwd,omitempty"`
	URL               string            `toml:"url,omitempty"`
	BearerTokenEnvVar string            `toml:"bearer_token_env_var,omitempty"`
	HTTPHeaders       map[string]string `toml:"http_headers,omitempty"`
	EnvHTTPHeaders    map[string]string `toml:"env_http_headers,omitempty"`
	Enabled           *bool             `toml:"enabled,omitempty"`
	Required          *bool             `toml:"required,omitempty"`
	StartupTimeoutSec *float64          `toml:"startup_timeout_sec,omitempty"`
	ToolTimeoutSec    *float64          `toml:"tool_timeout_sec,omitempty"`
	EnabledTools      []string          `toml:"enabled_tools,omitempty"`
	DisabledTools     []string          `toml:"disabled_tools,omitempty"`
}
