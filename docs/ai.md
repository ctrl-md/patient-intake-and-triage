# AI, rules and triage (Days 6 to 10)

Demo project. Not a medical device. Fake data only.

## What happens

1. Staff makes an intake link. Patient opens `/intake/<token>` and chats.
2. Every patient message is saved. Then the **rules engine** reads the patient's words (plain Go, no AI).
   - If a rule says **emergency**, the AI is skipped. The patient gets a fixed "call emergency services" message and a red banner.
   - If not, the **AI assistant** asks one follow-up question. If the AI is off or fails, a fixed list of questions is used.
3. Patient presses **Finish**. Triage runs in the background:
   - rules urgency + AI summary and urgency (strict JSON, checked in Go)
   - **final = the higher of the two.** AI can raise urgency, never lower it.
   - if the AI fails or sends bad JSON twice, the case is marked at least **urgent**.
4. Staff see the urgency on the dashboard and the summary, red flags and chat on the intake page.

## Safety choices

- Rules run first and always win. Negation ("no chest pain") is handled by looking at the 3 words before a match. When unsure, the flag is raised.
- Only the patient's words go to the rules. The assistant's questions never trigger a flag.
- AI output must match the schema exactly: `summary`, `urgency` (routine/urgent/emergency), `reasoning`. Unknown keys, wrong values, empty or very long text are rejected. One retry, then give up.
- The patient chat is treated as data, not instructions (prompt says so, and the output is validated anyway).
- Errors from the AI provider are logged without patient text or the API key.

## Setup

In `backend/.env`:

```
LLM_PROVIDER=gemini      # or groq
LLM_API_KEY=your-key
LLM_MODEL=               # empty = gemini-2.5-flash / llama-3.3-70b-versatile
LLM_TIMEOUT=20s
```

No key means no AI. The chat still works with fixed questions and the rules still run.
Model names change often. If you see "model not found", set `LLM_MODEL` to a current one.

## API

| Route | Who | What |
|---|---|---|
| `POST /public/intakes/{token}/messages` | patient | Saves the message. Returns `message`, `reply`, `emergency` |
| `POST /public/intakes/{token}/complete` | patient | Finishes. Starts triage in the background |
| `GET /intakes/{id}/triage` | staff | Saved result, 404 if none yet |
| `POST /intakes/{id}/triage/run` | staff | Run again (completed intakes only). Keeps staff override fields |
| `GET /intakes` | staff | Each intake now has `urgency` (or null) |

## Code map

- `internal/llm` : Gemini and Groq over plain HTTP. Timeout per try, retries on 429/5xx/network, `Fake` for tests.
- `internal/triage` : `rules.go`, `ai.go` (prompt + validation), `service.go`, `pgstore.go`, `handler.go`.
- `internal/assistant` : the chat helper and the fixed fallback questions.
- `internal/intake` : `Hooks` plug the above into the chat without the intake package knowing about AI.
