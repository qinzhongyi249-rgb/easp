package skill

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/easp-platform/easp/internal/models"
)

func TestSkillEngineRequiredStepReturnsRequiresInput(t *testing.T) {
	engine := NewSkillEngine()
	steps := `[{
		"name":"ask_required",
		"type":"required",
		"params":{
			"fields":["email","role_name"],
			"message":"创建用户需要邮箱和角色名称"
		}
	}]`

	exec, err := engine.ExecuteWithMode(context.Background(), models.Skill{
		ID:       "skill-required",
		TenantID: "tenant-a",
		Name:     "创建用户",
		Steps:    steps,
		Status:   SkillStatusPublished,
	}, map[string]interface{}{"email": "new@example.com"}, ExecutionModeProduction)
	if err != nil {
		t.Fatalf("ExecuteWithMode returned unexpected error: %v", err)
	}
	if exec.Status != "requires_input" {
		t.Fatalf("expected requires_input status, got %s", exec.Status)
	}
	missing, _ := exec.Outputs["missing_fields"].([]string)
	if len(missing) != 1 || missing[0] != "role_name" {
		t.Fatalf("expected missing role_name, got %#v", exec.Outputs["missing_fields"])
	}
	if exec.Outputs["message"] == "" {
		t.Fatalf("expected message for user clarification")
	}
}

func TestSkillEngineParallelStepRunsChildrenConcurrently(t *testing.T) {
	atomic.StoreInt32(&calls, 0)
	engine := NewSkillEngineWithCaller("tenant-a", func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error) {
		atomic.AddInt32(&calls, 1)
		time.Sleep(80 * time.Millisecond)
		return map[string]interface{}{"tool": toolName}, nil
	})
	steps := `[{
		"name":"parallel_checks",
		"type":"parallel",
		"params":{
			"steps":[
				{"name":"list_users","type":"mcp_tool","action":"list_users","output_var":"users"},
				{"name":"list_roles","type":"mcp_tool","action":"list_roles","output_var":"roles"}
			]
		},
		"output_var":"checks"
	}]`
	start := time.Now()
	exec, err := engine.ExecuteWithMode(context.Background(), models.Skill{
		ID:       "skill-parallel",
		TenantID: "tenant-a",
		Name:     "并发检查",
		Steps:    steps,
		Status:   SkillStatusPublished,
	}, map[string]interface{}{}, ExecutionModeProduction)
	if err != nil {
		t.Fatalf("ExecuteWithMode returned unexpected error: %v", err)
	}
	if exec.Status != "completed" {
		t.Fatalf("expected completed, got %s error=%s", exec.Status, exec.Error)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected 2 child calls, got %d", got)
	}
	if elapsed := time.Since(start); elapsed >= 150*time.Millisecond {
		t.Fatalf("parallel children should run concurrently, elapsed=%s", elapsed)
	}
	checks, ok := exec.Outputs["checks"].(map[string]interface{})
	if !ok || checks["list_users"] == nil || checks["list_roles"] == nil {
		t.Fatalf("expected child outputs under checks, got %#v", exec.Outputs["checks"])
	}
}

var calls int32
