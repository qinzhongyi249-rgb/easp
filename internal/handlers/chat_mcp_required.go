package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// findToolByName 从工具定义列表里按 name 找到对应的 ToolDefinition。
func findToolByName(tools []ToolDefinition, name string) *ToolDefinition {
	for i := range tools {
		if tools[i].Function.Name == name {
			return &tools[i]
		}
	}
	return nil
}

// extractRequiredFields 从 tool.Function.Parameters (JSON Schema 结构) 里取 required 字段列表 + 字段描述映射。
// Parameters 通常为 map[string]any 或 json.RawMessage。
func extractRequiredFields(params any) (required []string, propDescriptions map[string]string) {
	propDescriptions = map[string]string{}
	if params == nil {
		return nil, propDescriptions
	}
	// 支持 map[string]any 与 json.RawMessage 两种存储形式
	var m map[string]any
	switch v := params.(type) {
	case map[string]any:
		m = v
	case json.RawMessage:
		if len(v) == 0 {
			return nil, propDescriptions
		}
		_ = json.Unmarshal(v, &m)
	case []byte:
		_ = json.Unmarshal(v, &m)
	case string:
		_ = json.Unmarshal([]byte(v), &m)
	default:
		// 尝试通过 marshal+unmarshal
		if raw, err := json.Marshal(v); err == nil {
			_ = json.Unmarshal(raw, &m)
		}
	}
	if m == nil {
		return nil, propDescriptions
	}
	if req, ok := m["required"].([]any); ok {
		for _, r := range req {
			if s, ok := r.(string); ok && s != "" {
				required = append(required, s)
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		for name, prop := range props {
			if pm, ok := prop.(map[string]any); ok {
				if desc, ok := pm["description"].(string); ok && desc != "" {
					propDescriptions[name] = desc
				}
			}
		}
	}
	return required, propDescriptions
}

// checkMCPRequiredParams 在真实执行工具之前预检 required 参数。
// 返回：
//   - missing:  真正缺失的字段名列表（对应用户需要补充的项）
//   - message:  面向用户的自然语言提示（可直接作为 assistant 回复的 stub 使用）
//   - resultJSON: 一个满足 requires_input 契约的伪 result 字符串，供 requiresInputReplyFromToolResult 复用
//
// 认定为缺失的判定：required 字段在 args 中不存在，或值为 nil、空字符串、空数组/对象。
func checkMCPRequiredParams(toolName string, args map[string]any, params any) (missing []string, message string, resultJSON string) {
	required, propDescriptions := extractRequiredFields(params)
	if len(required) == 0 {
		return nil, "", ""
	}
	for _, field := range required {
		if isMissingArg(args, field) {
			missing = append(missing, field)
		}
	}
	if len(missing) == 0 {
		return nil, "", ""
	}
	sort.Strings(missing)

	// 组装面向用户的提示
	fieldParts := make([]string, 0, len(missing))
	for _, f := range missing {
		if desc, ok := propDescriptions[f]; ok && desc != "" {
			fieldParts = append(fieldParts, fmt.Sprintf("%s（%s）", f, desc))
		} else {
			fieldParts = append(fieldParts, f)
		}
	}
	displayName := getToolDisplayName(toolName)
	if displayName == "" {
		displayName = toolName
	}
	message = fmt.Sprintf("执行「%s」还缺少必要参数：%s。请补充这些信息后我再继续。",
		displayName, strings.Join(fieldParts, "、"))

	// 复用 requires_input 契约，供既有 requiresInputReplyFromToolResult 展示
	payload := map[string]any{
		"success": true,
		"status":  "requires_input",
		"outputs": map[string]any{
			"message":        message,
			"missing_fields": missing,
			"tool_name":      toolName,
		},
	}
	if raw, err := json.Marshal(payload); err == nil {
		resultJSON = string(raw)
	}
	return missing, message, resultJSON
}

// isMissingArg 判定 args[field] 是否算"缺失"——不存在、nil、空字符串、空数组/对象、空白字符串。
func isMissingArg(args map[string]any, field string) bool {
	v, ok := args[field]
	if !ok {
		return true
	}
	if v == nil {
		return true
	}
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val) == ""
	case []any:
		return len(val) == 0
	case map[string]any:
		return len(val) == 0
	}
	return false
}
