package core

import "time"

type EventType string

const (
	EventStageStart      EventType = "stage_start"
	EventStageComplete   EventType = "stage_complete"
	EventStageFailed     EventType = "stage_failed"
	EventHumanRequired   EventType = "human_required"
	EventPipelineDone    EventType = "pipeline_done"
	EventPipelineFailed  EventType = "pipeline_failed"
	EventPipelinePaused  EventType = "pipeline_paused"
	EventPipelineResumed EventType = "pipeline_resumed"
	EventActionApplied   EventType = "action_applied"
	EventAgentOutput     EventType = "agent_output"
	EventPipelineStuck   EventType = "pipeline_stuck"
)

type Event struct {
	Type       EventType         `json:"type"`
	PipelineID string            `json:"pipeline_id"`
	ProjectID  string            `json:"project_id"`
	Stage      StageID           `json:"stage,omitempty"`
	Agent      string            `json:"agent,omitempty"`
	Data       map[string]string `json:"data,omitempty"`
	Error      string            `json:"error,omitempty"`
	Timestamp  time.Time         `json:"timestamp"`
}
