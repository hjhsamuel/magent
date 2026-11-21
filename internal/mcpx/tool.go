package mcpx

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (m *MCP) GetTools() ([]*Tool, error) {
	ctx := context.Background()

	conn, err := m.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	result, err := conn.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, err
	}

	tools := make([]*Tool, 0, len(result.Tools))
	for _, item := range result.Tools {
		tools = append(tools, &Tool{
			Name:        item.Name,
			Description: item.Description,
		})
	}
	return tools, nil
}
