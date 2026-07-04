package handlers

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

// containsHallucinatedToolCall 快速判定文本里是否有需要被 parseXMLToolCalls 处理的模式。
// 用作调用 fallback 前的廉价前置门，避免对纯文本回复也跑一遍正则。
func containsHallucinatedToolCall(content string) bool {
	if content == "" {
		return false
	}
	if strings.Contains(content, "<invoke") {
		return true
	}
	if strings.Contains(content, "<parameter") && looseParameterHasToolMarker(content) {
		return true
	}
	if jsonHallucinateRe.MatchString(content) {
		return true
	}
	return false
}

// parseXMLToolCalls 处理模型把工具调用当纯文本输出的多种 hallucinate 格式：
//
//  1. 标准 Claude/XML: <tool_use><invoke name="mcp_x"><parameter name="k">v</parameter></invoke></tool_use>
//  2. 裸 invoke:      <invoke name="mcp_x"><parameter name="k">v</parameter></invoke>
//  3. 裸 parameter（无 invoke 包裹）：正文里有 <parameter name="tool">mcp_x</parameter> + 多个 <parameter name="X">Y</parameter>
//  4. JSON hallucinate: {"tool":"mcp_x","arguments":{"k":"v"}} 或 {"name":"mcp_x","arguments":{...}}
//
// 未识别到时返回空 slice 和原 content。
func parseXMLToolCalls(content string) (calls []ToolCall, cleanedContent string) {
	if content == "" {
		return nil, content
	}
	// 优先按标签识别（1 & 2）；若没有 invoke 再退回宽松模式（3 & 4）
	if strings.Contains(content, "<invoke") {
		return parseInvokeXML(content)
	}
	if strings.Contains(content, "<parameter") && looseParameterHasToolMarker(content) {
		return parseLooseParameters(content)
	}
	if strings.Contains(content, "\"tool\"") || strings.Contains(content, "\"name\"") {
		if c, cleaned, ok := parseJSONToolCall(content); ok {
			return c, cleaned
		}
	}
	return nil, content
}

var (
	invokeRe      = regexp.MustCompile(`(?s)<invoke\s+name="([^"]+)"\s*>(.*?)</invoke>`)
	paramRe       = regexp.MustCompile(`(?s)<parameter\s+name="([^"]+)"\s*>(.*?)</parameter>`)
	toolUseOpenRe = regexp.MustCompile(`<tool_use\s*>`)
	toolUseClose  = regexp.MustCompile(`</tool_use\s*>`)
)

// parseInvokeXML 处理标准/裸 invoke 格式。
func parseInvokeXML(content string) (calls []ToolCall, cleanedContent string) {
	matches := invokeRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil, content
	}
	removeRanges := make([][2]int, 0, len(matches))
	for _, m := range matches {
		start, end := m[0], m[1]
		name := content[m[2]:m[3]]
		body := content[m[4]:m[5]]
		args := parseParametersInside(body)
		argsJSON, _ := json.Marshal(args)
		calls = append(calls, toolCallOf(len(calls)+1, name, string(argsJSON)))
		removeRanges = append(removeRanges, [2]int{start, end})
	}
	cleanedContent = removeByteRanges(content, removeRanges)
	cleanedContent = toolUseOpenRe.ReplaceAllString(cleanedContent, "")
	cleanedContent = toolUseClose.ReplaceAllString(cleanedContent, "")
	return calls, strings.TrimSpace(cleanedContent)
}

// parseParametersInside 从 <invoke> body 里解析 <parameter> 列表；
// 单一 requestBody/body/args 参数当 JSON payload 优先展开。
func parseParametersInside(body string) map[string]any {
	args := map[string]any{}
	paramMatches := paramRe.FindAllStringSubmatch(body, -1)
	if len(paramMatches) == 1 {
		key := paramMatches[0][1]
		if key == "requestBody" || key == "body" || key == "args" || key == "arguments" {
			raw := strings.TrimSpace(paramMatches[0][2])
			var parsed map[string]any
			if err := json.Unmarshal([]byte(raw), &parsed); err == nil {
				return parsed
			}
			args[key] = raw
			return args
		}
	}
	for _, pm := range paramMatches {
		args[pm[1]] = coerceValue(pm[2])
	}
	return args
}

// looseParameterHasToolMarker 判定"裸 parameter"模式：正文里必须有
// <parameter name="tool"|"name"|"tool_name">... 之类明确指定工具名的标签。
func looseParameterHasToolMarker(content string) bool {
	return regexp.MustCompile(`<parameter\s+name="(tool|name|tool_name)"\s*>`).MatchString(content)
}

// parseLooseParameters 处理没有 invoke 标签的裸 parameter 序列。
func parseLooseParameters(content string) (calls []ToolCall, cleanedContent string) {
	paramMatches := paramRe.FindAllStringSubmatchIndex(content, -1)
	if len(paramMatches) == 0 {
		return nil, content
	}
	// 第一遍：确定工具名。优先 tool > tool_name > name。
	var toolName, toolKey string
	for _, prio := range []string{"tool", "tool_name", "name"} {
		for _, m := range paramMatches {
			if content[m[2]:m[3]] == prio {
				toolName = strings.TrimSpace(content[m[4]:m[5]])
				toolKey = prio
				break
			}
		}
		if toolName != "" {
			break
		}
	}
	if toolName == "" {
		return nil, content
	}
	// 第二遍：其余参数收集。跳过第一个匹配到的 toolKey，之后同 key 视为普通参数。
	args := map[string]any{}
	seenToolKey := false
	minStart, maxEnd := -1, -1
	for _, m := range paramMatches {
		key := content[m[2]:m[3]]
		val := strings.TrimSpace(content[m[4]:m[5]])
		if minStart == -1 {
			minStart = m[0]
		}
		if m[1] > maxEnd {
			maxEnd = m[1]
		}
		if key == toolKey && !seenToolKey {
			seenToolKey = true
			continue
		}
		switch key {
		case "requestBody", "body", "arguments":
			var parsed map[string]any
			if err := json.Unmarshal([]byte(val), &parsed); err == nil {
				for k, v := range parsed {
					args[k] = v
				}
			} else {
				args[key] = val
			}
		default:
			args[key] = coerceValue(val)
		}
	}
	argsJSON, _ := json.Marshal(args)
	calls = append(calls, toolCallOf(1, toolName, string(argsJSON)))
	cleaned := content
	if minStart >= 0 && maxEnd > minStart {
		cleaned = content[:minStart] + content[maxEnd:]
	}
	// 顺便清理 hallucinate 里常见的 {"tool":"...","arguments":{...}} 尾巴
	cleaned = jsonHallucinateRe.ReplaceAllString(cleaned, "")
	return calls, strings.TrimSpace(cleaned)
}

// jsonHallucinateRe 匹配模型顺手输出的 {"tool":"x","arguments":{...}} / {"name":"x","arguments":{...}}
var jsonHallucinateRe = regexp.MustCompile(`(?s)\{[^{}]*"(?:tool|name|tool_name)"\s*:\s*"[^"]+"\s*,\s*"arguments"\s*:\s*\{[^{}]*\}\s*\}`)

// parseJSONToolCall 处理 JSON hallucinate（{"tool":"x","arguments":{...}}）。
func parseJSONToolCall(content string) (calls []ToolCall, cleanedContent string, ok bool) {
	m := jsonHallucinateRe.FindStringIndex(content)
	if m == nil {
		return nil, content, false
	}
	raw := content[m[0]:m[1]]
	var obj struct {
		Tool     string          `json:"tool"`
		Name     string          `json:"name"`
		ToolName string          `json:"tool_name"`
		Args     json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return nil, content, false
	}
	name := obj.Tool
	if name == "" {
		name = obj.Name
	}
	if name == "" {
		name = obj.ToolName
	}
	if name == "" {
		return nil, content, false
	}
	args := map[string]any{}
	_ = json.Unmarshal(obj.Args, &args)
	argsJSON, _ := json.Marshal(args)
	calls = append(calls, toolCallOf(1, name, string(argsJSON)))
	cleaned := content[:m[0]] + content[m[1]:]
	return calls, strings.TrimSpace(cleaned), true
}

func toolCallOf(idx int, name, argsJSON string) ToolCall {
	tc := ToolCall{
		ID:   "xmltool_" + strconv.Itoa(idx),
		Type: "function",
	}
	tc.Function.Name = name
	tc.Function.Arguments = argsJSON
	return tc
}

// coerceValue 把 XML 里的字符串值强行推断为 JSON 原生类型。
func coerceValue(raw string) any {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	switch strings.ToLower(s) {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	if (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) || (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) {
		var v any
		if err := json.Unmarshal([]byte(s), &v); err == nil {
			return v
		}
	}
	return s
}

// removeByteRanges 删除多个不重叠区间（按起点排序倒序删除）。
func removeByteRanges(s string, ranges [][2]int) string {
	if len(ranges) == 0 {
		return s
	}
	result := s
	for i := len(ranges) - 1; i >= 0; i-- {
		r := ranges[i]
		if r[0] < 0 || r[1] > len(result) || r[0] >= r[1] {
			continue
		}
		result = result[:r[0]] + result[r[1]:]
	}
	return result
}
