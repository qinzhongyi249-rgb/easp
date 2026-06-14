package handlers

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestAssistantActivityTrackerRefreshesIdleDeadline(t *testing.T) {
	base := time.Date(2026, 6, 14, 13, 0, 0, 0, time.UTC)
	tracker := newAssistantActivityTracker(30 * time.Second)
	tracker.touchAt(base)

	if tracker.idleTimedOutAt(base.Add(29 * time.Second)) {
		t.Fatalf("should not timeout before idle deadline")
	}
	tracker.touchAt(base.Add(25 * time.Second))
	if tracker.idleTimedOutAt(base.Add(54 * time.Second)) {
		t.Fatalf("status/data activity should refresh idle deadline")
	}
	if !tracker.idleTimedOutAt(base.Add(56 * time.Second)) {
		t.Fatalf("should timeout only after no activity for idle window")
	}
}

func TestSystemPromptIsConciseAndPrefersShortOutput(t *testing.T) {
	prompt := getSystemPrompt("tenant-a", []string{"list_users", "update_user", "builtin_test_curl_import", "builtin_create_skill"})
	if !strings.Contains(prompt, "输出尽量精简") {
		t.Fatalf("expected concise output instruction, got: %s", prompt)
	}
	if len([]rune(prompt)) > 500 {
		t.Fatalf("system prompt should stay compact, got %d runes", len([]rune(prompt)))
	}
}

func TestAssistantDeltaBufferChunksLongContent(t *testing.T) {
	base := time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC)
	buf := newAssistantDeltaBuffer(8, 32, 80*time.Millisecond)

	longText := "这是一段很长的回答内容，用来验证后端不会等完整段落生成后才一次性返回给前端，而是拆成多个较小片段。"
	chunks := buf.push(longText, base, false)
	chunks = append(chunks, buf.push("", base.Add(10*time.Millisecond), true)...)

	if len(chunks) < 2 {
		t.Fatalf("expected long content to be split into multiple chunks, got %d: %#v", len(chunks), chunks)
	}
	for _, chunk := range chunks {
		if got := len([]rune(chunk)); got > 32 {
			t.Fatalf("chunk should be capped around 5-10 tokens / small text slices, got %d runes: %q", got, chunk)
		}
	}
	if strings.Join(chunks, "") != longText {
		t.Fatalf("chunks should preserve content order")
	}
}

func TestAssistantDeltaBufferFlushesSmallChunkAfterInterval(t *testing.T) {
	base := time.Date(2026, 6, 14, 18, 0, 0, 0, time.UTC)
	buf := newAssistantDeltaBuffer(8, 32, 80*time.Millisecond)

	if chunks := buf.push("短句", base, false); len(chunks) != 0 {
		t.Fatalf("small chunk should wait briefly for smoother output, got %#v", chunks)
	}
	chunks := buf.push("", base.Add(90*time.Millisecond), false)
	if len(chunks) != 1 || chunks[0] != "短句" {
		t.Fatalf("small chunk should flush after interval, got %#v", chunks)
	}
}

func TestRequiresInputReplyUsesChineseFieldMeaningForChineseUser(t *testing.T) {
	result := map[string]any{
		"status": "requires_input",
		"outputs": map[string]any{
			"message":        "创建用户需要补充邮箱和显示名称。角色名称可选。",
			"missing_fields": []string{"email"},
		},
	}
	data, _ := json.Marshal(result)

	reply, ok := requiresInputReplyFromToolResult(string(data), "请帮我创建一个用户")
	if !ok {
		t.Fatalf("expected requires_input reply")
	}
	if strings.Contains(reply, "email") {
		t.Fatalf("reply should not expose raw English field name: %s", reply)
	}
	if !strings.Contains(reply, "邮箱") {
		t.Fatalf("reply should explain email as 邮箱, got: %s", reply)
	}
}
