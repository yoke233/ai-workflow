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
