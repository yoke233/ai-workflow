import type { Thread, ThreadMessage } from "./collaboration";

export type WorkItemStatus =
  | "open"
  | "accepted"
  | "queued"
  | "running"
  | "blocked"
  | "failed"
  | "done"
  | "cancelled"
  | "closed"
  | string;

export type WorkItemPriority = "low" | "medium" | "high" | "urgent";

export interface WorkItem {
  id: number;
  project_id?: number | null;
  resource_space_id?: number | null;
  parent_work_item_id?: number | null;
  root_work_item_id?: number | null;
  final_deliverable_id?: number | null;
  title: string;
  body: string;
  priority: WorkItemPriority;
  executor_profile_id?: string;
  reviewer_profile_id?: string;
  active_profile_id?: string;
  sponsor_profile_id?: string;
  created_by_profile_id?: string;
  labels?: string[];
  depends_on?: number[];
  escalation_path?: string[];
  status: WorkItemStatus;
  metadata?: Record<string, unknown>;
  archived_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface CreateWorkItemRequest {
  project_id?: number;
  resource_space_id?: number;
  parent_work_item_id?: number;
  root_work_item_id?: number;
  final_deliverable_id?: number;
  title: string;
  body?: string;
  priority?: WorkItemPriority;
  executor_profile_id?: string;
  reviewer_profile_id?: string;
  active_profile_id?: string;
  sponsor_profile_id?: string;
  created_by_profile_id?: string;
  labels?: string[];
  depends_on?: number[];
  escalation_path?: string[];
  metadata?: Record<string, unknown>;
}

export interface UpdateWorkItemRequest {
  project_id?: number;
  resource_space_id?: number;
  parent_work_item_id?: number;
  root_work_item_id?: number;
  final_deliverable_id?: number;
  title?: string;
  body?: string;
  status?: WorkItemStatus;
  priority?: WorkItemPriority;
  executor_profile_id?: string;
  reviewer_profile_id?: string;
  active_profile_id?: string;
  sponsor_profile_id?: string;
  created_by_profile_id?: string;
  labels?: string[];
  depends_on?: number[];
  escalation_path?: string[];
  metadata?: Record<string, unknown>;
}

export interface Project {
  id: number;
  name: string;
  kind: "dev" | "general" | string;
  description?: string;
  metadata?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

export type ActionType = "exec" | "gate" | "composite" | "plan" | string;

export type ActionStatus =
  | "pending"
  | "ready"
  | "running"
  | "waiting_gate"
  | "blocked"
  | "failed"
  | "done"
  | "cancelled"
  | string;

export interface Action {
  id: number;
  work_item_id: number;
  name: string;
  description?: string;
  depends_on?: number[];
  type: ActionType;
  status: ActionStatus;
  position: number;
  agent_role?: string;
  required_capabilities?: string[];
  acceptance_criteria?: string[];
  input?: string;
  timeout?: number;
  max_retries: number;
  retry_count: number;
  config?: Record<string, unknown>;
  created_at: string;
  updated_at: string;
}

export type RunStatus =
  | "created"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled"
  | string;

export type RunErrorKind =
  | "transient"
  | "permanent"
  | "need_help"
  | string;

export interface Run {
  id: number;
  action_id: number;
  work_item_id: number;
  status: RunStatus;
  agent_id?: string;
  agent_context_id?: number | null;
  briefing_snapshot?: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  error_message?: string;
  error_kind?: RunErrorKind;
  attempt: number;
  started_at?: string | null;
  finished_at?: string | null;
  created_at: string;
  result_markdown?: string;
  result_metadata?: Record<string, unknown>;
}

export type EventType =
  | "work_item.queued"
  | "work_item.started"
  | "work_item.completed"
  | "work_item.failed"
  | "work_item.cancelled"
  | "action.ready"
  | "action.started"
  | "action.completed"
  | "action.failed"
  | "action.blocked"
  | "run.created"
  | "run.started"
  | "run.succeeded"
  | "run.failed"
  | "run.agent_output"
  | "gate.passed"
  | "gate.rejected"
  | "chat.output"
  | string;

export interface Event {
  id: number;
  type: EventType;
  work_item_id?: number;
  action_id?: number;
  run_id?: number;
  data?: Record<string, unknown>;
  timestamp: string;
}

export interface RunWorkItemResponse {
  work_item_id: number;
  status: "accepted" | string;
  message?: string;
}

export interface CancelWorkItemResponse {
  work_item_id: number;
  status: "cancelled" | string;
}

export interface BootstrapPRWorkItemRequest {
  base_branch?: string;
  title?: string;
  body?: string;
}

export interface BootstrapPRWorkItemResponse {
  work_item_id: number;
  implement_action_id: number;
  commit_push_action_id: number;
  open_pr_action_id: number;
  gate_action_id: number;
}

export interface CreateProjectRequest {
  name: string;
  kind?: "dev" | "general" | string;
  description?: string;
  metadata?: Record<string, string>;
}

export interface UpdateProjectRequest {
  name?: string;
  kind?: "dev" | "general" | string;
  description?: string;
  metadata?: Record<string, string>;
}

export interface RequirementMatchedProject {
  project_id: number;
  project_name: string;
  relevance?: "high" | "medium" | "low" | string;
  reason?: string;
  suggested_scope?: string;
}

export interface RequirementSuggestedAgent {
  profile_id: string;
  reason?: string;
}

export interface RequirementAnalysis {
  summary: string;
  type: "single_project" | "cross_project" | "new_project" | string;
  matched_projects?: RequirementMatchedProject[];
  suggested_agents?: RequirementSuggestedAgent[];
  complexity?: "low" | "medium" | "high" | string;
  suggested_meeting_mode?: "direct" | "concurrent" | "group_chat" | string;
  risks?: string[];
}

export interface RequirementThreadContextRef {
  project_id: number;
  access?: "read" | "check" | "write" | string;
}

export interface RequirementSuggestedThread {
  title: string;
  context_refs?: RequirementThreadContextRef[];
  agents?: string[];
  meeting_mode?: "direct" | "concurrent" | "group_chat" | string;
  meeting_max_rounds?: number;
}

export interface AnalyzeRequirementRequest {
  description: string;
  context?: string;
}

export interface AnalyzeRequirementResponse {
  analysis: RequirementAnalysis;
  suggested_thread: RequirementSuggestedThread;
}

export interface ThreadContextRef {
  id: number;
  thread_id: number;
  project_id: number;
  access: "read" | "check" | "write" | string;
  note?: string;
  granted_by?: string;
  created_at: string;
  expires_at?: string | null;
}

export interface CreateThreadFromRequirementRequest {
  description: string;
  context?: string;
  owner_id?: string;
  analysis?: RequirementAnalysis;
  thread_config: RequirementSuggestedThread;
}

export interface CreateThreadFromRequirementResponse {
  thread: Thread;
  context_refs?: ThreadContextRef[];
  agents?: string[];
  message?: ThreadMessage;
  invite_errors?: Record<string, string>;
}

export type ResourceSpaceKind = "git" | "local_fs" | "s3" | "http" | "webdav" | string;

export interface ResourceSpace {
  id: number;
  project_id: number;
  kind: ResourceSpaceKind;
  root_uri: string;
  role?: string;
  config?: Record<string, unknown>;
  label?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateResourceSpaceRequest {
  kind: ResourceSpaceKind;
  root_uri: string;
  role?: string;
  config?: Record<string, unknown>;
  label?: string;
}

export interface UpdateResourceSpaceRequest {
  kind?: ResourceSpaceKind;
  root_uri?: string;
  role?: string;
  config?: Record<string, unknown>;
  label?: string;
}

// ---------------------------------------------------------------------------
// Resources
// ---------------------------------------------------------------------------

export interface Resource {
  id: number;
  project_id: number;
  work_item_id?: number | null;
  run_id?: number | null;
  message_id?: number | null;
  storage_kind: string;
  uri: string;
  role: string;
  file_name: string;
  mime_type?: string;
  size_bytes?: number;
  checksum?: string;
  metadata?: Record<string, unknown>;
  created_at: string;
}

// ---------------------------------------------------------------------------
// Action IO declarations
// ---------------------------------------------------------------------------

export type IODirection = "input" | "output";

export interface ActionIODecl {
  id: number;
  action_id: number;
  space_id?: number | null;
  resource_id?: number | null;
  direction: IODirection;
  path: string;
  media_type?: string;
  description?: string;
  required: boolean;
  created_at: string;
}

export interface CreateActionIODeclRequest {
  space_id?: number;
  resource_id?: number;
  direction: IODirection;
  path: string;
  media_type?: string;
  description?: string;
  required?: boolean;
}

export interface CreateActionRequest {
  name: string;
  type: "exec" | "gate" | "composite" | "plan";
  position?: number;
  agent_role?: string;
  required_capabilities?: string[];
  acceptance_criteria?: string[];
  timeout?: string;
  max_retries?: number;
  config?: Record<string, unknown>;
}

export interface GenerateActionsRequest {
  description: string;
  files?: Record<string, string>;
}

export interface UpdateActionRequest {
  name?: string;
  type?: "exec" | "gate" | "composite" | "plan";
  position?: number;
  description?: string;
  agent_role?: string;
  required_capabilities?: string[];
  acceptance_criteria?: string[];
  timeout?: string;
  max_retries?: number;
  config?: Record<string, unknown>;
}
