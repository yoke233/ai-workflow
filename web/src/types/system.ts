export interface SandboxProviderSupport {
  supported: boolean;
  implemented: boolean;
  reason?: string;
}

export interface SandboxSupportResponse {
  os: string;
  arch: string;
  enabled: boolean;
  configured_provider: string;
  current_provider: string;
  current_supported: boolean;
  providers: Record<string, SandboxProviderSupport>;
}

export interface UpdateSandboxSupportRequest {
  enabled?: boolean;
  provider?: string;
}

export interface LLMConfigItem {
  id: string;
  type: "openai_chat_completion" | "openai_response" | "anthropic";
  base_url: string;
  api_key: string;
  model: string;
}

export interface LLMConfigResponse {
  default_config_id: string;
  configs: LLMConfigItem[];
}

export interface UpdateLLMConfigRequest {
  default_config_id?: string;
  configs?: LLMConfigItem[];
}
