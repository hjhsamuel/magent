package mcpx

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type MCP struct {
	url     string
	headers map[string]string
	client  *mcp.Client
}

func (m *MCP) connect(ctx context.Context) (*mcp.ClientSession, error) {
	transport := &mcp.StreamableClientTransport{
		Endpoint: m.url,
		HTTPClient: &http.Client{
			Transport: &Transport{
				Headers: m.headers,
				Base:    http.DefaultTransport,
			},
		},
	}
	return m.client.Connect(ctx, transport, nil)
}

func (m *MCP) Ping(ctx context.Context) error {
	conn, err := m.connect(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	return conn.Ping(ctx, nil)
}

func NewMCP(url string, headers map[string]string) (*MCP, error) {
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "magent",
		Version: "0.1.0",
	}, nil)
	return &MCP{
		url:     url,
		headers: headers,
		client:  client,
	}, nil
}
