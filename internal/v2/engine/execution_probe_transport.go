package engine

import "time"

type natsExecutionProbeRequest struct {
	ExecutionID  int64  `json:"execution_id"`
	SessionID    string `json:"session_id"`
	InvocationID string `json:"invocation_id,omitempty"`
	Question     string `json:"question"`
	TimeoutMS    int64  `json:"timeout_ms"`
}

type natsExecutionProbeResponse struct {
	Reachable  bool      `json:"reachable"`
	Answered   bool      `json:"answered"`
	ReplyText  string    `json:"reply_text,omitempty"`
	Error      string    `json:"error,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}
