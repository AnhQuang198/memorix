package httpx

import (
	"encoding/json"
	"testing"
)

func TestErrorEnvelope_Shape(t *testing.T) {
	e := NewError(CodeValidation, "email không hợp lệ").WithField("email", "bắt buộc").WithTrace("trace-123")
	b, _ := json.Marshal(e)
	var got map[string]map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	inner := got["error"]
	if inner["code"] != "VALIDATION_ERROR" {
		t.Errorf("code = %v, want VALIDATION_ERROR", inner["code"])
	}
	if inner["message"] != "email không hợp lệ" {
		t.Errorf("message = %v", inner["message"])
	}
	if inner["trace_id"] != "trace-123" {
		t.Errorf("trace_id = %v", inner["trace_id"])
	}
	if fields, ok := inner["fields"].(map[string]any); !ok || fields["email"] != "bắt buộc" {
		t.Errorf("fields = %v", inner["fields"])
	}
}

func TestErrorEnvelope_HTTPStatus(t *testing.T) {
	cases := map[ErrorCode]int{
		CodeValidation: 400, CodeUnauthenticated: 401, CodeForbidden: 403,
		CodeNotFound: 404, CodeConflict: 409, CodeRateLimited: 429, CodeInternal: 500,
	}
	for code, want := range cases {
		if got := NewError(code, "x").HTTPStatus(); got != want {
			t.Errorf("%s HTTPStatus = %d, want %d", code, got, want)
		}
	}
}
