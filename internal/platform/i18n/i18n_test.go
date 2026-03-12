package i18n

import (
	"net/http"
	"testing"
)

func TestDetectLang(t *testing.T) {
	tests := []struct {
		accept string
		want   Lang
	}{
		{"", LangEN},
		{"en-US,en;q=0.9", LangEN},
		{"zh-CN,zh;q=0.9,en;q=0.8", LangZhCN},
		{"zh-TW,zh;q=0.9", LangZhCN},
		{"ja,zh-CN;q=0.8", LangZhCN},
		{"fr-FR,fr;q=0.9,en;q=0.8", LangEN},
	}
	for _, tt := range tests {
		r, _ := http.NewRequest("GET", "/", nil)
		if tt.accept != "" {
			r.Header.Set("Accept-Language", tt.accept)
		}
		got := DetectLang(r)
		if got != tt.want {
			t.Errorf("DetectLang(%q) = %q, want %q", tt.accept, got, tt.want)
		}
	}
}

func TestTranslate(t *testing.T) {
	// English returns fallback
	got := Translate(LangEN, "NOT_FOUND", "not found")
	if got != "not found" {
		t.Errorf("Translate(EN) = %q, want %q", got, "not found")
	}

	// Chinese returns translation
	got = Translate(LangZhCN, "NOT_FOUND", "not found")
	if got != "资源未找到" {
		t.Errorf("Translate(ZhCN, NOT_FOUND) = %q, want %q", got, "资源未找到")
	}

	// Unknown code returns fallback
	got = Translate(LangZhCN, "UNKNOWN_CODE", "some error")
	if got != "some error" {
		t.Errorf("Translate(ZhCN, UNKNOWN_CODE) = %q, want %q", got, "some error")
	}
}
