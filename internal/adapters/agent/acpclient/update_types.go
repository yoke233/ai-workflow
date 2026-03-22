package acpclient

// ACP session update type constants — these match the wire values produced by
// the notification decoder and are used to route incoming SessionUpdate events.
const (
	UpdateTypeAgentThoughtChunk = "agent_thought_chunk"
	UpdateTypeAgentMessageChunk = "agent_message_chunk"
	UpdateTypeUserMessageChunk  = "user_message_chunk"
	UpdateTypeAgentThought      = "agent_thought"
	UpdateTypeAgentMessage      = "agent_message"
	UpdateTypeUserMessage       = "user_message"
	UpdateTypeToolCall          = "tool_call"
	UpdateTypeToolCallUpdate    = "tool_call_update"
	UpdateTypeToolCallCompleted = "tool_call_completed"
	UpdateTypeUsageUpdate       = "usage_update"
	UpdateTypePlan              = "plan"

	UpdateTypeAvailableCommandsUpdate = "available_commands_update"
	UpdateTypeCurrentModeUpdate       = "current_mode_update"
	UpdateTypeConfigOptionUpdate      = "config_option_update"
	UpdateTypeSessionInfoUpdate       = "session_info_update"
)

// Chunk-variant aliases used for legacy/compat matching in isACPChunkUpdateType.
const (
	UpdateTypeAssistantMessageChunk = "assistant_message_chunk"
	UpdateTypeMessageChunk          = "message_chunk"
)

// Tool-call status values.
const (
	ToolCallStatusCompleted = "completed"
)
