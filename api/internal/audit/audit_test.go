package audit

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestRedactMasksSensitiveFieldsAtAnyDepth(t *testing.T) {
	type nested struct {
		QRToken string `json:"qr_token"`
		Note    string `json:"note"`
	}
	in := map[string]any{
		"name":               "Siti",
		"password_hash":      "$argon2id$secret",
		"Temporary_Password": "Abc123xyz9",
		"refresh_token":      "r",
		"token_hash":         "h",
		"api_secret":         "s",
		"credentials":        []string{"a"},
		"room":               nested{QRToken: "door-token", Note: "ok"},
		"list":               []any{map[string]any{"access_token": "a", "keep": 1}},
	}

	raw, err := redact(in)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, leaked := range []string{"argon2id", "Abc123xyz9", "door-token", `"r"`, `"h"`, `"s"`} {
		if strings.Contains(text, leaked) {
			t.Errorf("redacted output still contains %q: %s", leaked, text)
		}
	}

	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["name"] != "Siti" {
		t.Errorf("name = %v, want it untouched", out["name"])
	}
	if out["password_hash"] != Redacted || out["Temporary_Password"] != Redacted {
		t.Errorf("sensitive fields not masked: %v", out)
	}
	room := out["room"].(map[string]any)
	if room["qr_token"] != Redacted || room["note"] != "ok" {
		t.Errorf("nested struct = %v", room)
	}
	item := out["list"].([]any)[0].(map[string]any)
	if item["access_token"] != Redacted || item["keep"] != float64(1) {
		t.Errorf("array element = %v", item)
	}
}

func TestRedactKeepsNilAsNull(t *testing.T) {
	raw, err := redact(nil)
	if err != nil || raw != nil {
		t.Errorf("redact(nil) = %q, %v; want nil, nil", raw, err)
	}
}

func TestRedactLeavesPlainValuesAlone(t *testing.T) {
	raw, err := redact(map[string]any{"role": "TU", "is_active": true, "count": 3})
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out["role"] != "TU" || out["is_active"] != true || out["count"] != float64(3) {
		t.Errorf("plain values changed: %v", out)
	}
}
