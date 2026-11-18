# Patient Intake And Triage

AI patient intake and triage for clinics. A patient chats about their symptoms before the visit. The AI asks follow-up questions, writes a short summary and suggests how urgent the case is. Clinic staff get a live queue, sorted so the sickest patients are seen first.

I built it in 15 days, with one push every day.

> **Demo project, not a medical device.** Triagely helps staff sort patients. It does not diagnose anyone. A trained clinician must review every case. Use fake data only.

## The problem

In many clinics, patients wait in one line, first come first served. The person with chest pain can sit behind five people with a cold. Triagely collects the symptoms early and puts the urgent cases at the top.

## How it works

1. Staff create an intake link and send it to the patient (no patient login needed).
2. The patient chats about what is wrong.
3. The AI asks follow-up questions: how long, how bad, any other symptoms, existing conditions.
4. **Red-flag rules run first.** Go checks every message for danger signs (chest pain, trouble breathing, heavy bleeding, fainting, signs of stroke, and similar). If a rule hits, the case is marked **emergency** right away.
5. The AI writes a short clinical summary and suggests an urgency level.
6. The final urgency is the higher of the rules result and the AI result.
7. Staff see the case in a live queue, open it, and confirm or change the urgency. Every change is logged.

## Safety rules

These are the most important part of the project.

- Red-flag rules always run before the AI and always win.
- The AI can raise urgency, but it can never lower a case below what the rules set.
- If the AI fails, times out or returns something invalid, the case moves **up** in urgency, never down.
- AI output must match a strict JSON schema. Anything that does not match is rejected.
- Staff can override any result, and every override is saved in an audit log.
- All AI results are labelled as suggestions.
- No real patient data. Fake data only.

## Stack

| Part | Tech |
|---|---|
| Backend | Go, chi, pgx, Postgres, JWT auth |
| AI | Gemini or Groq (free tier), called directly from Go |
| Frontend | Next.js (App Router), TypeScript, Tailwind CSS |
| Live updates | Server-Sent Events (or WebSocket) |
| Local setup | Docker Compose |

One Go service. No Kafka, no microservices. Keep it simple.

## Features

- Patient intake chat with a one-time link
- AI follow-up questions that adapt to the patient's answers
- Rule-based red-flag check (tested)
- AI summary and urgency score (emergency, urgent, routine)
- Live triage queue for staff, sorted by urgency and waiting time
- Case detail page with the full chat, the summary and the reasons for the score
- Staff override with a required reason
- Alerts for new emergency cases
- Audit log of all actions
- Eval set of sample cases to check the triage quality

## Project structure

```
triagely/
  backend/
    cmd/server/        entry point
    internal/
      auth/            JWT and staff login
      intake/          sessions, messages
      rules/           red-flag rules engine
      ai/              LLM client, prompts, schema validation
      triage/          combines rules + AI into a final urgency
      queue/           queue and live updates
      audit/           audit log
    migrations/        SQL migrations
  frontend/
    app/               Next.js pages (patient chat, staff dashboard)
    components/
    lib/               API client
  evals/               sample cases and expected urgency
  docker-compose.yml
  README.md
```

## Run it locally

You need Go, Node.js, Docker, and an API key for Gemini or Groq.

```bash
git clone <your-repo-url>
cd triagely

# Postgres
docker compose up -d

# Backend
cd backend
cp .env.example .env        # add your LLM API key and JWT secret
go run ./cmd/server

# Frontend (new terminal)
cd frontend
npm install
npm run dev
```

Backend runs on `http://localhost:8080`, frontend on `http://localhost:3000`.

## Environment variables

```
DATABASE_URL=postgres://triagely:triagely@localhost:5432/triagely?sslmode=disable
JWT_SECRET=change-me
LLM_PROVIDER=gemini        # or groq
LLM_API_KEY=your-key
PORT=8080
```

## API overview

| Method | Route | What it does |
|---|---|---|
| POST | `/auth/register` | Create a staff account |
| POST | `/auth/login` | Log in, get a JWT |
| POST | `/intakes` | Staff creates an intake link |
| POST | `/intakes/:token/messages` | Patient sends a message, gets the next question |
| GET | `/queue` | Live queue for staff (sorted by urgency) |
| GET | `/cases/:id` | Case detail, chat and summary |
| PATCH | `/cases/:id/urgency` | Staff override (reason required) |
| GET | `/health` | Health check |

## Testing and evals

- Unit tests for the rules engine (every red flag has a test)
- Tests for the AI client using a mock LLM
- `evals/` holds 20 sample cases with the expected urgency. Run them to see how often the final result matches.

```bash
cd backend
go test ./...
go run ./cmd/eval
```

## 15-day build plan

One push per day. Tick each box when it is pushed.

- [ ] **Day 1:** Repo, Docker Compose with Postgres, Go server with `/health`, this README
- [ ] **Day 2:** Database schema and migrations (staff, patients, intakes, messages, triage results)
- [ ] **Day 3:** Staff auth in Go (register, login, JWT middleware)
- [ ] **Day 4:** Intake sessions API (create session, one-time link, store messages) with tests
- [ ] **Day 5:** Next.js setup, staff login page, protected layout, API client
- [ ] **Day 6:** Patient intake chat UI (plain chat, no AI yet)
- [ ] **Day 7:** LLM client in Go (timeouts, retries, mock for tests)
- [ ] **Day 8:** AI follow-up questions, so the chat adapts to what the patient says
- [ ] **Day 9:** Red-flag rules engine in Go that runs before the AI, with tests
- [ ] **Day 10:** AI summary and urgency score as strict JSON, validated in Go
- [ ] **Day 11:** Staff dashboard with the live triage queue
- [ ] **Day 12:** Case detail page, staff override, audit log
- [ ] **Day 13:** Alerts for emergency cases (live updates with SSE or WebSocket)
- [ ] **Day 14:** Evals (20 sample cases), more tests, bug fixes
- [ ] **Day 15:** Deploy, screenshots, demo video, final README polish

## Roadmap (after day 15)

- WhatsApp intake link
- Support for local languages and Pidgin
- Vitals input (temperature, pulse, blood pressure) from a nurse
- Role-based access (nurse, doctor, admin)

## License

MIT
