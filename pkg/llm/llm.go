package llm

import (
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

type LLM struct {
	client openai.Client
	model  string
}

func NewLLM(baseUrl, apiKey, model string) (*LLM, error) {
	var opts []option.RequestOption
	if baseUrl != "" {
		opts = append(opts, option.WithBaseURL(baseUrl))
	}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	}
	client := openai.NewClient(opts...)
	return &LLM{
		client: client,
		model:  model,
	}, nil
}
