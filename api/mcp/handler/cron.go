package handler

import (
	"context"
	"errors"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/hjhsamuel/magent/api/mcp/entities"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func cronInput(properties map[string]*jsonschema.Schema, required ...string) *jsonschema.Schema {
	return &jsonschema.Schema{
		Type:                 "object",
		Properties:           properties,
		Required:             required,
		AdditionalProperties: &jsonschema.Schema{Not: &jsonschema.Schema{}},
	}
}

func (a *Api) initCron() {
	const objectID = "[0-9a-fA-F]{24}"
	id := &jsonschema.Schema{
		Type:        "string",
		Pattern:     "^" + objectID + "$",
		Description: "The 24-character hexadecimal job ID returned by get_cronjob_list for the current user.",
	}
	status := &jsonschema.Schema{
		Type:        "integer",
		Enum:        []any{0, 1},
		Description: "Job status. Use 0 for online (scheduled) or 1 for offline (paused).",
	}
	spec := &jsonschema.Schema{
		Type:      "string",
		MinLength: jsonschema.Ptr(1),
		Description: "A cron schedule has six fields in order: second, minute, hour, day of month, month, and day of week. '0 0 9 * * *' runs daily at 09:00.\n" +
			"In a cron field, '*' or '?' matches any value, '-' specifies an inclusive range (1-5 in day of week means Monday through Friday), " +
			"',' lists values (1,15 in day of month means the 1st and 15th), and '/' sets a step (*/15 in the minute field means minutes 0, 15, 30, and 45).\n" +
			"Use '@every 30m' for a 30-minute interval. In @daily, @weekly, and @monthly schedules, ':' separates hours and minutes, and ',' can list times, as in '@daily 09:00,17:30'. " +
			"Only the day field of @monthly accepts 'L' for the last day of the month and 'L-N' for N days before it (N from 0 to 30). For example, '@monthly L-1 09:00' runs at 09:00 on the second-to-last day.",
		Examples: []any{"0 0 9 * * *", "@every 30m", "@daily 0:00,12:00", "@weekly 1-5 9:00", "@monthly L 5:00"},
	}

	mcp.AddTool(a.mcpSvc, &mcp.Tool{
		Name:        "get_cronjob_list",
		Description: "List the current user's cron jobs in ascending job ID order. Returns one page in data and a cursor in offset. Omit status to include both online and offline jobs.",
		InputSchema: cronInput(
			map[string]*jsonschema.Schema{
				"offset": {
					Type:        "string",
					Pattern:     "^(" + objectID + ")?$",
					Description: "Exclusive pagination cursor. Omit it or pass an empty string for the first page. For the next page, pass the offset returned by the previous call.",
				},
				"limit": {
					Type:        "integer",
					Minimum:     jsonschema.Ptr(1.0),
					Maximum:     jsonschema.Ptr(50.0),
					Description: "Maximum number of jobs to return per page. Required integer from 1 to 50 inclusive.",
				},
				"status": status,
			},
			"limit",
		),
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, a.GetCronList)

	mcp.AddTool(a.mcpSvc, &mcp.Tool{
		Name:        "add_cronjob",
		Description: "Create and enable a cron job for the current user. Returns the new job ID. Each successful call creates a separate job.",
		InputSchema: cronInput(
			map[string]*jsonschema.Schema{
				"name": {
					Type:        "string",
					MinLength:   jsonschema.Ptr(1),
					Description: "Display name for the job. Must contain at least one character.",
				},
				"spec": spec,
				"content": {
					Type:        "string",
					MinLength:   jsonschema.Ptr(1),
					Description: "Full input text sent to the agent each time the job runs. Must contain at least one character.",
				},
			},
			"name", "spec", "content"),
	}, a.AddCronJob)

	update := cronInput(
		map[string]*jsonschema.Schema{
			"id": id,
			"name": {
				Type:        "string",
				Description: "New display name for the job.",
			},
			"spec": spec,
			"content": {
				Type:        "string",
				Description: "Full replacement text to send to the agent each time the job runs.",
			},
			"status": status,
		},
		"id",
	)
	update.MinProperties = jsonschema.Ptr(2)
	mcp.AddTool(a.mcpSvc, &mcp.Tool{
		Name:        "update_cronjob",
		Description: "Update a cron job owned by the current user. Supply id and at least one of name, spec, content, or status. Omitted fields keep their current values. Returns OK on success.",
		InputSchema: update,
	}, a.UpdateCronJob)

	mcp.AddTool(a.mcpSvc, &mcp.Tool{
		Name:        "delete_cronjob",
		Description: "Permanently delete a cron job owned by the current user and remove its schedule. To pause it instead, call update_cronjob with status set to 1. Returns OK on success.",
		InputSchema: cronInput(
			map[string]*jsonschema.Schema{
				"id": id,
			},
			"id",
		),
		Annotations: &mcp.ToolAnnotations{DestructiveHint: jsonschema.Ptr(true)},
	}, a.DeleteCronJob)
}

var errCronNotImplemented = errors.New("cron tool not implemented")

func (a *Api) GetCronList(ctx context.Context, req *mcp.CallToolRequest, input entities.GetCronListReq) (*mcp.CallToolResult, entities.GetCronListRsp, error) {
	return nil, entities.GetCronListRsp{}, errCronNotImplemented
}

func (a *Api) AddCronJob(ctx context.Context, req *mcp.CallToolRequest, input entities.AddCronJobReq) (*mcp.CallToolResult, any, error) {
	return nil, nil, errCronNotImplemented
}

func (a *Api) UpdateCronJob(ctx context.Context, req *mcp.CallToolRequest, input entities.UpdateCronJobReq) (*mcp.CallToolResult, any, error) {
	return nil, nil, errCronNotImplemented
}

func (a *Api) DeleteCronJob(ctx context.Context, req *mcp.CallToolRequest, input entities.DeleteCronJobReq) (*mcp.CallToolResult, any, error) {
	return nil, nil, errCronNotImplemented
}
