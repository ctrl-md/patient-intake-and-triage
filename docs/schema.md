# Database schema

Five tables, plus `schema_migrations` (created by the migration runner).

```mermaid
erDiagram
    staff ||--o{ intakes : creates
    patients ||--o{ intakes : has
    intakes ||--o{ messages : contains
    intakes ||--o| triage_results : gets
    staff ||--o{ triage_results : reviews

    staff {
        uuid id PK
        text email "unique, not case sensitive"
        text password_hash
        text full_name
        text role "nurse, doctor, admin"
    }
    patients {
        uuid id PK
        text full_name
        text phone
        date date_of_birth
        text sex
    }
    intakes {
        uuid id PK
        uuid patient_id FK
        uuid created_by FK
        text token_hash "hash of the link token"
        text status "pending, in_progress, completed, expired"
        timestamptz expires_at
    }
    messages {
        uuid id PK
        uuid intake_id FK
        text role "patient, assistant, system"
        text content
    }
    triage_results {
        uuid id PK
        uuid intake_id FK "one result per intake"
        text rules_urgency
        jsonb red_flags
        text ai_urgency
        text ai_summary
        text final_urgency
        text staff_urgency
        text override_reason
    }
```

## Notes

- **Urgency** is one of `emergency`, `urgent`, `routine`.
- `final_urgency` is the higher of `rules_urgency` and `ai_urgency`. If the AI fails, `ai_urgency` stays empty and the case moves up, not down.
- A staff override needs a reason. The database enforces this with a check.
- Link tokens are stored as a hash. The real token is only shown once, when staff create the link.
- Deleting an intake also deletes its messages and triage result.
- Migrations are plain SQL files in `backend/internal/db/migrations/`. They run in name order when the server starts.
