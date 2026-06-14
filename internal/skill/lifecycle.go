package skill

import (
	"fmt"
	"strings"

	"github.com/easp-platform/easp/internal/models"
)

const (
	SkillStatusDraft     = "draft"
	SkillStatusTesting   = "testing"
	SkillStatusPublished = "published"
	SkillStatusDisabled  = "disabled"

	ExecutionModeDryRun     = "dry_run"
	ExecutionModeSandbox    = "sandbox"
	ExecutionModeProduction = "production"
)

func NormalizeSkillStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", SkillStatusDraft:
		return SkillStatusDraft
	case SkillStatusTesting:
		return SkillStatusTesting
	case SkillStatusPublished, "active":
		return SkillStatusPublished
	case SkillStatusDisabled, "archived", "inactive":
		return SkillStatusDisabled
	default:
		return strings.ToLower(strings.TrimSpace(status))
	}
}

func NormalizeExecutionMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case ExecutionModeDryRun:
		return ExecutionModeDryRun
	case ExecutionModeProduction:
		return ExecutionModeProduction
	case ExecutionModeSandbox, "":
		return ExecutionModeSandbox
	default:
		return strings.ToLower(strings.TrimSpace(mode))
	}
}

func IsPublishedSkillStatus(status string) bool {
	return NormalizeSkillStatus(status) == SkillStatusPublished
}

func CanExecuteSkill(sk models.Skill, mode string) error {
	status := NormalizeSkillStatus(sk.Status)
	mode = NormalizeExecutionMode(mode)

	if status == SkillStatusDisabled {
		return fmt.Errorf("技能已禁用，无法执行")
	}
	if mode == ExecutionModeProduction && status != SkillStatusPublished {
		return fmt.Errorf("技能状态为%s，只有published状态允许生产执行", status)
	}
	if mode == ExecutionModeDryRun || mode == ExecutionModeSandbox || mode == ExecutionModeProduction {
		return nil
	}
	return fmt.Errorf("不支持的执行模式: %s", mode)
}

func CanExecuteMCPTool(tool models.MCPTool, mode string) error {
	status := NormalizeSkillStatus(tool.Status)
	mode = NormalizeExecutionMode(mode)

	if status == SkillStatusDisabled {
		return fmt.Errorf("MCP工具已禁用，无法执行")
	}
	if mode == ExecutionModeProduction && status != SkillStatusPublished {
		return fmt.Errorf("MCP工具状态为%s，只有published状态允许生产执行", status)
	}
	if mode == ExecutionModeDryRun || mode == ExecutionModeSandbox || mode == ExecutionModeProduction {
		return nil
	}
	return fmt.Errorf("不支持的执行模式: %s", mode)
}

func ShouldSkipSideEffects(mode string) bool {
	mode = NormalizeExecutionMode(mode)
	return mode == ExecutionModeDryRun || mode == ExecutionModeSandbox
}
