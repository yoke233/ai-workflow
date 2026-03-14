package api

import (
	"context"
	"fmt"
	"net/http"

	threadapp "github.com/yoke233/ai-workflow/internal/application/threadapp"
	"github.com/yoke233/ai-workflow/internal/core"
)

type threadAppRuntime struct {
	handler *Handler
}

func (r threadAppRuntime) CleanupThread(ctx context.Context, threadID int64) error {
	if r.handler == nil || r.handler.threadPool == nil {
		return nil
	}
	return r.handler.threadPool.CleanupThread(ctx, threadID)
}

func (h *Handler) threadService() *threadapp.Service {
	if h == nil {
		return nil
	}
	var tx threadapp.Tx
	if txStore, ok := h.store.(core.TransactionalStore); ok {
		tx = threadAppTx{store: txStore}
	}
	return threadapp.New(threadapp.Config{
		Store:   h.store,
		Tx:      tx,
		Runtime: threadAppRuntime{handler: h},
	})
}

type threadAppTx struct {
	store core.TransactionalStore
}

func (t threadAppTx) InTx(ctx context.Context, fn func(ctx context.Context, store threadapp.TxStore) error) error {
	if t.store == nil {
		return fmt.Errorf("thread transaction adapter is not configured")
	}
	return t.store.InTx(ctx, func(store core.Store) error {
		txStore, ok := store.(threadapp.TxStore)
		if !ok {
			return fmt.Errorf("transaction store %T does not implement threadapp tx store", store)
		}
		return fn(ctx, txStore)
	})
}

func writeThreadAppError(w http.ResponseWriter, err error) bool {
	switch threadapp.CodeOf(err) {
	case threadapp.CodeThreadNotFound:
		writeError(w, http.StatusNotFound, "thread not found", threadapp.CodeThreadNotFound)
	case threadapp.CodeWorkItemNotFound:
		writeError(w, http.StatusNotFound, "work item not found", threadapp.CodeWorkItemNotFound)
	case threadapp.CodeLinkNotFound:
		writeError(w, http.StatusNotFound, "link not found", threadapp.CodeLinkNotFound)
	case threadapp.CodeMissingTitle:
		writeError(w, http.StatusBadRequest, "title is required", threadapp.CodeMissingTitle)
	case threadapp.CodeMissingWorkItemID:
		writeError(w, http.StatusBadRequest, "work_item_id is required", threadapp.CodeMissingWorkItemID)
	case threadapp.CodeMissingThreadSummary:
		writeError(w, http.StatusBadRequest, "please generate or fill in summary first", threadapp.CodeMissingThreadSummary)
	case threadapp.CodeCleanupThreadFailed:
		writeError(w, http.StatusInternalServerError, err.Error(), threadapp.CodeCleanupThreadFailed)
	default:
		return false
	}
	return true
}

func writeThreadAppFailure(w http.ResponseWriter, err error, fallbackCode string) {
	if writeThreadAppError(w, err) {
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error(), fallbackCode)
}
