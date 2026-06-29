package openapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// OpenAPISpec OpenAPI规范 (兼容 OpenAPI 3.0 + Swagger 2.0)
type OpenAPISpec struct {
	OpenAPI string                 `json:"openapi"`  // OpenAPI 3.0
	Swagger string                 `json:"swagger"`  // Swagger 2.0 兼容
	Info    Info                   `json:"info"`
	Servers []Server               `json:"servers,omitempty"`
	Paths   map[string]PathItem    `json:"paths"`
	BasePath string                `json:"basePath,omitempty"` // Swagger 2.0 兼容
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

// 我们需要用 interface{} 先解析整个 paths，然后过滤掉非 object 项
// 有些文档生成工具会在 paths 下插入 x-* 字段，值不是 PathItem，会导致解析失败
type loosePaths map[string]interface{}

func parseOpenAPISpec(data []byte) (*OpenAPISpec, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	var spec OpenAPISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		// 如果直接解析失败，尝试手动处理 paths，过滤掉非 object 项
		// 重新解析
		var spec2 struct {
			OpenAPI string                 `json:"openapi"`
			Swagger string                 `json:"swagger"`
			Info    Info                   `json:"info"`
			Servers []Server               `json:"servers,omitempty"`
			Paths   loosePaths             `json:"paths"`
			BasePath string               `json:"basePath,omitempty"`
			Components *Components         `json:"components,omitempty"`
		}
		if err2 := json.Unmarshal(data, &spec2); err2 != nil {
			return nil, err // still fail, return original error
		}
		spec.OpenAPI = spec2.OpenAPI
		spec.Swagger = spec2.Swagger
		spec.Info = spec2.Info
		spec.Servers = spec2.Servers
		spec.BasePath = spec2.BasePath
		spec.Components = spec2.Components
		spec.Paths = make(map[string]PathItem)

		for path, value := range spec2.Paths {
			if path == "" || !IsObject(value) {
				continue // skip non-object entries (like x-* extensions with scalar values)
			}
			// marshal again and parse into PathItem
			pathItemBytes, _ := json.Marshal(value)
			var pathItem PathItem
			if json.Unmarshal(pathItemBytes, &pathItem) == nil {
				spec.Paths[path] = pathItem
			}
			// if fails to parse PathItem, skip it (ignore broken entries)
		}
	}
	return &spec, nil
}

// IsObject checks if value is a JSON object
func IsObject(v interface{}) bool {
	_, ok := v.(map[string]interface{})
	return ok
}

// ParseOpenAPISpec 解析OpenAPI规范
func ParseOpenAPISpec(data []byte) (*OpenAPISpec, error) {
	return parseOpenAPISpec(data)
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

	basePath := spec.BasePath // Swagger 2.0 basePath

	for path, pathItem := range spec.Paths {
		// Swagger 2.0: 拼接 basePath
		if basePath != "" {
			path = basePath + path
		}

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
