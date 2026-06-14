package mcp

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/models"
	"github.com/google/uuid"
)

type CurlImportCandidate struct {
	Method      string            `json:"method"`
	URL         string            `json:"url"`
	BaseURL     string            `json:"base_url"`
	Path        string            `json:"path"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body,omitempty"`
	InputSchema string            `json:"input_schema"`
}

type CurlImportTestResult struct {
	ID           string              `json:"id"`
	TenantID     string              `json:"tenant_id"`
	Candidate    CurlImportCandidate `json:"candidate"`
	Success      bool                `json:"success"`
	StatusCode   int                 `json:"status_code,omitempty"`
	ResponseBody string              `json:"response_body,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
}

type CurlImportCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	RiskLevel   string `json:"risk_level"`
}

func ParseCurlImportCommand(command string) (CurlImportCandidate, error) {
	tokens, err := splitShellLike(command)
	if err != nil {
		return CurlImportCandidate{}, err
	}
	if len(tokens) == 0 || tokens[0] != "curl" {
		return CurlImportCandidate{}, fmt.Errorf("只支持 curl 命令")
	}
	candidate := CurlImportCandidate{Method: "GET", Headers: map[string]string{}}
	for i := 1; i < len(tokens); i++ {
		t := tokens[i]
		switch t {
		case "-X", "--request":
			i++
			if i >= len(tokens) {
				return CurlImportCandidate{}, fmt.Errorf("curl %s 缺少方法", t)
			}
			candidate.Method = strings.ToUpper(strings.TrimSpace(tokens[i]))
		case "-H", "--header":
			i++
			if i >= len(tokens) {
				return CurlImportCandidate{}, fmt.Errorf("curl %s 缺少请求头", t)
			}
			parts := strings.SplitN(tokens[i], ":", 2)
			if len(parts) == 2 {
				candidate.Headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		case "-d", "--data", "--data-raw", "--data-binary", "--data-ascii":
			i++
			if i >= len(tokens) {
				return CurlImportCandidate{}, fmt.Errorf("curl %s 缺少请求体", t)
			}
			candidate.Body = tokens[i]
			if candidate.Method == "GET" {
				candidate.Method = "POST"
			}
		default:
			if strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://") {
				candidate.URL = t
			}
		}
	}
	if candidate.URL == "" {
		return CurlImportCandidate{}, fmt.Errorf("curl 命令缺少 URL")
	}
	parsed, err := url.Parse(candidate.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return CurlImportCandidate{}, fmt.Errorf("curl URL 无效")
	}
	candidate.BaseURL = parsed.Scheme + "://" + parsed.Host
	candidate.Path = parsed.EscapedPath()
	if candidate.Path == "" {
		candidate.Path = "/"
	}
	if parsed.RawQuery != "" {
		candidate.Path += "?" + parsed.RawQuery
	}
	candidate.InputSchema = inferInputSchema(candidate.Body, parsed.Query())
	return candidate, nil
}

func BuildMCPToolFromCurlTestResult(tenantID string, req CurlImportCreateRequest, result *CurlImportTestResult) (models.MCPTool, error) {
	if result == nil || !result.Success || result.TenantID != tenantID {
		return models.MCPTool{}, fmt.Errorf("必须先完成当前租户的成功 curl 测试，才能创建 MCP 工具")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return models.MCPTool{}, fmt.Errorf("name 为必填项")
	}
	risk := strings.ToLower(strings.TrimSpace(req.RiskLevel))
	if risk == "" {
		risk = "medium"
	}
	if risk != "low" && risk != "medium" && risk != "high" {
		return models.MCPTool{}, fmt.Errorf("不支持的风险等级: %s", req.RiskLevel)
	}
	desc := strings.TrimSpace(req.Description)
	method := result.Candidate.Method
	path := result.Candidate.Path
	schema := result.Candidate.InputSchema
	now := time.Now()
	return models.MCPTool{ID: uuid.New().String(), TenantID: tenantID, Name: name, Description: &desc, InputSchema: &schema, BackendMethod: &method, BackendPath: &path, RiskLevel: risk, Status: "draft", Enabled: false, CreatedAt: now, UpdatedAt: now}, nil
}

func inferInputSchema(body string, query url.Values) string {
	props := map[string]any{}
	for key := range query {
		props[key] = map[string]any{"type": "string"}
	}
	if strings.TrimSpace(body) != "" {
		var obj map[string]any
		if err := json.Unmarshal([]byte(body), &obj); err == nil {
			for k, v := range obj {
				props[k] = map[string]any{"type": jsonSchemaType(v)}
			}
		}
	}
	b, _ := json.Marshal(map[string]any{"type": "object", "properties": props, "required": []string{}})
	return string(b)
}

func jsonSchemaType(v any) string {
	switch v.(type) {
	case bool:
		return "boolean"
	case float64, int, int64:
		return "number"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "string"
	}
}

func splitShellLike(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	var quote rune
	escaped := false
	for _, r := range s {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if r == ' ' || r == '\n' || r == '\t' {
			if b.Len() > 0 {
				out = append(out, b.String())
				b.Reset()
			}
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteRune('\\')
	}
	if quote != 0 {
		return nil, fmt.Errorf("curl 命令引号未闭合")
	}
	if b.Len() > 0 {
		out = append(out, b.String())
	}
	return out, nil
}
