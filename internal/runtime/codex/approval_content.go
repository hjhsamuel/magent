package codex

import (
	"encoding/json"
	"html"
	"strings"
)

// approvalContent renders only user-facing information as Markdown.
// Never interpolate protocol values as markup.
func approvalContent(method string, params json.RawMessage) (string, error) {
	var p ApprovalParams
	if err := json.Unmarshal(params, &p); err != nil {
		return "", err
	}
	title := "命令执行审批"
	switch method {
	case "item/fileChange/requestApproval":
		title = "文件修改审批"
	case "item/permissions/requestApproval":
		title = "权限申请审批"
	}
	var b strings.Builder
	b.WriteString("### " + title + "\n")
	field := func(label, value string, code bool) {
		if strings.TrimSpace(value) == "" {
			return
		}
		b.WriteString("\n**" + label + "**\n\n")
		value = strings.ReplaceAll(value, "\r\n", "\n")
		if code {
			fence := "```"
			for strings.Contains(value, fence) {
				fence += "`"
			}
			b.WriteString(fence + "text\n" + value + "\n" + fence + "\n")
		} else {
			var escaped strings.Builder
			for _, r := range value {
				if strings.ContainsRune("\\`*_{}[]()#+-.!|>~", r) {
					escaped.WriteByte('\\')
				}
				escaped.WriteRune(r)
			}
			b.WriteString(strings.ReplaceAll(html.EscapeString(escaped.String()), "\n", "  \n") + "\n")
		}
	}
	if strings.TrimSpace(p.Reason) == "" {
		p.Reason = "未提供审批原因"
	}
	field("审批原因", p.Reason, false)
	field("期望执行命令", p.Command, true)
	field("工作目录", p.Cwd, true)
	field("文件访问范围", p.GrantRoot, true)
	if n := p.NetworkApprovalContext; n != nil {
		address := n.Host
		if n.Protocol != "" {
			address = n.Protocol + "://" + address
		}
		field("网络访问", address, false)
	} else if p.Permissions != nil && p.Permissions.Network != nil {
		status := "不申请网络访问"
		if p.Permissions.Network.Enabled {
			status = "申请启用网络访问"
		}
		field("网络访问", status, false)
	}
	if p.Permissions != nil && p.Permissions.FileSystem != nil {
		field("允许读取", strings.Join(p.Permissions.FileSystem.Read, "\n"), true)
		field("允许写入", strings.Join(p.Permissions.FileSystem.Write, "\n"), true)
	}
	return b.String(), nil
}
