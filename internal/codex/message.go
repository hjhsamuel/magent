package codex

import (
	"encoding/json"
	"fmt"
)

type Method string

const (
	MethodInitialize  Method = "initialize"
	MethodInitialized Method = "initialized"

	MethodThreadStart         Method = "thread/start"
	MethodThreadResume        Method = "thread/resume"
	MethodThreadStatusChanged Method = "thread/status/changed"

	MethodTurnStart Method = "turn/start"
)

type Message struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method Method          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}
