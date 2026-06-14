package skill

import (
	"testing"

	"github.com/easp-platform/easp/internal/models"
)

func TestNormalizeSkillLifecycleStatus(t *testing.T) {
	cases := map[string]string{
		"":          SkillStatusDraft,
		"active":    SkillStatusPublished,
		"published": SkillStatusPublished,
		"testing":   SkillStatusTesting,
		"archived":  SkillStatusDisabled,
		"disabled":  SkillStatusDisabled,
	}
	for input, want := range cases {
		if got := NormalizeSkillStatus(input); got != want {
			t.Fatalf("NormalizeSkillStatus(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestCanExecuteSkillByExecutionMode(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		mode    string
		wantErr bool
	}{
		{name: "published production allowed", status: SkillStatusPublished, mode: ExecutionModeProduction, wantErr: false},
		{name: "testing sandbox allowed", status: SkillStatusTesting, mode: ExecutionModeSandbox, wantErr: false},
		{name: "draft dry run allowed", status: SkillStatusDraft, mode: ExecutionModeDryRun, wantErr: false},
		{name: "draft production denied", status: SkillStatusDraft, mode: ExecutionModeProduction, wantErr: true},
		{name: "testing production denied", status: SkillStatusTesting, mode: ExecutionModeProduction, wantErr: true},
		{name: "disabled sandbox denied", status: SkillStatusDisabled, mode: ExecutionModeSandbox, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CanExecuteSkill(models.Skill{Status: tc.status}, tc.mode)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CanExecuteSkill(status=%q, mode=%q) err=%v, wantErr=%v", tc.status, tc.mode, err, tc.wantErr)
			}
		})
	}
}

func TestDryRunDoesNotCallSideEffectExecutors(t *testing.T) {
	if !ShouldSkipSideEffects(ExecutionModeDryRun) {
		t.Fatal("dry_run must skip side effects")
	}
	if !ShouldSkipSideEffects(ExecutionModeSandbox) {
		t.Fatal("sandbox must skip side effects until connector sandbox routing exists")
	}
	if ShouldSkipSideEffects(ExecutionModeProduction) {
		t.Fatal("production must not skip side effects")
	}
}

func TestCanExecuteMCPToolByLifecycleStatus(t *testing.T) {
	cases := []struct {
		name    string
		status  string
		mode    string
		wantErr bool
	}{
		{name: "published production allowed", status: SkillStatusPublished, mode: ExecutionModeProduction, wantErr: false},
		{name: "active production compatibility allowed", status: "active", mode: ExecutionModeProduction, wantErr: false},
		{name: "draft sandbox allowed", status: SkillStatusDraft, mode: ExecutionModeSandbox, wantErr: false},
		{name: "draft production denied", status: SkillStatusDraft, mode: ExecutionModeProduction, wantErr: true},
		{name: "testing production denied", status: SkillStatusTesting, mode: ExecutionModeProduction, wantErr: true},
		{name: "disabled sandbox denied", status: SkillStatusDisabled, mode: ExecutionModeSandbox, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CanExecuteMCPTool(models.MCPTool{Status: tc.status}, tc.mode)
			if (err != nil) != tc.wantErr {
				t.Fatalf("CanExecuteMCPTool(status=%q, mode=%q) err=%v, wantErr=%v", tc.status, tc.mode, err, tc.wantErr)
			}
		})
	}
}
