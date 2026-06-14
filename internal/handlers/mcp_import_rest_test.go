package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestImportOpenAPIRequestPreservesRESTfulMethod(t *testing.T) {
	var req ImportOpenAPIRequest
	err := json.Unmarshal([]byte(`{
		"name":"UpdateUser",
		"base_url":"https://api.example.com",
		"api_path":"/users/{id}",
		"method":"PUT",
		"description":"update user"
	}`), &req)
	if err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	if req.Method != "PUT" {
		t.Fatalf("expected method PUT, got %q", req.Method)
	}
	if req.APIPath != "/users/{id}" {
		t.Fatalf("expected api path preserved, got %q", req.APIPath)
	}
}

func TestNormalizeRESTImportDefaultsToDraftDisabledTool(t *testing.T) {
	got, err := normalizeRESTImportRequest(ImportOpenAPIRequest{
		ConnectorID: "conn-1",
		Name:        "get_user",
		APIPath:     "/users/{id}",
		Method:      "get",
	})
	if err != nil {
		t.Fatalf("normalizeRESTImportRequest returned error: %v", err)
	}
	if got.Method != "GET" {
		t.Fatalf("expected method GET, got %q", got.Method)
	}
	if got.InputSchema != `{"type":"object","properties":{}}` {
		t.Fatalf("expected default empty object schema, got %s", got.InputSchema)
	}
	if got.Status != "draft" {
		t.Fatalf("REST import should default to draft, got %q", got.Status)
	}
	if got.RiskLevel != "medium" {
		t.Fatalf("REST import should default risk to medium, got %q", got.RiskLevel)
	}
	if got.Enabled {
		t.Fatalf("draft REST import should default to disabled to avoid production exposure")
	}
}

func TestNormalizeRESTImportRejectsInvalidInputSchema(t *testing.T) {
	_, err := normalizeRESTImportRequest(ImportOpenAPIRequest{
		ConnectorID:  "conn-1",
		Name:         "bad_schema_tool",
		APIPath:      "/bad",
		Method:       "POST",
		InputSchema:  `{"type":"object","properties":`,
		Status:       "testing",
		RiskLevel:    "high",
		EnabledValue: boolPtr(false),
	})
	if err == nil {
		t.Fatalf("expected invalid input_schema to be rejected")
	}
	if !strings.Contains(err.Error(), "input_schema") {
		t.Fatalf("expected error to mention input_schema, got %v", err)
	}
}

func TestNormalizeRESTImportAcceptsExplicitGovernanceConfig(t *testing.T) {
	enabled := true
	got, err := normalizeRESTImportRequest(ImportOpenAPIRequest{
		ConnectorID:  "conn-1",
		Name:         "create_order",
		APIPath:      "/orders",
		Method:       "post",
		InputSchema:  `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`,
		Status:       "published",
		RiskLevel:    "high",
		EnabledValue: &enabled,
		Description:  "创建订单",
	})
	if err != nil {
		t.Fatalf("normalizeRESTImportRequest returned error: %v", err)
	}
	if got.Status != "published" || got.RiskLevel != "high" || !got.Enabled {
		t.Fatalf("unexpected governance config: status=%q risk=%q enabled=%v", got.Status, got.RiskLevel, got.Enabled)
	}
}

func TestNormalizeRESTImportRejectsRelativePathWithoutLeadingSlash(t *testing.T) {
	_, err := normalizeRESTImportRequest(ImportOpenAPIRequest{
		ConnectorID: "conn-1",
		Name:        "bad_path",
		APIPath:     "users/{id}",
		Method:      "GET",
	})
	if err == nil {
		t.Fatalf("expected api_path without leading slash to be rejected")
	}
	if !strings.Contains(err.Error(), "api_path") {
		t.Fatalf("expected error to mention api_path, got %v", err)
	}
}

func TestNormalizeRESTImportRejectsRequiredFieldsMissingFromProperties(t *testing.T) {
	_, err := normalizeRESTImportRequest(ImportOpenAPIRequest{
		ConnectorID: "conn-1",
		Name:        "bad_required",
		APIPath:     "/orders",
		Method:      "POST",
		InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["missing"]}`,
	})
	if err == nil {
		t.Fatalf("expected schema with required field absent from properties to be rejected")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Fatalf("expected error to mention required, got %v", err)
	}
}

func TestNormalizeRESTImportRejectsEnabledToolThatIsNotPublished(t *testing.T) {
	enabled := true
	_, err := normalizeRESTImportRequest(ImportOpenAPIRequest{
		ConnectorID:  "conn-1",
		Name:         "draft_enabled",
		APIPath:      "/orders",
		Method:       "POST",
		Status:       "draft",
		EnabledValue: &enabled,
	})
	if err == nil {
		t.Fatalf("expected enabled draft tool to be rejected")
	}
	if !strings.Contains(err.Error(), "published") {
		t.Fatalf("expected error to mention published, got %v", err)
	}
}

func boolPtr(v bool) *bool { return &v }
