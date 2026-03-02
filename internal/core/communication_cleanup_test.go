package core

import (
	"os"
	"strings"
	"testing"
)

func TestCoreNoRedundantCommunicationAbstractions(t *testing.T) {
	content, err := os.ReadFile("plugin.go")
	if err != nil {
		t.Fatalf("read plugin.go: %v", err)
	}

	src := string(content)
	for _, legacy := range []string{
		"SlotAgent",
		"SlotRuntime",
	} {
		if strings.Contains(src, legacy) {
			t.Fatalf("legacy reference still exists in core plugin slots: %s", legacy)
		}
	}
}
