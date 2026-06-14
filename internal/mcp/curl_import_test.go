package mcp

import (
	"encoding/json"
	"testing"
)

func TestParseCurlCommandBuildsImportCandidate(t *testing.T) {
	candidate, err := ParseCurlImportCommand(`curl -X POST 'https://api.example.com/v1/users' -H 'X-Test-Token: demo' -H 'Content-Type: application/json' -d '{"name":"alice"}'`)
	if err != nil {
		t.Fatalf("ParseCurlImportCommand returned error: %v", err)
	}
	if candidate.Method != "POST" {
		t.Fatalf("expected POST, got %q", candidate.Method)
	}
	if candidate.BaseURL != "https://api.example.com" {
		t.Fatalf("expected base URL https://api.example.com, got %q", candidate.BaseURL)
	}
	if candidate.Path != "/v1/users" {
		t.Fatalf("expected path /v1/users, got %q", candidate.Path)
	}
	if candidate.Headers["X-Test-Token"] == "" {
		t.Fatalf("expected X-Test-Token header to be parsed")
	}
	var schema map[string]any
	if err := json.Unmarshal([]byte(candidate.InputSchema), &schema); err != nil {
		t.Fatalf("input schema is not valid JSON: %v", err)
	}
	props := schema["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Fatalf("expected body field name to be included in schema, got %v", props)
	}
}

func TestCreateToolFromCurlRequiresSuccessfulTestResult(t *testing.T) {
	_, err := BuildMCPToolFromCurlTestResult("tenant-1", CurlImportCreateRequest{
		Name: "create_user",
	}, nil)
	if err == nil {
		t.Fatalf("creating MCP tool from curl without successful test result should be rejected")
	}
}
