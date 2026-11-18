package service

import (
	"encoding/json"
	"strings"

	"github.com/hjhsamuel/magent/internal/codex"
)

type completedTurn struct {
	TurnID  string
	Summary string
}

func turnSummary(turn codex.Turn) string {
	if turn.Error != nil {
		return turn.Error.Message
	}
	var parts []string
	for _, raw := range turn.Items {
		var item struct {
			Type string
			Text string
		}
		if json.Unmarshal(raw, &item) == nil && item.Type == "agentMessage" {
			parts = append(parts, item.Text)
		}
	}
	return strings.Join(parts, "\n")
}
