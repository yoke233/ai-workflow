package secretary

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/core"
)

type acpEventPublisher interface {
	Publish(evt core.Event)
}

type ChatRunEventRecorder interface {
	AppendChatRunEvent(event core.ChatRunEvent) error
}

type ACPHandlerSessionContext struct {
	SessionID    string
	ChangedFiles []string
}

type ACPHandler struct {
	acpclient.NopHandler

	cwd           string
	sessionID     string
	chatSessionID string
	projectID     string
	publisher     acpEventPublisher
	recorder      ChatRunEventRecorder

	mu          sync.Mutex
	changedSet  map[string]struct{}
	changedList []string
}

var _ acpclient.Handler = (*ACPHandler)(nil)
var _ acpclient.EventHandler = (*ACPHandler)(nil)

func NewACPHandler(cwd string, sessionID string, publisher acpEventPublisher) *ACPHandler {
	return &ACPHandler{
		cwd:        strings.TrimSpace(cwd),
		sessionID:  strings.TrimSpace(sessionID),
		publisher:  publisher,
		changedSet: make(map[string]struct{}),
	}
}

func (h *ACPHandler) SetRunEventRecorder(recorder ChatRunEventRecorder) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.recorder = recorder
}

func (h *ACPHandler) SetSessionID(sessionID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionID = strings.TrimSpace(sessionID)
}

func (h *ACPHandler) SetProjectID(projectID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.projectID = strings.TrimSpace(projectID)
}

func (h *ACPHandler) SetChatSessionID(chatSessionID string) {
	if h == nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.chatSessionID = strings.TrimSpace(chatSessionID)
}

func (h *ACPHandler) HandleWriteFile(_ context.Context, req acpclient.WriteFileRequest) (acpclient.WriteFileResult, error) {
	if h == nil {
		return acpclient.WriteFileResult{}, errors.New("acp handler is nil")
	}

	targetPath, relPath, err := h.normalizePathInScope(req.Path)
	if err != nil {
		return acpclient.WriteFileResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return acpclient.WriteFileResult{}, fmt.Errorf("ensure parent dir: %w", err)
	}

	content := []byte(req.Content)
	if err := os.WriteFile(targetPath, content, 0o644); err != nil {
		return acpclient.WriteFileResult{}, fmt.Errorf("write file %q: %w", relPath, err)
	}

	filePaths := h.recordChangedFile(relPath)
	h.publishFilesChanged(filePaths)

	return acpclient.WriteFileResult{BytesWritten: len(content)}, nil
}

func (h *ACPHandler) SessionContext() ACPHandlerSessionContext {
	if h == nil {
		return ACPHandlerSessionContext{}
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	changed := make([]string, len(h.changedList))
	copy(changed, h.changedList)
	sessionID := h.sessionID
	if trimmedChatSessionID := strings.TrimSpace(h.chatSessionID); trimmedChatSessionID != "" {
		sessionID = trimmedChatSessionID
	}
	return ACPHandlerSessionContext{
		SessionID:    sessionID,
		ChangedFiles: changed,
	}
}

func (h *ACPHandler) HandleSessionUpdate(_ context.Context, update acpclient.SessionUpdate) error {
	if h == nil {
		return nil
	}

	h.mu.Lock()
	projectID := strings.TrimSpace(h.projectID)
	chatSessionID := strings.TrimSpace(h.chatSessionID)
	agentSessionID := strings.TrimSpace(h.sessionID)
	recorder := h.recorder
	h.mu.Unlock()
	if chatSessionID == "" {
		chatSessionID = agentSessionID
	}
	if agentSessionID == "" {
		agentSessionID = strings.TrimSpace(update.SessionID)
	}

	data := map[string]string{
		"session_id":       chatSessionID,
		"agent_session_id": agentSessionID,
	}
	if rawUpdate := strings.TrimSpace(update.RawUpdateJSON); rawUpdate != "" {
		data["acp_update_json"] = rawUpdate
	}

	updateType := strings.TrimSpace(update.Type)
	if recorder != nil && !isACPChunkUpdateType(updateType) && chatSessionID != "" && projectID != "" {
		payload := map[string]any{
			"session_id":       chatSessionID,
			"agent_session_id": agentSessionID,
		}
		if text := strings.TrimSpace(update.Text); text != "" {
			payload["text"] = text
		}
		if status := strings.TrimSpace(update.Status); status != "" {
			payload["status"] = status
		}
		if rawUpdate := strings.TrimSpace(update.RawUpdateJSON); rawUpdate != "" {
			var acpPayload any
			if err := json.Unmarshal([]byte(rawUpdate), &acpPayload); err == nil {
				payload["acp"] = acpPayload
			} else {
				payload["acp_raw"] = rawUpdate
			}
		}
		if err := recorder.AppendChatRunEvent(core.ChatRunEvent{
			SessionID:  chatSessionID,
			ProjectID:  projectID,
			EventType:  string(core.EventChatRunUpdate),
			UpdateType: updateType,
			Payload:    payload,
			CreatedAt:  time.Now().UTC(),
		}); err != nil {
			log.Printf("[acp] persist chat run event failed project_id=%s session_id=%s update_type=%s err=%v", projectID, chatSessionID, updateType, err)
		}
	}

	if h.publisher != nil {
		h.publisher.Publish(core.Event{
			Type:      core.EventChatRunUpdate,
			ProjectID: projectID,
			Data:      data,
			Timestamp: time.Now(),
		})
	}
	return nil
}

func isACPChunkUpdateType(updateType string) bool {
	switch strings.TrimSpace(updateType) {
	case "agent_message_chunk", "assistant_message_chunk", "message_chunk":
		return true
	default:
		return false
	}
}

func (h *ACPHandler) normalizePathInScope(rawPath string) (string, string, error) {
	cwd := strings.TrimSpace(h.cwd)
	if cwd == "" {
		return "", "", errors.New("handler cwd is required")
	}
	cwdAbs, err := filepath.Abs(cwd)
	if err != nil {
		return "", "", fmt.Errorf("resolve cwd: %w", err)
	}

	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		return "", "", errors.New("write file path is required")
	}

	target := trimmed
	if !filepath.IsAbs(target) {
		target = filepath.Join(cwdAbs, target)
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}

	rel, err := filepath.Rel(cwdAbs, targetAbs)
	if err != nil {
		return "", "", fmt.Errorf("check path scope: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", "", fmt.Errorf("path %q is outside cwd scope", trimmed)
	}

	rel = filepath.ToSlash(filepath.Clean(rel))
	return targetAbs, rel, nil
}

func (h *ACPHandler) recordChangedFile(path string) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.changedSet[path]; !ok {
		h.changedSet[path] = struct{}{}
		h.changedList = append(h.changedList, path)
	}

	out := make([]string, len(h.changedList))
	copy(out, h.changedList)
	return out
}

func (h *ACPHandler) publishFilesChanged(filePaths []string) {
	if h.publisher == nil {
		return
	}

	h.mu.Lock()
	projectID := strings.TrimSpace(h.projectID)
	sessionID := strings.TrimSpace(h.chatSessionID)
	if sessionID == "" {
		sessionID = strings.TrimSpace(h.sessionID)
	}
	h.mu.Unlock()

	h.publisher.Publish(core.Event{
		Type:      core.EventSecretaryFilesChanged,
		ProjectID: projectID,
		Data: map[string]string{
			"session_id": sessionID,
			"file_paths": strings.Join(filePaths, ","),
		},
		Timestamp: time.Now(),
	})
}
