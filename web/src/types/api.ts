import type {
  ChatSession,
  GitHubConnectionStatus,
  Pipeline,
  PipelineStatus,
  Project,
  TaskItem,
  TaskItemStatus,
  TaskPlan,
  TaskPlanStatus,
} from "./workflow";

export interface CreateProjectRequest {
  name: string;
  repo_path: string;
  github?: {
    owner?: string;
    repo?: string;
  };
}

export type ProjectSourceType = "local_path" | "local_new" | "github_clone";

export interface CreateProjectCreateRequest {
  name: string;
  source_type: ProjectSourceType;
  repo_path?: string;
  remote_url?: string;
  ref?: string;
}

export interface CreateProjectCreateRequestResponse {
  request_id: string;
}

export interface GetProjectCreateRequestResponse {
  request_id: string;
  status: "pending" | "running" | "succeeded" | "failed" | string;
  source_type?: ProjectSourceType;
  project_id?: string;
  progress?: number;
  message?: string;
  error?: string;
}

export interface CreatePipelineRequest {
  name: string;
  description?: string;
  template: string;
  config?: Record<string, unknown>;
}

export interface CreateChatRequest {
  message: string;
  session_id?: string;
}

export interface CreateChatResponse {
  session_id: string;
  status: "accepted" | "running" | "queued" | string;
}

export interface CancelChatResponse {
  session_id: string;
  status: "cancelling" | "cancelled" | string;
}

export interface ChatRunEvent {
  id: number;
  session_id: string;
  project_id: string;
  event_type: string;
  update_type: string;
  payload: Record<string, unknown>;
  created_at: string;
}

export interface CreatePlanRequest {
  session_id: string;
  name?: string;
  fail_policy?: "block" | "skip" | "human";
}

export interface CreatePlanFromFilesRequest {
  session_id: string;
  name?: string;
  fail_policy?: "block" | "skip" | "human";
  file_paths: string[];
}

export interface FileEntry {
  path: string;
  name: string;
  type: "file" | "dir";
  git_status: string;
}

export interface RepoTreeResponse {
  dir: string;
  items: FileEntry[];
}

export interface RepoStatusResponse {
  items: FileEntry[];
}

export interface RepoDiffResponse {
  file_path: string;
  diff: string;
}

export interface SubmitPlanReviewResponse {
  status: TaskPlanStatus | string;
}

export type PlanRejectFeedbackCategory =
  | "cycle"
  | "missing_node"
  | "bad_granularity"
  | "coverage_gap"
  | "other";

export interface PlanRejectFeedback {
  category: PlanRejectFeedbackCategory;
  detail: string;
  expected_direction?: string;
}

export interface PlanActionRequest {
  action: "approve" | "reject" | "abort" | "abandon";
  feedback?: PlanRejectFeedback;
}

export interface PlanActionResponse {
  status: TaskPlanStatus | string;
}

export interface TaskActionRequest {
  action: "retry" | "skip" | "abort";
}

export interface TaskActionResponse {
  status: TaskItemStatus | string;
}

export interface PipelineActionRequest {
  action:
    | "approve"
    | "reject"
    | "modify"
    | "skip"
    | "rerun"
    | "change_role"
    | "abort"
    | "pause"
    | "resume";
  stage?: string;
  message?: string;
  role?: string;
}

export interface PipelineActionResponse {
  status: PipelineStatus | string;
  current_stage?: string;
}

export interface PipelineCheckpoint {
  pipeline_id: string;
  stage_name: string;
  status: "in_progress" | "success" | "failed" | "skipped" | "invalidated" | string;
  artifacts: Record<string, string>;
  started_at: string;
  finished_at: string;
  agent_used: string;
  tokens_used: number;
  retry_count: number;
  error?: string;
}

export type GetPipelineCheckpointsResponse = PipelineCheckpoint[];

export type ListProjectsResponse = Project[] | null;

export interface PaginatedResponse<T> {
  items: T[];
  total: number;
  offset: number;
}

export interface ApiPipeline extends Pipeline {
  task_item_id: string;
  github?: {
    connection_status?: GitHubConnectionStatus;
    issue_number?: number;
    issue_url?: string;
    pr_number?: number;
    pr_url?: string;
  };
}

export interface ApiTaskItem extends TaskItem {
  inputs: string[];
  outputs: string[];
  acceptance: string[];
  constraints: string[];
  github?: {
    issue_number?: number;
    issue_url?: string;
  };
}

export interface ApiTaskPlan extends TaskPlan {
  spec_profile: string;
  contract_version: string;
  contract_checksum: string;
  tasks: ApiTaskItem[];
}

export type ListPipelinesResponse = PaginatedResponse<ApiPipeline>;
export type ListPlansResponse = PaginatedResponse<ApiTaskPlan>;

export interface PlanDagNode {
  id: string;
  title: string;
  status: TaskItemStatus;
  pipeline_id: string;
}

export interface PlanDagEdge {
  from: string;
  to: string;
}

export interface PlanDagStats {
  total: number;
  pending: number;
  ready: number;
  running: number;
  done: number;
  failed: number;
}

export interface PlanDagResponse {
  nodes: PlanDagNode[];
  edges: PlanDagEdge[];
  stats: PlanDagStats;
}

export interface PlanReviewIssue {
  severity: string;
  issue_id: string;
  description: string;
  suggestion: string;
}

export interface PlanProposedFix {
  issue_id?: string;
  description: string;
  suggestion?: string;
}

export interface PlanReviewRecord {
  id: number;
  issue_id: string;
  round: number;
  reviewer: string;
  verdict: string;
  issues: PlanReviewIssue[];
  fixes: PlanProposedFix[];
  score?: number;
  created_at: string;
}

export interface PlanChangeRecord {
  id: string;
  issue_id: string;
  field: string;
  old_value: string;
  new_value: string;
  reason: string;
  changed_by: string;
  created_at: string;
}

export interface PipelineLogEntry {
  id: number;
  pipeline_id: string;
  stage: string;
  type: string;
  agent: string;
  content: string;
  timestamp: string;
}

export interface GetPipelineLogsQuery {
  stage?: string;
  limit?: number;
  offset?: number;
}

export type GetPipelineLogsResponse = PaginatedResponse<PipelineLogEntry>;

export interface IssueTimelineRefs {
  issue_id: string;
  pipeline_id?: string;
  stage?: string;
}

export interface IssueTimelineEntry {
  event_id: string;
  kind: "review" | "change" | "action" | "checkpoint" | "log" | "audit" | string;
  created_at: string;
  actor_type: "human" | "agent" | "system" | string;
  actor_name: string;
  actor_avatar_seed: string;
  title: string;
  body: string;
  status: "success" | "failed" | "running" | "info" | "warning" | string;
  refs: IssueTimelineRefs;
  meta: Record<string, unknown>;
}

export interface ListIssueTimelineQuery {
  limit?: number;
  offset?: number;
}

export type ListIssueTimelineResponse = PaginatedResponse<IssueTimelineEntry>;

export interface AdminAuditLogItem {
  id: number;
  project_id?: string;
  issue_id?: string;
  pipeline_id: string;
  stage?: string;
  action: string;
  message: string;
  source: string;
  user_id: string;
  created_at: string;
}

export interface ApiStatsResponse {
  total_pipelines: number;
  active_pipelines: number;
  success_rate: number;
  avg_duration: string;
  tokens_used: {
    claude: number;
    codex: number;
  };
}

export type ListChatsResponse = ChatSession[];
export type ListChatRunEventsResponse = ChatRunEvent[];
export type GetChatResponse = ChatSession;
export type CreatePlanResponse = ApiTaskPlan;
export type ListAdminAuditLogResponse = PaginatedResponse<AdminAuditLogItem>;
