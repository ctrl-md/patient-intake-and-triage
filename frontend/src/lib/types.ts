// Shapes of the JSON the Go API sends. Keep in sync with the backend.

export type StaffRole = "nurse" | "doctor" | "admin";

export interface Staff {
  id: string;
  email: string;
  full_name: string;
  role: StaffRole;
}

export interface AuthResponse {
  token: string;
  expires_at: string;
  staff: Staff;
}

export type Urgency = "routine" | "urgent" | "emergency";

export type IntakeStatus = "pending" | "in_progress" | "completed" | "expired";

export interface Intake {
  id: string;
  patient_id: string | null;
  patient_name: string | null;
  status: IntakeStatus;
  expires_at: string;
  started_at: string | null;
  completed_at: string | null;
  created_at: string;
  message_count: number;
  urgency: Urgency | null; // set after triage
}

export interface NewPatient {
  full_name: string;
  phone?: string;
  date_of_birth?: string; // 2006-01-02
  sex?: "female" | "male" | "other";
}

export interface CreateIntakeResponse {
  intake: Intake;
  token: string;
  link: string;
}

// ---- chat ----

export type MessageRole = "patient" | "assistant" | "system";

export interface ChatMessage {
  id?: string;
  role: MessageRole;
  content: string;
  created_at: string;
}

/** What the patient link page loads. */
export interface PublicIntake {
  status: IntakeStatus;
  expires_at: string;
  messages: ChatMessage[];
}

export interface SendMessageResponse {
  message: ChatMessage;
  reply?: ChatMessage; // missing when the server has no assistant
  emergency?: boolean;
}

// ---- triage ----

export interface RedFlag {
  code: string;
  label: string;
  urgency: Urgency;
  evidence: string;
}

export interface Triage {
  intake_id: string;
  rules_urgency: Urgency;
  red_flags: RedFlag[];
  ai_urgency: Urgency | null; // null when the AI failed
  ai_summary: string | null;
  ai_reasoning: string | null;
  final_urgency: Urgency;
  staff_urgency: Urgency | null;
  override_reason: string | null;
  reviewed_at: string | null;
  created_at: string;
}

export interface IntakeDetail {
  intake: Intake;
  messages: ChatMessage[];
}
