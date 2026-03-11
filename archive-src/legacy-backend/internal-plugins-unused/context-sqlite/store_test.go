package contextsqlite

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/yoke233/ai-workflow/internal/core"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := New(path)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCRUD(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	uri := "viking://resources/42/specs/101/requirement.md"
	content := []byte("# Requirement\nDo something.")

	// Write
	if err := s.Write(ctx, uri, content); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// Read
	got, err := s.Read(ctx, uri)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("Read: got %q, want %q", got, content)
	}

	// List
	entries, err := s.List(ctx, "viking://resources/42/specs/101/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(entries) != 1 || entries[0].Name != "requirement.md" {
		t.Fatalf("List: got %+v", entries)
	}

	// Remove
	if err := s.Remove(ctx, uri); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	// Read after remove -> error
	if _, err := s.Read(ctx, uri); err == nil {
		t.Fatal("expected error after Remove, got nil")
	}
}

func TestListDirectories(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_ = s.Write(ctx, "viking://r/docs/a.md", []byte("a"))
	_ = s.Write(ctx, "viking://r/docs/b.md", []byte("b"))
	_ = s.Write(ctx, "viking://r/docs/sub/c.md", []byte("c"))

	entries, err := s.List(ctx, "viking://r/docs/")
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	wantNames := map[string]bool{"a.md": true, "b.md": true, "sub": true}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if !wantNames[e.Name] {
			t.Errorf("unexpected entry: %+v", e)
		}
		if e.Name == "sub" && !e.IsDir {
			t.Error("sub should be a directory")
		}
	}
}

func TestSession(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sess, err := s.CreateSession(ctx, "test-session-1")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID() != "test-session-1" {
		t.Fatalf("ID: got %q", sess.ID())
	}

	if err := sess.AddMessage("user", []core.MessagePart{
		{Type: "text", Content: "hello"},
	}); err != nil {
		t.Fatalf("AddMessage: %v", err)
	}

	if err := sess.Used([]string{"viking://resources/42/specs/101/requirement.md"}); err != nil {
		t.Fatalf("Used: %v", err)
	}

	result, err := sess.Commit()
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if result.Status != "committed" {
		t.Fatalf("Commit status: got %q", result.Status)
	}
}

func TestMaterialize(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	_ = s.Write(ctx, "viking://r/specs/101/requirement.md", []byte("req content"))
	_ = s.Write(ctx, "viking://r/specs/101/api.md", []byte("api content"))

	dir := t.TempDir()
	if err := s.Materialize(ctx, "viking://r/specs/101/", dir); err != nil {
		t.Fatalf("Materialize: %v", err)
	}

	for _, tc := range []struct {
		file, want string
	}{
		{"requirement.md", "req content"},
		{"api.md", "api content"},
	} {
		got, err := os.ReadFile(filepath.Join(dir, tc.file))
		if err != nil {
			t.Fatalf("read %s: %v", tc.file, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %q, want %q", tc.file, got, tc.want)
		}
	}
}

func TestFindSearchReturnEmpty(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	results, err := s.Find(ctx, "anything", core.FindOpts{})
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Find: expected empty, got %d", len(results))
	}

	results, err = s.Search(ctx, "anything", "sess", core.SearchOpts{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search: expected empty, got %d", len(results))
	}
}

func TestAbstractOverviewReturnEmpty(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	abs, err := s.Abstract(ctx, "viking://any")
	if err != nil || abs != "" {
		t.Fatalf("Abstract: got %q, %v", abs, err)
	}

	ov, err := s.Overview(ctx, "viking://any")
	if err != nil || ov != "" {
		t.Fatalf("Overview: got %q, %v", ov, err)
	}
}

func TestConcurrentSafety(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		uri := "viking://concurrent/file"
		go func() {
			defer wg.Done()
			_ = s.Write(ctx, uri, []byte("data"))
		}()
		go func() {
			defer wg.Done()
			_, _ = s.Read(ctx, uri)
		}()
	}
	wg.Wait()
}

func TestGetSession(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	sess, err := s.GetSession(ctx, "get-sess")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if sess.ID() != "get-sess" {
		t.Fatalf("ID: got %q", sess.ID())
	}
}

func TestLink(t *testing.T) {
	ctx := context.Background()
	s := newTestStore(t)

	from := "viking://resources/42/specs/101/"
	to := []string{"viking://resources/42/docs/arch.md"}
	if err := s.Link(ctx, from, to, "related"); err != nil {
		t.Fatalf("Link: %v", err)
	}

	// Verify via direct DB query.
	var toURI string
	err := s.db.QueryRow(`SELECT to_uri FROM context_links WHERE from_uri=?`, from).Scan(&toURI)
	if err != nil {
		t.Fatalf("query link: %v", err)
	}
	if toURI != to[0] {
		t.Fatalf("link to_uri: got %q, want %q", toURI, to[0])
	}
}

func TestModule(t *testing.T) {
	m := Module()
	if m.Name != "context-sqlite" {
		t.Fatalf("Name: got %q", m.Name)
	}
	if m.Slot != core.SlotContext {
		t.Fatalf("Slot: got %q", m.Slot)
	}
	tmpDir := t.TempDir()
	p, err := m.Factory(map[string]any{"path": filepath.Join(tmpDir, "test.db")})
	if err != nil {
		t.Fatalf("Factory: %v", err)
	}
	defer p.Close()
	if _, ok := p.(core.ContextStore); !ok {
		t.Fatal("Factory result does not implement ContextStore")
	}
}
