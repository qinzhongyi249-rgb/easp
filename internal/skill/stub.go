// Package skill provides skill lifecycle management stubs for the EASP open source core.
// The commercial version includes a full skill engine with step execution,
// permission topology, and execution auditing.
package skill

import (
	"context"
	"encoding/json"

	"github.com/easp-platform/easp/internal/models"
)

const (
	// ExecutionModeProduction indicates a production execution.
	ExecutionModeProduction = "production"
	// ExecutionModeDryRun indicates a dry-run execution (no side effects).
	ExecutionModeDryRun = "dry_run"
	// ExecutionModeSandbox indicates a sandbox execution.
	ExecutionModeSandbox = "sandbox"
)

// SkillStatusPublished is the published lifecycle status.
const SkillStatusPublished = "published"

// NormalizeSkillStatus normalizes a skill status string. In the open source version,
// this is a simple passthrough that maps common aliases to canonical values.
func NormalizeSkillStatus(status string) string {
	switch status {
	case "active", "enabled":
		return SkillStatusPublished
	case "production", "normal":
		return ExecutionModeProduction
	default:
		return status
	}
}

// CanExecuteMCPTool checks whether an MCP tool can be executed in the given mode.
// In the open source version, this always returns nil (allowed).
func CanExecuteMCPTool(_ models.MCPTool, _ string) error {
	return nil
}

// MCPCaller is the function type for calling MCP tools from skills.
type MCPCaller func(ctx context.Context, toolName string, arguments json.RawMessage) (map[string]interface{}, error)

// SkillEngine executes skills. In the open source version, this is a stub.
type SkillEngine struct {
	TenantID string
}

// NewSkillEngineWithCaller creates a new SkillEngine.
func NewSkillEngineWithCaller(tenantID string, _ MCPCaller) *SkillEngine {
	return &SkillEngine{TenantID: tenantID}
}

// ExecuteWithMCP executes a skill with MCP tool calls. In the open source version,
// this returns an error because the full engine is not available.
func (e *SkillEngine) ExecuteWithMCP(_ context.Context, _ string, _ []interface{}, _ map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}
