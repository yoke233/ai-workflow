package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/yoke233/ai-workflow/internal/acpclient"
	"github.com/yoke233/ai-workflow/internal/v2/core"
)

// NATSSessionManagerConfig configures the NATS-backed session manager.
type NATSSessionManagerConfig struct {
	// NATSConn is an already-connected NATS connection.
	NATSConn *nats.Conn

	// StreamPrefix is the JetStream stream name prefix (default: "aiworkflow").
	StreamPrefix string

	// ServerID uniquely identifies this server in multi-server setups.
	// Used as a prefix in prompt IDs to avoid collisions across servers.
	// Auto-generated from hostname + PID if empty.
	ServerID string

	// Store is used for persisting prompt metadata.
	Store core.Store
}

// NATSSessionManager implements SessionManager using NATS JetStream.
// Prompts are published as messages, executors consume them via queue groups,
// results and events are streamed back through dedicated subjects.
//
// Subject layout:
//
//	{prefix}.prompt.submit.{agent_type}  — prompt submission (consumed by executors)
//	{prefix}.prompt.result.{prompt_id}   — final result
//	{prefix}.prompt.events.{prompt_id}   — streaming events during execution
//	{prefix}.executor.register           — executor heartbeat/registration
type NATSSessionManager struct {
	nc       *nats.Conn
	js       jetstream.JetStream
	prefix   string
	serverID string
	store    core.Store

	mu      sync.Mutex
	handles map[string]*natsHandle
	nextID  int64

	activeCount atomic.Int32
	drainWg     sync.WaitGroup
}

type natsHandle struct {
	id        string
	sessionIn SessionAcquireInput
}

// natsPromptMessage is the payload published to the prompt submission subject.
type natsPromptMessage struct {
	PromptID string             `json:"prompt_id"`
	HandleID string             `json:"handle_id"`
	Text     string             `json:"text"`
	Input    SessionAcquireInput `json:"-"` // serialized separately
	FlowID   int64              `json:"flow_id"`
	StepID   int64              `json:"step_id"`
	ExecID   int64              `json:"exec_id"`
	AgentID  string             `json:"agent_id"`
	WorkDir  string             `json:"work_dir"`
}

// natsPromptResult is the payload published to the result subject.
type natsPromptResult struct {
	PromptID     string `json:"prompt_id"`
	Text         string `json:"text"`
	StopReason   string `json:"stop_reason"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	Error        string `json:"error,omitempty"`
}

// natsEventMessage wraps a streaming event for NATS transport.
type natsEventMessage struct {
	PromptID string                 `json:"prompt_id"`
	Seq      int64                  `json:"seq"`
	Update   acpclient.SessionUpdate `json:"update"`
}

// NewNATSSessionManager creates a NATS-backed session manager.
func NewNATSSessionManager(cfg NATSSessionManagerConfig) (*NATSSessionManager, error) {
	if cfg.NATSConn == nil {
		return nil, fmt.Errorf("NATS connection is required")
	}

	prefix := strings.TrimSpace(cfg.StreamPrefix)
	if prefix == "" {
		prefix = "aiworkflow"
	}

	js, err := jetstream.New(cfg.NATSConn)
	if err != nil {
		return nil, fmt.Errorf("create JetStream context: %w", err)
	}

	serverID := strings.TrimSpace(cfg.ServerID)
	if serverID == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "unknown"
		}
		serverID = fmt.Sprintf("%s-%d", hostname, os.Getpid())
	}

	m := &NATSSessionManager{
		nc:       cfg.NATSConn,
		js:       js,
		prefix:   prefix,
		serverID: serverID,
		store:    cfg.Store,
		handles:  make(map[string]*natsHandle),
	}

	if err := m.ensureStreams(context.Background()); err != nil {
		return nil, fmt.Errorf("ensure JetStream streams: %w", err)
	}

	return m, nil
}

func (m *NATSSessionManager) ensureStreams(ctx context.Context) error {
	// Prompt submission stream — consumed by executor workers.
	_, err := m.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      m.prefix + "_prompts",
		Subjects:  []string{m.prefix + ".prompt.submit.>"},
		Retention: jetstream.WorkQueuePolicy,
		MaxAge:    24 * time.Hour,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create prompts stream: %w", err)
	}

	// Results stream — published by executors, consumed by watchers.
	_, err = m.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      m.prefix + "_results",
		Subjects:  []string{m.prefix + ".prompt.result.>"},
		Retention: jetstream.InterestPolicy,
		MaxAge:    24 * time.Hour,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create results stream: %w", err)
	}

	// Events stream — streaming events during execution.
	_, err = m.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      m.prefix + "_events",
		Subjects:  []string{m.prefix + ".prompt.events.>"},
		Retention: jetstream.InterestPolicy,
		MaxAge:    1 * time.Hour,
		Storage:   jetstream.FileStorage,
	})
	if err != nil {
		return fmt.Errorf("create events stream: %w", err)
	}

	return nil
}

// Acquire stores session metadata locally — actual ACP session creation happens on the executor.
func (m *NATSSessionManager) Acquire(_ context.Context, in SessionAcquireInput) (*SessionHandle, error) {
	m.mu.Lock()
	m.nextID++
	handleID := fmt.Sprintf("nats-%d", m.nextID)
	nh := &natsHandle{
		id:        handleID,
		sessionIn: in,
	}
	m.handles[handleID] = nh
	m.mu.Unlock()

	return &SessionHandle{ID: handleID}, nil
}

// SubmitPrompt publishes the prompt to JetStream for remote execution.
func (m *NATSSessionManager) SubmitPrompt(ctx context.Context, handle *SessionHandle, text string) (string, error) {
	m.mu.Lock()
	nh, ok := m.handles[handle.ID]
	if !ok {
		m.mu.Unlock()
		return "", fmt.Errorf("session handle %q not found", handle.ID)
	}
	m.nextID++
	promptID := fmt.Sprintf("np-%s-%d-%d", m.serverID, time.Now().UnixNano(), m.nextID)
	m.mu.Unlock()

	agentType := "default"
	if nh.sessionIn.Driver != nil {
		agentType = nh.sessionIn.Driver.ID
	}

	msg := natsPromptMessage{
		PromptID: promptID,
		HandleID: handle.ID,
		Text:     text,
		FlowID:   nh.sessionIn.FlowID,
		StepID:   nh.sessionIn.StepID,
		ExecID:   nh.sessionIn.ExecID,
		AgentID:  agentType,
		WorkDir:  nh.sessionIn.WorkDir,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("marshal prompt message: %w", err)
	}

	subject := fmt.Sprintf("%s.prompt.submit.%s", m.prefix, agentType)
	_, err = m.js.Publish(ctx, subject, data)
	if err != nil {
		return "", fmt.Errorf("publish prompt to NATS: %w", err)
	}

	m.activeCount.Add(1)
	m.drainWg.Add(1)

	slog.Info("nats session manager: prompt submitted",
		"prompt_id", promptID, "agent", agentType, "flow_id", msg.FlowID)

	return promptID, nil
}

// WatchPrompt subscribes to the result and event subjects for a given prompt.
// It blocks until the result is received or ctx is cancelled.
func (m *NATSSessionManager) WatchPrompt(ctx context.Context, promptID string, lastEventSeq int64, sink EventSink) (*SessionPromptResult, error) {
	defer func() {
		m.activeCount.Add(-1)
		m.drainWg.Done()
	}()

	// Subscribe to events stream for this prompt.
	eventSubject := fmt.Sprintf("%s.prompt.events.%s", m.prefix, promptID)
	resultSubject := fmt.Sprintf("%s.prompt.result.%s", m.prefix, promptID)

	// Create ephemeral consumer for events.
	if sink != nil {
		eventConsumer, err := m.js.CreateOrUpdateConsumer(ctx, m.prefix+"_events", jetstream.ConsumerConfig{
			FilterSubject: eventSubject,
			DeliverPolicy: jetstream.DeliverAllPolicy,
			AckPolicy:     jetstream.AckExplicitPolicy,
		})
		if err != nil {
			slog.Warn("nats watch: failed to create event consumer", "prompt_id", promptID, "error", err)
		} else {
			go m.consumeEvents(ctx, eventConsumer, sink)
		}
	}

	// Create consumer for result.
	resultConsumer, err := m.js.CreateOrUpdateConsumer(ctx, m.prefix+"_results", jetstream.ConsumerConfig{
		FilterSubject: resultSubject,
		DeliverPolicy: jetstream.DeliverLastPolicy,
		AckPolicy:     jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return nil, fmt.Errorf("create result consumer: %w", err)
	}

	// Block until result message arrives.
	for {
		msgs, err := resultConsumer.Fetch(1, jetstream.FetchMaxWait(5*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		for msg := range msgs.Messages() {
			var result natsPromptResult
			if err := json.Unmarshal(msg.Data(), &result); err != nil {
				_ = msg.Nak()
				return nil, fmt.Errorf("unmarshal result: %w", err)
			}
			_ = msg.Ack()

			if result.Error != "" {
				return nil, fmt.Errorf("remote execution failed: %s", result.Error)
			}

			return &SessionPromptResult{
				Text:         result.Text,
				StopReason:   result.StopReason,
				InputTokens:  result.InputTokens,
				OutputTokens: result.OutputTokens,
			}, nil
		}

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
}

func (m *NATSSessionManager) consumeEvents(ctx context.Context, consumer jetstream.Consumer, sink EventSink) {
	for {
		msgs, err := consumer.Fetch(10, jetstream.FetchMaxWait(2*time.Second))
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		for msg := range msgs.Messages() {
			var ev natsEventMessage
			if err := json.Unmarshal(msg.Data(), &ev); err != nil {
				_ = msg.Nak()
				continue
			}
			_ = msg.Ack()
			_ = sink.HandleSessionUpdate(ctx, ev.Update)
		}
		if ctx.Err() != nil {
			return
		}
	}
}

// RecoverPrompts queries NATS for prompts that may have been in-flight during a restart.
func (m *NATSSessionManager) RecoverPrompts(ctx context.Context, since time.Time) ([]PromptStatus, error) {
	// In NATS mode, prompts that were published but not yet consumed are still in the stream.
	// Prompts that were being executed will have their results published by the executor.
	// We return an empty list here — the executor worker handles recovery by re-publishing results.
	slog.Info("nats session manager: recovery check", "since", since)
	return nil, nil
}

// Release is a no-op in NATS mode — sessions are managed by executors.
func (m *NATSSessionManager) Release(_ context.Context, handle *SessionHandle) error {
	m.mu.Lock()
	delete(m.handles, handle.ID)
	m.mu.Unlock()
	return nil
}

// CleanupFlow is a no-op in NATS mode — executor workers manage their own sessions.
func (m *NATSSessionManager) CleanupFlow(_ int64) {}

// DrainActive blocks until all in-flight prompts complete.
func (m *NATSSessionManager) DrainActive(ctx context.Context) error {
	done := make(chan struct{})
	go func() {
		m.drainWg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ActiveCount returns the number of prompts being watched.
func (m *NATSSessionManager) ActiveCount() int {
	return int(m.activeCount.Load())
}

// Close drains the NATS connection.
func (m *NATSSessionManager) Close() {
	if m.nc != nil {
		m.nc.Drain()
	}
}
