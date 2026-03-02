package agentcodex

import (
	"bufio"
	"encoding/json"
	"io"
	"time"

	"github.com/user/ai-workflow/internal/core"
)

type CodexStreamParser struct {
	scanner *bufio.Scanner
}

func NewCodexStreamParser(r io.Reader) *CodexStreamParser {
	return &CodexStreamParser{scanner: bufio.NewScanner(r)}
}

func (p *CodexStreamParser) Next() (*core.StreamEvent, error) {
	for p.scanner.Scan() {
		line := p.scanner.Text()
		if line == "" {
			continue
		}

		// When `codex exec --json` is used, stdout is JSONL with events.
		// We only care about the final assistant message.
		var evt struct {
			Type string `json:"type"`
			Item *struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"item"`
		}
		if err := json.Unmarshal([]byte(line), &evt); err != nil {
			// Fallback: treat unexpected lines as plain text.
			return &core.StreamEvent{Type: "text", Content: line, Timestamp: time.Now()}, nil
		}

		if evt.Type == "item.completed" && evt.Item != nil && evt.Item.Type == "agent_message" {
			content := evt.Item.Text
			if content == "" {
				continue
			}
			return &core.StreamEvent{Type: "done", Content: content, Timestamp: time.Now()}, nil
		}
		// Ignore other event types.
	}
	if err := p.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}
