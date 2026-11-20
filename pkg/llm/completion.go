package llm

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go/v3"
)

// Complete returns the text of a single non-streaming chat completion.
func (l *LLM) Complete(ctx context.Context, model, instructions, input string) (string, error) {
	if model == "" {
		model = l.model
	}

	result, err := l.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(instructions),
			openai.UserMessage(input),
		},
	})
	if err != nil {
		return "", fmt.Errorf("chat completion: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", errors.New("chat completion returned no choices")
	}
	choice := result.Choices[0]
	if choice.FinishReason != "stop" {
		return "", fmt.Errorf("incomplete chat completion: %s", choice.FinishReason)
	}
	text := strings.TrimSpace(choice.Message.Content)
	if text == "" {
		return "", errors.New("chat completion returned no text")
	}
	return text, nil
}
