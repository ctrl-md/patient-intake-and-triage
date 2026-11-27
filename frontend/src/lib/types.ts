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
