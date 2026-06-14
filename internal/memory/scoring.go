package memory

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/easp-platform/easp/internal/models"
)

// MemoryMergeDecision describes whether a new memory should be merged into an existing memory.
type MemoryMergeDecision struct {
	Merge         bool
	Conflict      bool
	Similarity    float64
	MergedContent string
	Reason        string
}

// MemoryScoreBreakdown explains why a memory was selected for recall.
// Layer 3 exposes the score components so the backend can audit and the frontend can display them.
type MemoryScoreBreakdown struct {
	MemoryID       string  `json:"memory_id"`
	KeywordScore   float64 `json:"keyword_score"`
	VectorScore    float64 `json:"vector_score"`
	RecencyScore   float64 `json:"recency_score"`
	FrequencyScore float64 `json:"frequency_score"`
	TypeScore      float64 `json:"type_score"`
	FinalScore     float64 `json:"final_score"`
	Explanation    string  `json:"explanation"`
}

// ScoreUserMemory computes a deterministic recall score for a user memory.
// Layer 2 keeps scoring local and explainable: keyword + recency + frequency + type weight.
// vector_score is intentionally not included here yet; it can be added by callers later.
func ScoreUserMemory(mem models.UserMemory, keywords []string, now time.Time) float64 {
	return ScoreUserMemoryBreakdown(mem, keywords, nil, now).FinalScore
}

// ScoreUserMemoryBreakdown computes the full Layer 3 hybrid score.
// Vector score is weighted strongly enough to recover semantic matches that do not share keywords.
func ScoreUserMemoryBreakdown(mem models.UserMemory, keywords []string, vectorScores map[string]float64, now time.Time) MemoryScoreBreakdown {
	keyword := keywordScore(mem.Content, keywords)
	vector := 0.0
	if vectorScores != nil {
		vector = normalizeVectorScore(vectorScores[mem.ID]) * 4.0
	}
	recency := recencyScore(mem, now)
	frequency := math.Log1p(float64(mem.AccessCount)) * 0.25
	typeScore := typeWeight(mem.Type)
	final := keyword + vector + recency + frequency + typeScore
	return MemoryScoreBreakdown{
		MemoryID:       mem.ID,
		KeywordScore:   keyword,
		VectorScore:    vector,
		RecencyScore:   recency,
		FrequencyScore: frequency,
		TypeScore:      typeScore,
		FinalScore:     final,
		Explanation:    fmt.Sprintf("keyword=%.2f vector=%.2f recency=%.2f frequency=%.2f type=%.2f final=%.2f", keyword, vector, recency, frequency, typeScore, final),
	}
}

// RankUserMemories orders memories by composite score and applies limit.
func RankUserMemories(memories []models.UserMemory, keywords []string, limit int, now time.Time) []models.UserMemory {
	return RankUserMemoriesHybrid(memories, keywords, nil, limit, now)
}

// RankUserMemoriesHybrid orders memories by keyword + vector + decay/frequency/type score.
func RankUserMemoriesHybrid(memories []models.UserMemory, keywords []string, vectorScores map[string]float64, limit int, now time.Time) []models.UserMemory {
	ranked := append([]models.UserMemory(nil), memories...)
	sort.SliceStable(ranked, func(i, j int) bool {
		si := ScoreUserMemoryBreakdown(ranked[i], keywords, vectorScores, now).FinalScore
		sj := ScoreUserMemoryBreakdown(ranked[j], keywords, vectorScores, now).FinalScore
		if si == sj {
			return ranked[i].CreatedAt.After(ranked[j].CreatedAt)
		}
		return si > sj
	})
	if limit > 0 && len(ranked) > limit {
		return ranked[:limit]
	}
	return ranked
}

// ExplainUserMemoryRanking returns per-memory scoring details in ranked order.
func ExplainUserMemoryRanking(memories []models.UserMemory, keywords []string, vectorScores map[string]float64, limit int, now time.Time) []MemoryScoreBreakdown {
	ranked := RankUserMemoriesHybrid(memories, keywords, vectorScores, limit, now)
	breakdowns := make([]MemoryScoreBreakdown, 0, len(ranked))
	for _, mem := range ranked {
		breakdowns = append(breakdowns, ScoreUserMemoryBreakdown(mem, keywords, vectorScores, now))
	}
	return breakdowns
}

// SelectUserMemoriesToArchive returns the lowest-value memories when active memories exceed capacity.
func SelectUserMemoriesToArchive(memories []models.UserMemory, maxActive int, now time.Time) []models.UserMemory {
	if maxActive <= 0 || len(memories) <= maxActive {
		return nil
	}
	ranked := RankUserMemoriesHybrid(memories, nil, nil, 0, now)
	archive := append([]models.UserMemory(nil), ranked[maxActive:]...)
	sort.SliceStable(archive, func(i, j int) bool {
		return ScoreUserMemory(archive[i], nil, now) < ScoreUserMemory(archive[j], nil, now)
	})
	return archive
}

// ShouldMergeUserMemory decides if incoming content is similar enough to merge into an existing memory.
// Conflicting negations are never auto-merged.
func ShouldMergeUserMemory(existing models.UserMemory, incoming string) MemoryMergeDecision {
	similarity := tokenJaccard(existing.Content, incoming)
	conflict := hasNegationConflict(existing.Content, incoming)
	decision := MemoryMergeDecision{Similarity: similarity, Conflict: conflict}
	if conflict {
		decision.Reason = "similar content contains negation conflict"
		return decision
	}
	if similarity >= 0.45 {
		decision.Merge = true
		decision.MergedContent = mergeContent(existing.Content, incoming)
		decision.Reason = "similar non-conflicting memory"
		return decision
	}
	decision.Reason = "similarity below merge threshold"
	return decision
}

func keywordScore(content string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}
	contentLower := strings.ToLower(content)
	score := 0.0
	seen := map[string]bool{}
	for _, kw := range keywords {
		kw = strings.ToLower(strings.TrimSpace(kw))
		if kw == "" || seen[kw] {
			continue
		}
		seen[kw] = true
		if strings.Contains(contentLower, kw) {
			score += 1.2
		}
	}
	return score
}

func recencyScore(mem models.UserMemory, now time.Time) float64 {
	reference := mem.CreatedAt
	if mem.LastSeenAt != nil && mem.LastSeenAt.After(reference) {
		reference = *mem.LastSeenAt
	}
	if mem.LastAccessedAt != nil && mem.LastAccessedAt.After(reference) {
		reference = *mem.LastAccessedAt
	}
	if reference.IsZero() || reference.After(now) {
		return 0
	}
	days := now.Sub(reference).Hours() / 24
	// 30-day half-life style decay. Recent memory approaches 1.5, old memory approaches 0.
	return 1.5 / (1 + days/30)
}

func typeWeight(memType string) float64 {
	switch strings.ToLower(memType) {
	case "preference":
		return 0.8
	case "fact":
		return 0.45
	case "feedback":
		return 0.35
	case "event":
		return 0.2
	default:
		return 0.1
	}
}

func normalizeVectorScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 1 {
		return 1
	}
	return score
}

func tokenJaccard(a, b string) float64 {
	as := tokenSet(a)
	bs := tokenSet(b)
	if len(as) == 0 || len(bs) == 0 {
		return 0
	}
	intersection := 0
	for token := range as {
		if bs[token] {
			intersection++
		}
	}
	union := len(as) + len(bs) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func tokenSet(s string) map[string]bool {
	normalized := strings.ToLower(s)
	parts := strings.FieldsFunc(normalized, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
	})
	set := map[string]bool{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		set[p] = true
		// Chinese memory often has no spaces; add rune-level fallback for CJK-ish text.
		if len([]rune(p)) > 1 {
			for _, r := range p {
				if unicode.Is(unicode.Han, r) {
					set[string(r)] = true
				}
			}
		}
	}
	return set
}

func hasNegationConflict(a, b string) bool {
	na := NormalizeMemoryContent(a)
	nb := NormalizeMemoryContent(b)
	negativeMarkers := []string{"不", "不喜欢", "不要", "不想", "禁止", "别", "非", "不能"}
	aNeg, bNeg := false, false
	for _, marker := range negativeMarkers {
		if strings.Contains(na, marker) {
			aNeg = true
		}
		if strings.Contains(nb, marker) {
			bNeg = true
		}
	}
	if aNeg == bNeg {
		return false
	}
	strippedA := stripNegationMarkers(na, negativeMarkers)
	strippedB := stripNegationMarkers(nb, negativeMarkers)
	return tokenJaccard(strippedA, strippedB) >= 0.45 || strings.Contains(strippedA, strippedB) || strings.Contains(strippedB, strippedA)
}

func stripNegationMarkers(s string, markers []string) string {
	for _, marker := range markers {
		s = strings.ReplaceAll(s, marker, "")
	}
	return s
}

func mergeContent(existing, incoming string) string {
	existing = strings.TrimSpace(existing)
	incoming = strings.TrimSpace(incoming)
	if existing == "" {
		return incoming
	}
	if incoming == "" || NormalizeMemoryContent(existing) == NormalizeMemoryContent(incoming) {
		return existing
	}
	if strings.Contains(existing, incoming) {
		return existing
	}
	if strings.Contains(incoming, existing) {
		return incoming
	}
	return existing + "；" + incoming
}
