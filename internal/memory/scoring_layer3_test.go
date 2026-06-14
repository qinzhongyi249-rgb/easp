package memory

import (
	"testing"
	"time"

	"github.com/easp-platform/easp/internal/models"
)

func TestScoreUserMemoryBreakdownIncludesVectorScoreAndExplanation(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	seen := now.Add(-2 * time.Hour)
	mem := models.UserMemory{
		ID:          "m1",
		Type:        "fact",
		Content:     "EASP 使用 Go 后端和 React 前端",
		AccessCount: 2,
		LastSeenAt:  &seen,
		CreatedAt:   now.Add(-24 * time.Hour),
	}

	breakdown := ScoreUserMemoryBreakdown(mem, []string{"EASP"}, map[string]float64{"m1": 0.92}, now)

	if breakdown.MemoryID != "m1" {
		t.Fatalf("expected memory id in breakdown, got %+v", breakdown)
	}
	if breakdown.KeywordScore <= 0 || breakdown.VectorScore <= 0 || breakdown.FinalScore <= 0 {
		t.Fatalf("expected keyword/vector/final scores, got %+v", breakdown)
	}
	if breakdown.Explanation == "" {
		t.Fatalf("expected human-readable explanation, got %+v", breakdown)
	}
}

func TestRankUserMemoriesHybridLetsVectorScoreRecoverSemanticMatches(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	old := now.Add(-60 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	memories := []models.UserMemory{
		{ID: "keyword", Type: "fact", Content: "用户讨论普通报表", AccessCount: 1, LastSeenAt: &recent, CreatedAt: recent},
		{ID: "semantic", Type: "fact", Content: "EASP 是 API-to-MCP 企业级网关", AccessCount: 0, LastSeenAt: &old, CreatedAt: old},
	}

	ranked := RankUserMemoriesHybrid(memories, []string{"普通报表"}, map[string]float64{"semantic": 0.99}, 2, now)

	if len(ranked) != 2 || ranked[0].ID != "semantic" {
		t.Fatalf("expected high vector semantic match first, got %+v", ranked)
	}
}

func TestSelectUserMemoriesToArchiveKeepsHighestValueMemories(t *testing.T) {
	now := time.Date(2026, 6, 13, 10, 0, 0, 0, time.UTC)
	old := now.Add(-120 * 24 * time.Hour)
	recent := now.Add(-1 * time.Hour)
	memories := []models.UserMemory{
		{ID: "old-low", Type: "event", Content: "旧的临时事件", AccessCount: 0, LastSeenAt: &old, CreatedAt: old},
		{ID: "recent-pref", Type: "preference", Content: "用户偏好中文", AccessCount: 8, LastSeenAt: &recent, CreatedAt: recent},
		{ID: "recent-fact", Type: "fact", Content: "用户使用 EASP", AccessCount: 3, LastSeenAt: &recent, CreatedAt: recent},
	}

	archive := SelectUserMemoriesToArchive(memories, 2, now)

	if len(archive) != 1 || archive[0].ID != "old-low" {
		t.Fatalf("expected old low-value memory archived, got %+v", archive)
	}
}
