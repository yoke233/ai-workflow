package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/yoke233/ai-workflow/internal/core"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // allow all origins for dev
}

func (h *Handler) listEvents(w http.ResponseWriter, r *http.Request) {
	filter := buildEventFilter(r)

	events, err := h.store.ListEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if events == nil {
		events = []*core.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}

func (h *Handler) listFlowEvents(w http.ResponseWriter, r *http.Request) {
	flowID, ok := urlParamInt64(r, "flowID")
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid flow ID", "BAD_ID")
		return
	}

	filter := buildEventFilter(r)
	filter.FlowID = &flowID

	events, err := h.store.ListEvents(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error(), "STORE_ERROR")
		return
	}
	if events == nil {
		events = []*core.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}

func buildEventFilter(r *http.Request) core.EventFilter {
	filter := core.EventFilter{
		Limit:  queryInt(r, "limit", 100),
		Offset: queryInt(r, "offset", 0),
	}

	if s := r.URL.Query().Get("flow_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			filter.FlowID = &id
		}
	}
	if s := r.URL.Query().Get("step_id"); s != "" {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			filter.StepID = &id
		}
	}
	if s := r.URL.Query().Get("types"); s != "" {
		for _, t := range strings.Split(s, ",") {
			if t = strings.TrimSpace(t); t != "" {
				filter.Types = append(filter.Types, core.EventType(t))
			}
		}
	}
	return filter
}

// wsEvents upgrades to WebSocket and streams real-time events from the EventBus.
// Query params:
//   - flow_id: optional, filter events to a specific flow
//   - types: optional, comma-separated event types to subscribe to
func (h *Handler) wsEvents(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade writes its own error
	}
	defer conn.Close()

	// Parse subscribe options from query params.
	var types []core.EventType
	if s := r.URL.Query().Get("types"); s != "" {
		for _, t := range strings.Split(s, ",") {
			if t = strings.TrimSpace(t); t != "" {
				types = append(types, core.EventType(t))
			}
		}
	}

	var flowFilter int64
	if s := r.URL.Query().Get("flow_id"); s != "" {
		flowFilter, _ = strconv.ParseInt(s, 10, 64)
	}

	sub := h.bus.Subscribe(core.SubscribeOpts{
		Types:      types,
		BufferSize: 64,
	})
	defer sub.Cancel()

	// Read pump: detect client disconnect.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	for {
		select {
		case <-done:
			return
		case ev, ok := <-sub.C:
			if !ok {
				return
			}
			// Apply flow filter if specified.
			if flowFilter != 0 && ev.FlowID != flowFilter {
				continue
			}

			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := conn.WriteJSON(ev); err != nil {
				return
			}
		}
	}
}

// wsMessage is the WebSocket message envelope (for potential future use).
type wsMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

