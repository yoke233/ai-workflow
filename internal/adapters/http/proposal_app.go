package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/yoke233/zhanggui/internal/application/proposalapp"
	"github.com/yoke233/zhanggui/internal/core"
)

func (h *Handler) proposalService() *proposalapp.Service {
	if h == nil {
		return nil
	}
	var tx proposalapp.Tx
	if txStore, ok := h.store.(core.TransactionalStore); ok {
		tx = proposalAppTx{store: txStore}
	}
	return proposalapp.New(proposalapp.Config{
		Store: h.store,
		Tx:    tx,
		Bus:   h.bus,
	})
}

type proposalAppTx struct {
	store core.TransactionalStore
}

func (t proposalAppTx) InTx(ctx context.Context, fn func(ctx context.Context, store proposalapp.Store) error) error {
	if t.store == nil {
		return fmt.Errorf("proposal transaction adapter is not configured")
	}
	return t.store.InTx(ctx, func(store core.Store) error {
		txStore, ok := store.(proposalapp.Store)
		if !ok {
			return fmt.Errorf("transaction store %T does not implement proposalapp store", store)
		}
		return fn(ctx, txStore)
	})
}

func writeProposalAppFailure(w http.ResponseWriter, err error, fallbackCode string) {
	switch {
	case errors.Is(err, core.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")
	case errors.Is(err, core.ErrInvalidTransition):
		writeError(w, http.StatusConflict, err.Error(), "INVALID_STATE")
	default:
		if fallbackCode == "" {
			fallbackCode = "PROPOSAL_FAILED"
		}
		writeError(w, http.StatusBadRequest, err.Error(), fallbackCode)
	}
}
