package memory

import (
	"strings"
	"testing"
	"time"

	"github.com/easp-platform/easp/internal/models"
)

func TestScoreUserMemoryPrefersRecentFrequentlyUsedRelevantMemory(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	recentSeen := now.Add(-2 * time.Hour)
	oldSeen := now.Add(-45 * 24 * time.Hour)

	recent := models.UserMemory{
		ID:             "recent",
		Type:           "preference",
		Content:        "用户偏好使用中文简洁回答",
		AccessCount:    8,
		LastAccessedAt: &recentSeen,
		LastSeenAt:     &recentSeen,
		CreatedAt:      now.Add(-3 * 24 * time.Hour),
	}
	old := models.UserMemory{
		ID:             "old",
		Type:           "fact",
		Content:        "用户偏好使用中文回答",
		AccessCount:    0,
		LastAccessedAt: &oldSeen,
		LastSeenAt:     &oldSeen,
		CreatedAt:      now.Add(-90 * 24 * time.Hour),
	}

	recentScore := ScoreUserMemory(recent, []string{"中文", "回答"}, now)
	oldScore := ScoreUserMemory(old, []string{"中文", "回答"}, now)

	if recentScore <= oldScore {
		t.Fatalf("expected recent/frequent preference to outrank old fact: recent=%f old=%f", recentScore, oldScore)
	}
}

func TestRankUserMemoriesOrdersByCompositeScore(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	recent := now.Add(-1 * time.Hour)
	old := now.Add(-60 * 24 * time.Hour)
	memories := []models.UserMemory{
		{ID: "old", Type: "fact", Content: "用户喜欢中文输出", AccessCount: 0, LastSeenAt: &old, CreatedAt: old},
		{ID: "recent", Type: "preference", Content: "用户喜欢中文输出并要求简洁", AccessCount: 5, LastSeenAt: &recent, CreatedAt: recent},
	}

	ranked := RankUserMemories(memories, []string{"中文", "简洁"}, 2, now)

	if len(ranked) != 2 || ranked[0].ID != "recent" {
		t.Fatalf("expected recent memory first, got %+v", ranked)
	}
}

func TestShouldMergeUserMemoryAllowsSimilarNonConflictingContent(t *testing.T) {
	existing := models.UserMemory{ID: "m1", Type: "preference", Content: "用户喜欢中文回答"}
	decision := ShouldMergeUserMemory(existing, "用户喜欢中文简洁回答")

	if !decision.Merge || decision.Conflict {
		t.Fatalf("expected merge without conflict, got %+v", decision)
	}
	if !strings.Contains(decision.MergedContent, "用户喜欢中文回答") || !strings.Contains(decision.MergedContent, "用户喜欢中文简洁回答") {
		t.Fatalf("merged content should preserve both facts, got %q", decision.MergedContent)
	}
}

func TestShouldMergeUserMemoryDetectsConflict(t *testing.T) {
	existing := models.UserMemory{ID: "m1", Type: "preference", Content: "用户喜欢中文回答"}
	decision := ShouldMergeUserMemory(existing, "用户不喜欢中文回答")

	if decision.Merge || !decision.Conflict {
		t.Fatalf("expected conflict without merge, got %+v", decision)
	}
}
