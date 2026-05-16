// TypeScript mirrors of Go onboarding structs.
// Field names are PascalCase to match Go's JSON encoding (no json tags on Go side).
// SPEC: SPEC-MINK-ONBOARDING-001 §6.2

// Step 1: Locale selection
export interface LocaleChoice {
  Country: string; // ISO 3166-1 alpha-2, e.g., "KR"
  Language: string; // BCP 47 primary tag, e.g., "ko"
  Timezone: string; // IANA timezone ID, e.g., "Asia/Seoul"
  LegalFlags: string[]; // active legal-regime flags, e.g., ["GDPR"]
}

// Step 2: Model setup
export interface ModelSetup {
  OllamaInstalled: boolean;
  DetectedModel: string;
  SelectedModel: string;
  ModelSizeBytes: number;
  RAMBytes: number;
}

// Step 3: CLI tools detection
export interface CLITool {
  Name: string;
  Version: string;
  Path: string;
}

export interface CLIToolsDetection {
  DetectedTools: CLITool[];
}

// Step 4: Persona profile
export type HonorificLevel = "formal" | "casual" | "intimate" | "";

export interface PersonaProfile {
  Name: string;
  HonorificLevel: HonorificLevel;
  Pronouns: string;
  SoulMarkdown: string;
}

// Step 5: Provider choice
export type Provider =
  | "anthropic"
  | "openai"
  | "google"
  | "ollama"
  | "deepseek"
  | "custom"
  | "unset";

export type AuthMethod = "oauth" | "api_key" | "env";

export interface ProviderChoice {
  Provider: Provider;
  AuthMethod: AuthMethod;
  APIKeyStored: boolean;
  CustomEndpoint: string;
  PreferredModel: string;
}

// Step 5 submit body: wraps ProviderChoice with the APIKey field
export interface ProviderStepInput {
  Choice: ProviderChoice;
  APIKey: string;
}

// Step 6: Messenger channel
export type MessengerType =
  | "local_terminal"
  | "slack"
  | "telegram"
  | "discord"
  | "custom";

export interface MessengerChannel {
  Type: MessengerType;
  BotTokenKey: string;
}

// Step 7: Consent flags
export interface ConsentFlags {
  ConversationStorageLocal: boolean;
  LoRATrainingAllowed: boolean;
  TelemetryEnabled: boolean;
  CrashReportingEnabled: boolean;
  GDPRExplicitConsent: boolean | null;
}

// Full data envelope — matches Go OnboardingData
export interface OnboardingData {
  Locale: LocaleChoice;
  Model: ModelSetup;
  CLITools: CLIToolsDetection;
  Persona: PersonaProfile;
  Provider: ProviderChoice;
  Messenger: MessengerChannel;
  Consent: ConsentFlags;
}

// Session state returned by all API endpoints
export interface OnboardingState {
  session_id: string;
  csrf_token: string;
  current_step: number;
  total_steps: number;
  data: OnboardingData;
  completed_at: string | null;
}

// SSE progress event from GET /install/api/session/{id}/pull/stream
// Field names are PascalCase to match Go struct encoding (no json tags).
export interface PullProgressUpdate {
  Phase: string;
  Layer: string;
  BytesTotal: number;
  BytesDone: number;
  PercentDone: number; // -1 when unknown, 0-100 otherwise
  Raw: string;
}

// Error shape returned by backend on non-200
export interface ApiError {
  error: {
    code: string;
    message: string;
  };
}
