package skill

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/easp-platform/easp/internal/database"
	"github.com/easp-platform/easp/internal/models"
)

// SkillEngine Skill执行引擎
type SkillEngine struct {
	stepExecutors map[string]StepExecutor
}

// StepExecutor 步骤执行器
type StepExecutor func(ctx context.Context, step Step, inputs map[string]interface{}) (map[string]interface{}, error)

// Step 步骤定义
type Step struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Action     string                 `json:"action"`
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
	StepName string                 `json:"step_name"`
	Status   string                 `json:"status"`
	Outputs  map[string]interface{} `json:"outputs"`
	Error    string                 `json:"error,omitempty"`
	Duration int64                  `json:"duration_ms"`
}

// NewSkillEngine 创建Skill执行引擎
func NewSkillEngine() *SkillEngine {
	engine := &SkillEngine{
		stepExecutors: make(map[string]StepExecutor),
	}
	engine.registerDefaultExecutors()
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
			nextName := e.getNextStepName(steps, step.Name)
			if nextName == "" {
				break
			}
			currentStep = nextName
		}
	}

	if execution.Status == "running" {
		execution.Status = "completed"
	}
	now := time.Now()
	execution.EndedAt = &now
	execution.Outputs = variables

	// 保存执行记录
	e.saveExecution(execution)

	return execution, nil
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

	// 解析参数
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
	return true
}

// resolveParams 解析参数
func (e *SkillEngine) resolveParams(params map[string]interface{}, variables map[string]interface{}) map[string]interface{} {
	resolved := make(map[string]interface{})
	for k, v := range params {
		if str, ok := v.(string); ok {
			if len(str) > 4 && str[:2] == "{{" && str[len(str)-2:] == "}}" {
				varName := str[2 : len(str)-2]
				if val, exists := variables[varName]; exists {
					resolved[k] = val
					continue
				}
			}
		}
		resolved[k] = v
	}
	return resolved
}

// saveExecution 保存执行记录
func (e *SkillEngine) saveExecution(execution *SkillExecution) {
	stepsJSON, _ := json.Marshal(execution.StepResults)
	outputsJSON, _ := json.Marshal(execution.Outputs)

	_, err := database.DB.Exec(`
		INSERT INTO skill_executions (id, skill_id, tenant_id, status, inputs, outputs, step_results, started_at, ended_at, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, execution.ID, execution.SkillID, execution.TenantID, execution.Status,
		jsonMarshal(execution.Inputs), string(outputsJSON), string(stepsJSON),
		execution.StartedAt, execution.EndedAt, execution.Error)
	if err != nil {
		log.Printf("Failed to save execution: %v", err)
	}
}

func jsonMarshal(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
