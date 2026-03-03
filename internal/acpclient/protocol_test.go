package acpclient

import "testing"

func TestToACPClientCapabilities(t *testing.T) {
	out := toACPClientCapabilities(ClientCapabilities{
		FSRead:   true,
		FSWrite:  true,
		Terminal: true,
	})
	if !out.Fs.ReadTextFile || !out.Fs.WriteTextFile || !out.Terminal {
		t.Fatalf("unexpected mapped capabilities: %#v", out)
	}
}
