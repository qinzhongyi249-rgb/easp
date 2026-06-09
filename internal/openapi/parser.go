package openapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAPISpec OpenAPI规范
type OpenAPISpec struct {
	OpenAPI string                 `json:"openapi"`
	Info    Info                   `json:"info"`
	Servers []Server               `json:"servers,omitempty"`
	Paths   map[string]PathItem    `json:"paths"`
	Components *Components         `json:"components,omitempty"`
}

// Info 信息
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Version     string `json:"version"`
}

// Server 服务器
type Server struct {
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
}

// PathItem 路径项
type PathItem struct {
	Get    *Operation `json:"get,omitempty"`
	Post   *Operation `json:"post,omitempty"`
	Put    *Operation `json:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty"`
}

// Operation 操作
type Operation struct {
	Summary     string              `json:"summary,omitempty"`
	Description string              `json:"description,omitempty"`
	OperationID string              `json:"operationId,omitempty"`
	Tags        []string            `json:"tags,omitempty"`
	Parameters  []Parameter         `json:"parameters,omitempty"`
	RequestBody *RequestBody        `json:"requestBody,omitempty"`
	Responses   map[string]Response `json:"responses"`
}

// Parameter 参数
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"`
	Description string  `json:"description,omitempty"`
	Required    bool    `json:"required,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody 请求体
type RequestBody struct {
	Description string               `json:"description,omitempty"`
	Required    bool                 `json:"required,omitempty"`
	Content     map[string]MediaType `json:"content"`
}

// MediaType 媒体类型
type MediaType struct {
	Schema *Schema `json:"schema,omitempty"`
}

// Response 响应
type Response struct {
	Description string               `json:"description"`
	Content     map[string]MediaType `json:"content,omitempty"`
}

// Schema 模式
type Schema struct {
	Type        string             `json:"type,omitempty"`
	Format      string             `json:"format,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Enum        []interface{}      `json:"enum,omitempty"`
	Default     interface{}        `json:"default,omitempty"`
	Ref         string             `json:"$ref,omitempty"`
}

// Components 组件
type Components struct {
	Schemas map[string]*Schema `json:"schemas,omitempty"`
}

// MCPToolFromOpenAPI 从OpenAPI生成MCP工具
type MCPToolFromOpenAPI struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	InputSchema interface{} `json:"inputSchema,omitempty"`
	Method      string      `json:"method"`
	Path        string      `json:"path"`
}

// ParseOpenAPISpec 解析OpenAPI规范
func ParseOpenAPISpec(data []byte) (*OpenAPISpec, error) {
	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}
	return &spec, nil
}

// FetchOpenAPISpec 从URL获取OpenAPI规范
func FetchOpenAPISpec(url string) (*OpenAPISpec, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch OpenAPI spec: status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return ParseOpenAPISpec(body)
}

// ConvertToMCPTools 将OpenAPI规范转换为MCP工具列表
func ConvertToMCPTools(spec *OpenAPISpec) []MCPToolFromOpenAPI {
	var tools []MCPToolFromOpenAPI

	for path, pathItem := range spec.Paths {
		operations := map[string]*Operation{
			"GET":    pathItem.Get,
			"POST":   pathItem.Post,
			"PUT":    pathItem.Put,
			"DELETE": pathItem.Delete,
			"PATCH":  pathItem.Patch,
		}

		for method, op := range operations {
			if op == nil {
				continue
			}

			tool := convertOperation(path, method, op, spec)
			tools = append(tools, tool)
		}
	}

	return tools
}

// convertOperation 转换单个操作
func convertOperation(path, method string, op *Operation, spec *OpenAPISpec) MCPToolFromOpenAPI {
	name := op.OperationID
	if name == "" {
		name = generateToolName(path, method)
	}

	description := op.Summary
	if description == "" {
		description = op.Description
	}
	if description == "" {
		description = fmt.Sprintf("%s %s", method, path)
	}

	inputSchema := buildInputSchema(op, spec)

	return MCPToolFromOpenAPI{
		Name:        name,
		Description: description,
		InputSchema: inputSchema,
		Method:      method,
		Path:        path,
	}
}

// generateToolName 生成工具名称
func generateToolName(path, method string) string {
	// 将路径转换为驼峰命名
	parts := strings.Split(strings.Trim(path, "/"), "/")
	var nameParts []string
	nameParts = append(nameParts, strings.ToLower(method))
	for _, part := range parts {
		if strings.HasPrefix(part, "{") {
			continue
		}
		nameParts = append(nameParts, part)
	}
	return strings.Join(nameParts, "_")
}

// buildInputSchema 构建输入Schema
func buildInputSchema(op *Operation, spec *OpenAPISpec) map[string]interface{} {
	schema := map[string]interface{}{
		"type": "object",
	}

	properties := make(map[string]interface{})
	var required []string

	// 处理路径和查询参数
	for _, param := range op.Parameters {
		propSchema := map[string]interface{}{
			"type": "string",
		}
		if param.Schema != nil {
			if param.Schema.Type != "" {
				propSchema["type"] = param.Schema.Type
			}
			if param.Schema.Description != "" {
				propSchema["description"] = param.Schema.Description
			} else if param.Description != "" {
				propSchema["description"] = param.Description
			}
			if param.Schema.Enum != nil {
				propSchema["enum"] = param.Schema.Enum
			}
		}
		properties[param.Name] = propSchema
		if param.Required {
			required = append(required, param.Name)
		}
	}

	// 处理请求体
	if op.RequestBody != nil {
		if mediaType, ok := op.RequestBody.Content["application/json"]; ok && mediaType.Schema != nil {
			bodySchema := resolveSchema(mediaType.Schema, spec)
			if bodySchema.Properties != nil {
				for name, prop := range bodySchema.Properties {
					propMap := map[string]interface{}{}
					if prop.Type != "" {
						propMap["type"] = prop.Type
					}
					if prop.Description != "" {
						propMap["description"] = prop.Description
					}
					properties[name] = propMap
				}
			}
			if bodySchema.Required != nil {
				required = append(required, bodySchema.Required...)
			}
		}
	}

	schema["properties"] = properties
	if len(required) > 0 {
		schema["required"] = required
	}

	return schema
}

// resolveSchema 解析Schema引用
func resolveSchema(schema *Schema, spec *OpenAPISpec) *Schema {
	if schema.Ref != "" && spec.Components != nil {
		refName := strings.TrimPrefix(schema.Ref, "#/components/schemas/")
		if resolved, ok := spec.Components.Schemas[refName]; ok {
			return resolved
		}
	}
	return schema
}
