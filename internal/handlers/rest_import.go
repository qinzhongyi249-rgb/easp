package handlers

import (
	"encoding/json"
	"fmt"
	"strings"

	skillPkg "github.com/easp-platform/easp/internal/skill"
)

type normalizedRESTImportRequest struct {
	ConnectorID string
	Name        string
	APIPath     string
	Method      string
	Description string
	InputSchema string
	Status      string
	RiskLevel   string
	Enabled     bool
}

func normalizeRESTImportRequest(req ImportOpenAPIRequest) (normalizedRESTImportRequest, error) {
	if strings.TrimSpace(req.ConnectorID) == "" || strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.APIPath) == "" || strings.TrimSpace(req.Method) == "" {
		return normalizedRESTImportRequest{}, fmt.Errorf("connector_id、name、api_path、method 为必填项")
	}

	method := normalizeHTTPMethod(req.Method)
	if method == "" {
		return normalizedRESTImportRequest{}, fmt.Errorf("不支持的请求方法")
	}

	apiPath := strings.TrimSpace(req.APIPath)
	if !strings.HasPrefix(apiPath, "/") {
		return normalizedRESTImportRequest{}, fmt.Errorf("api_path 必须以 / 开头")
	}

	inputSchema := strings.TrimSpace(req.InputSchema)
	if inputSchema == "" {
		inputSchema = `{"type":"object","properties":{}}`
	} else if err := validateRESTInputSchema(inputSchema); err != nil {
		return normalizedRESTImportRequest{}, err
	}

	status := skillPkg.NormalizeSkillStatus(req.Status)
	switch status {
	case skillPkg.SkillStatusDraft, skillPkg.SkillStatusTesting, skillPkg.SkillStatusPublished, skillPkg.SkillStatusDisabled:
	default:
		return normalizedRESTImportRequest{}, fmt.Errorf("不支持的生命周期状态: %s", req.Status)
	}

	riskLevel := strings.ToLower(strings.TrimSpace(req.RiskLevel))
	if riskLevel == "" {
		riskLevel = "medium"
	}
	switch riskLevel {
	case "low", "medium", "high":
	default:
		return normalizedRESTImportRequest{}, fmt.Errorf("不支持的风险等级: %s", req.RiskLevel)
	}

	enabled := false
	if req.EnabledValue != nil {
		enabled = *req.EnabledValue
	}
	if enabled && status != skillPkg.SkillStatusPublished {
		return normalizedRESTImportRequest{}, fmt.Errorf("只有 published 状态的 REST MCP 工具允许导入后启用")
	}

	return normalizedRESTImportRequest{
		ConnectorID: strings.TrimSpace(req.ConnectorID),
		Name:        strings.TrimSpace(req.Name),
		APIPath:     apiPath,
		Method:      method,
		Description: strings.TrimSpace(req.Description),
		InputSchema: inputSchema,
		Status:      status,
		RiskLevel:   riskLevel,
		Enabled:     enabled,
	}, nil
}

func validateRESTInputSchema(schema string) error {
	var decoded any
	if err := json.Unmarshal([]byte(schema), &decoded); err != nil {
		return fmt.Errorf("input_schema 不是合法 JSON: %w", err)
	}
	obj, ok := decoded.(map[string]any)
	if !ok {
		return fmt.Errorf("input_schema 必须是 JSON object")
	}
	if schemaType, ok := obj["type"].(string); ok && schemaType != "object" {
		return fmt.Errorf("input_schema.type 必须为 object")
	}
	properties := map[string]any{}
	if props, ok := obj["properties"]; ok {
		var okProps bool
		properties, okProps = props.(map[string]any)
		if !okProps {
			return fmt.Errorf("input_schema.properties 必须是 object")
		}
	}
	if required, ok := obj["required"]; ok {
		items, ok := required.([]any)
		if !ok {
			return fmt.Errorf("input_schema.required 必须是数组")
		}
		for _, item := range items {
			field, ok := item.(string)
			if !ok {
				return fmt.Errorf("input_schema.required 只能包含字符串")
			}
			if _, exists := properties[field]; !exists {
				return fmt.Errorf("input_schema.required 字段 %q 未在 properties 中定义", field)
			}
		}
	}
	return nil
}
