-- Day 2: core tables for Triagely.
-- Urgency values: emergency, urgent, routine.

CREATE TABLE staff (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    full_name     TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'nurse'
                  CHECK (role IN ('nurse', 'doctor', 'admin')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Emails are not case sensitive: Ada@x.com and ada@x.com are the same person.
CREATE UNIQUE INDEX staff_email_lower_idx ON staff (lower(email));

CREATE TABLE patients (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    full_name     TEXT NOT NULL,
    phone         TEXT,
    date_of_birth DATE,
    sex           TEXT CHECK (sex IN ('female', 'male', 'other')),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- One intake = one patient conversation (one visit).
CREATE TABLE intakes (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id   UUID REFERENCES patients (id) ON DELETE SET NULL,
    created_by   UUID REFERENCES staff (id) ON DELETE SET NULL,
    -- We store a hash of the link token, never the token itself.
    token_hash   TEXT NOT NULL UNIQUE,
    status       TEXT NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'in_progress', 'completed', 'expired')),
    expires_at   TIMESTAMPTZ NOT NULL,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX intakes_status_idx ON intakes (status);
CREATE INDEX intakes_patient_idx ON intakes (patient_id);

CREATE TABLE messages (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intake_id  UUID NOT NULL REFERENCES intakes (id) ON DELETE CASCADE,
    role       TEXT NOT NULL CHECK (role IN ('patient', 'assistant', 'system')),
    content    TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX messages_intake_created_idx ON messages (intake_id, created_at);

-- The triage result for an intake: what the rules said, what the AI said,
-- the final urgency, and any staff override. One result per intake.
CREATE TABLE triage_results (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    intake_id        UUID NOT NULL UNIQUE REFERENCES intakes (id) ON DELETE CASCADE,

    rules_urgency    TEXT NOT NULL
                     CHECK (rules_urgency IN ('emergency', 'urgent', 'routine')),
    red_flags        JSONB NOT NULL DEFAULT '[]'::jsonb,

    -- AI fields can be empty when the AI fails. Then the case goes UP in urgency.
    ai_urgency       TEXT CHECK (ai_urgency IN ('emergency', 'urgent', 'routine')),
    ai_summary       TEXT,
    ai_reasoning     TEXT,

    -- The higher of rules_urgency and ai_urgency.
    final_urgency    TEXT NOT NULL
                     CHECK (final_urgency IN ('emergency', 'urgent', 'routine')),

    -- Staff override. A reason is required when staff change the urgency.
    staff_urgency    TEXT CHECK (staff_urgency IN ('emergency', 'urgent', 'routine')),
    override_reason  TEXT,
    reviewed_by      UUID REFERENCES staff (id) ON DELETE SET NULL,
    reviewed_at      TIMESTAMPTZ,

    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT override_needs_reason CHECK (
        staff_urgency IS NULL OR
        (override_reason IS NOT NULL AND length(trim(override_reason)) > 0)
    )
);

-- Used by the staff queue.
CREATE INDEX triage_results_queue_idx ON triage_results (final_urgency, created_at);
