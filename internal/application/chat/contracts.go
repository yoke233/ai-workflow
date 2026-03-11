package chat

// Request is the input for a direct chat message.
type Request struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
	WorkDir   string `json:"work_dir,omitempty"`
}

// Response is the output from a direct chat message.
type Response struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
}
