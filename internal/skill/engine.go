package skill

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
)

// MCPCaller MCP工具调用函数类型（避免循环依赖）
// 参数: toolName, arguments(json.RawMessage)
// 返回: result(map), error
type MCPCaller func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error)

// SkillEngine Skill执行引擎
type SkillEngine struct {
	stepExecutors map[string]StepExecutor
	mcpCaller     MCPCaller
	tenantID      string
}

// StepExecutor 步骤执行器
type StepExecutor func(ctx context.Context, step Step, inputs map[string]interface{}) (map[string]interface{}, error)

// Step 步骤定义
type Step struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`         // mcp_tool / http_request / condition / assign / code
	Action     string                 `json:"action"`       // 工具名或URL
	Params     map[string]interface{} `json:"params,omitempty"`
	Condition  string                 `json:"condition,omitempty"`
	NextOnOK   string                 `json:"next_on_ok,omitempty"`
	NextOnFail string                 `json:"next_on_fail,omitempty"`
	OutputVar  string                 `json:"output_var,omitempty"`
}

// SkillExecution Skill执行实例
type SkillExecution struct {
	ID          string                 `json:"id"`
	SkillID     string                 `json:"skill_id"`
	TenantID    string                 `json:"tenant_id"`
	Status      string                 `json:"status"`
	Inputs      map[string]interface{} `json:"inputs"`
	Outputs     map[string]interface{} `json:"outputs"`
	StepResults []StepResult           `json:"step_results"`
	StartedAt   time.Time              `json:"started_at"`
	EndedAt     *time.Time             `json:"ended_at,omitempty"`
	Error       string                 `json:"error,omitempty"`
}

// StepResult 步骤结果
type StepResult struct {
	StepName   string                 `json:"step_name"`
	Status     string                 `json:"status"`
	Outputs    map[string]interface{} `json:"outputs,omitempty"`
	Error      string                 `json:"error,omitempty"`
	Duration   int64                  `json:"duration_ms"`
}

// NewSkillEngine 创建Skill执行引擎
func NewSkillEngine() *SkillEngine {
	engine := &SkillEngine{
		stepExecutors: make(map[string]StepExecutor),
	}
	engine.registerDefaultExecutors()
	return engine
}

// NewSkillEngineWithCaller 创建带MCP调用能力的Skill执行引擎
func NewSkillEngineWithCaller(tenantID string, mcpCaller MCPCaller) *SkillEngine {
	engine := &SkillEngine{
		stepExecutors: make(map[string]StepExecutor),
		mcpCaller:     mcpCaller,
		tenantID:      tenantID,
	}
	engine.registerDefaultExecutors()
	engine.registerMCPToolExecutor()
	engine.registerHTTPRequestExecutor()
	return engine
}

// registerDefaultExecutors 注册默认执行器
func (e *SkillEngine) registerDefaultExecutors() {
	// 条件判断执行器
	e.stepExecutors["condition"] = func(ctx context.Context, step Step, inputs map[string]interface{}) (map[string]interface{}, error) {
		return map[string]interface{}{"result": true}, nil
	}

	// 变量赋值执行器
	e.stepExecutors["assign"] = func(ctx context.Context, step Step, inputs map[string]interface{}) (map[string]interface{}, error) {
		return step.Params, nil
	}
}

// registerMCPToolExecutor 注册MCP工具执行器
func (e *SkillEngine) registerMCPToolExecutor() {
	e.stepExecutors["mcp_tool"] = func(ctx context.Context, step Step, inputs map[string]interface{}) (map[string]interface{}, error) {
		if e.mcpCaller == nil {
			return nil, fmt.Errorf("MCP caller not configured")
		}

		// Action 字段指定 MCP 工具名
		toolName := step.Action
		if toolName == "" {
			return nil, fmt.Errorf("mcp_tool step requires action (tool name)")
		}

		// 合并参数：step.Params + inputs（inputs 优先，因为包含变量替换后的值）
		finalParams := make(map[string]interface{})
		for k, v := range step.Params {
			finalParams[k] = v
		}
		for k, v := range inputs {
			finalParams[k] = v
		}

		argumentsJSON, _ := json.Marshal(finalParams)
		log.Printf("SkillEngine: calling MCP tool %s with args: %s", toolName, string(argumentsJSON))

		// 通过 caller 函数调用
		result, callErr := e.mcpCaller(ctx, toolName, json.RawMessage(argumentsJSON))
		if callErr != nil {
			return nil, fmt.Errorf("MCP tool call failed: %v", callErr)
		}

		return result, nil
	}
}

// registerHTTPRequestExecutor 注册HTTP请求执行器
func (e *SkillEngine) registerHTTPRequestExecutor() {
	e.stepExecutors["http_request"] = func(ctx context.Context, step Step, inputs map[string]interface{}) (map[string]interface{}, error) {
		// Action 字段指定 URL
		url := step.Action
		if url == "" {
			return nil, fmt.Errorf("http_request step requires action (URL)")
		}

		method := "GET"
		if m, ok := step.Params["method"].(string); ok && m != "" {
			method = strings.ToUpper(m)
		}

		var body io.Reader
		if method == "POST" || method == "PUT" || method == "PATCH" {
			// 合并参数
			finalParams := make(map[string]interface{})
			for k, v := range step.Params {
				if k != "method" && k != "headers" {
					finalParams[k] = v
				}
			}
			for k, v := range inputs {
				finalParams[k] = v
			}
			bodyBytes, _ := json.Marshal(finalParams)
			body = bytes.NewReader(bodyBytes)
		}

		req, err := http.NewRequestWithContext(ctx, method, url, body)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %v", err)
		}

		req.Header.Set("Content-Type", "application/json")

		// 设置自定义 headers
		if headers, ok := step.Params["headers"].(map[string]interface{}); ok {
			for k, v := range headers {
				req.Header.Set(k, fmt.Sprintf("%v", v))
			}
		}

		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response: %v", err)
		}

		var result map[string]interface{}
		if err := json.Unmarshal(respBody, &result); err != nil {
			result = map[string]interface{}{
				"status_code": resp.StatusCode,
				"body":        string(respBody),
			}
		} else {
			result["_status_code"] = resp.StatusCode
		}

		if resp.StatusCode >= 400 {
			return result, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}

		return result, nil
	}
}

// RegisterExecutor 注册执行器
func (e *SkillEngine) RegisterExecutor(stepType string, executor StepExecutor) {
	e.stepExecutors[stepType] = executor
}

// Execute 执行Skill
func (e *SkillEngine) Execute(ctx context.Context, skill models.Skill, inputs map[string]interface{}) (*SkillExecution, error) {
	execution := &SkillExecution{
		ID:        generateID(),
		SkillID:   skill.ID,
		TenantID:  skill.TenantID,
		Status:    "running",
		Inputs:    inputs,
		Outputs:   make(map[string]interface{}),
		StartedAt: time.Now(),
	}

	// 解析步骤
	var steps []Step
	if err := json.Unmarshal([]byte(skill.Steps), &steps); err != nil {
		execution.Status = "failed"
		execution.Error = fmt.Sprintf("failed to parse steps: %v", err)
		return execution, err
	}

	// 执行步骤
	variables := make(map[string]interface{})
	for k, v := range inputs {
		variables[k] = v
	}

	currentStep := ""
	for {
		step := e.findStep(steps, currentStep)
		if step == nil {
			break
		}

		stepResult := e.executeStep(ctx, *step, variables)
		execution.StepResults = append(execution.StepResults, stepResult)

		if stepResult.Status == "failed" {
			execution.Status = "failed"
			execution.Error = stepResult.Error
			if step.NextOnFail != "" {
				currentStep = step.NextOnFail
				continue
			}
			break
		}

		// 保存输出变量
		if step.OutputVar != "" && len(stepResult.Outputs) > 0 {
			variables[step.OutputVar] = stepResult.Outputs
		}

		// 下一步
		if step.NextOnOK != "" {
			currentStep = step.NextOnOK
		} else {
			currentStep = e.getNextStepName(steps, step.Name)
		}
	}

	if execution.Status == "running" {
		execution.Status = "completed"
		execution.Outputs = variables
	}

	now := time.Now()
	execution.EndedAt = &now
	e.saveExecution(execution)

	return execution, nil
}

// ExecuteWithMCP 执行Skill（MCP调用版本，接收原始步骤数据）
func (e *SkillEngine) ExecuteWithMCP(ctx context.Context, tenantID string, rawSteps []interface{}, inputs map[string]interface{}) (map[string]interface{}, error) {
	// 将原始步骤转换为 Step 结构
	stepsJSON, err := json.Marshal(rawSteps)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal steps: %w", err)
	}

	var steps []Step
	if err := json.Unmarshal(stepsJSON, &steps); err != nil {
		return nil, fmt.Errorf("failed to parse steps: %w", err)
	}

	// 初始化变量
	variables := make(map[string]interface{})
	for k, v := range inputs {
		variables[k] = v
	}

	// 执行步骤
	var stepResults []StepResult
	currentStep := ""
	for {
		step := e.findStep(steps, currentStep)
		if step == nil {
			break
		}

		stepResult := e.executeStep(ctx, *step, variables)
		stepResults = append(stepResults, stepResult)

		if stepResult.Status == "failed" {
			if step.NextOnFail != "" {
				currentStep = step.NextOnFail
				continue
			}
			return map[string]interface{}{
				"status":       "failed",
				"error":        stepResult.Error,
				"step_results": stepResults,
			}, fmt.Errorf("step '%s' failed: %s", step.Name, stepResult.Error)
		}

		// 保存输出变量
		if step.OutputVar != "" && len(stepResult.Outputs) > 0 {
			variables[step.OutputVar] = stepResult.Outputs
		}

		// 下一步
		if step.NextOnOK != "" {
			currentStep = step.NextOnOK
		} else {
			currentStep = e.getNextStepName(steps, step.Name)
		}
	}

	return map[string]interface{}{
		"status":       "completed",
		"outputs":      variables,
		"step_results": stepResults,
	}, nil
}

// findStep 查找步骤
func (e *SkillEngine) findStep(steps []Step, name string) *Step {
	if name == "" {
		if len(steps) > 0 {
			return &steps[0]
		}
		return nil
	}
	for i, step := range steps {
		if step.Name == name {
			return &steps[i]
		}
	}
	return nil
}

// getNextStepName 获取下一个步骤名称
func (e *SkillEngine) getNextStepName(steps []Step, currentName string) string {
	for i, step := range steps {
		if step.Name == currentName && i+1 < len(steps) {
			return steps[i+1].Name
		}
	}
	return ""
}

// executeStep 执行单个步骤
func (e *SkillEngine) executeStep(ctx context.Context, step Step, variables map[string]interface{}) StepResult {
	start := time.Now()
	result := StepResult{
		StepName: step.Name,
		Status:   "running",
		Outputs:  make(map[string]interface{}),
	}

	// 检查条件
	if step.Condition != "" {
		conditionMet := e.evaluateCondition(step.Condition, variables)
		if !conditionMet {
			result.Status = "skipped"
			result.Duration = time.Since(start).Milliseconds()
			return result
		}
	}

	// 解析参数（变量替换）
	params := e.resolveParams(step.Params, variables)

	// 执行步骤
	executor, ok := e.stepExecutors[step.Type]
	if !ok {
		result.Status = "failed"
		result.Error = fmt.Sprintf("unknown step type: %s", step.Type)
		result.Duration = time.Since(start).Milliseconds()
		return result
	}

	outputs, err := executor(ctx, step, params)
	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
	} else {
		result.Status = "completed"
		result.Outputs = outputs
	}

	result.Duration = time.Since(start).Milliseconds()
	return result
}

// evaluateCondition 评估条件
func (e *SkillEngine) evaluateCondition(condition string, variables map[string]interface{}) bool {
	// 简单实现：检查变量是否存在
	parts := strings.SplitN(condition, " ", 3)
	if len(parts) >= 2 {
		varName := parts[0]
		operator := parts[1]
		_, exists := variables[varName]
		switch operator {
		case "exists":
			return exists
		case "not_exists":
			return !exists
		}
	}
	return true
}

// resolveParams 解析参数（变量替换）
func (e *SkillEngine) resolveParams(params map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	if params == nil {
		return make(map[string]interface{})
	}
	result := make(map[string]interface{})
	for k, v := range params {
		result[k] = e.resolveValue(v, variables)
	}
	return result
}

// resolveValue 递归解析值中的变量引用
func (e *SkillEngine) resolveValue(v interface{}, variables map[string]interface{}) interface{} {
	switch val := v.(type) {
	case string:
		// 替换 ${var.name} 格式的变量引用
		if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
			varPath := val[2 : len(val)-1]
			return e.getNestedValue(varPath, variables)
		}
		return val
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v2 := range val {
			result[k] = e.resolveValue(v2, variables)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v2 := range val {
			result[i] = e.resolveValue(v2, variables)
		}
		return result
	default:
		return val
	}
}

// getNestedValue 获取嵌套变量值
func (e *SkillEngine) getNestedValue(path string, variables map[string]interface{}) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = variables
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil
		}
		current = m[part]
	}
	return current
}

// saveExecution 保存执行记录
func (e *SkillEngine) saveExecution(execution *SkillExecution) {
	inputsJSON, _ := json.Marshal(execution.Inputs)
	outputsJSON, _ := json.Marshal(execution.Outputs)
	stepResultsJSON, _ := json.Marshal(execution.StepResults)

	query := `INSERT INTO skill_executions (id, skill_id, tenant_id, status, inputs, outputs, step_results, error, started_at, ended_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := database.DB.Exec(query,
		execution.ID, execution.SkillID, execution.TenantID, execution.Status,
		string(inputsJSON), string(outputsJSON), string(stepResultsJSON),
		execution.Error, execution.StartedAt, execution.EndedAt)
	if err != nil {
		log.Printf("Failed to save skill execution: %v", err)
	}
}

// generateID 生成唯一ID
func generateID() string {
	return fmt.Sprintf("exec_%d", time.Now().UnixNano())
}
