package handler

import (
	"context"
	"errors"
	"strings"

	"github.com/hjhsamuel/magent/api/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// GetAuth returns the authenticated caller for the current MCP tool call.
func GetAuth(ctx context.Context) (*utils.Auth, error) {
	caller, ok := ctx.Value(utils.AuthKey).(*utils.Auth)
	if !ok || caller == nil {
		return nil, errors.New("unauthorized")
	}
	return caller, nil
}

func authenticateToolCall(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		if method != "tools/call" {
			return next(ctx, method, req)
		}

		if extra := req.GetExtra(); extra != nil {
			scheme, raw, ok := strings.Cut(extra.Header.Get("Authorization"), " ")
			if ok && strings.EqualFold(scheme, "Bearer") {
				if caller, err := utils.ParseAuth(raw); err == nil {
					return next(context.WithValue(ctx, utils.AuthKey, caller), method, req)
				}
			}
		}
		result := new(mcp.CallToolResult)
		result.SetError(errors.New("unauthorized: supply a valid user JWT in the Authorization: Bearer header"))
		return result, nil
	}
}
