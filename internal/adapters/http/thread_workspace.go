package api

import (
	"context"
	"fmt"

	"github.com/yoke233/ai-workflow/internal/threadctx"
)

type threadWorkspaceManager struct {
	store   Store
	dataDir string
}

func (m threadWorkspaceManager) EnsureThreadWorkspace(_ context.Context, threadID int64) error {
	if _, err := threadctx.EnsureLayout(m.dataDir, threadID); err != nil {
		return fmt.Errorf("ensure thread workspace: %w", err)
	}
	return nil
}

func (m threadWorkspaceManager) SyncThreadWorkspaceContext(ctx context.Context, threadID int64) error {
	if _, err := threadctx.SyncContextFile(ctx, m.store, m.dataDir, threadID); err != nil {
		return fmt.Errorf("sync thread workspace context: %w", err)
	}
	return nil
}
