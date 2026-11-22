# Intakes

An intake is one patient conversation. Staff create it and get a one-time link. The patient opens
the link (no login) and chats. The link token is the key, so it is long and random (43 characters).

| Method | Route | Login | What it does |
|---|---|---|---|
| POST | `/intakes` | staff | Create an intake. Returns the link. Body is optional (can include `patient`). |
| GET | `/intakes` | staff | List intakes, newest first. `?status=pending\|in_progress\|completed`, `?limit=1-200` |
| GET | `/intakes/{id}` | staff | One intake with all its messages |
| GET | `/public/intakes/{token}` | none | Patient sees their conversation so far |
| POST | `/public/intakes/{token}/messages` | none | Patient sends a message `{"content": "..."}` |
| POST | `/public/intakes/{token}/complete` | none | Patient says "I'm done" |

## Statuses

`pending` (link made, nothing sent) then `in_progress` (first message sent) then `completed`.

`expired` is not saved. It is worked out when you read an intake: if it is not completed and its
`expires_at` has passed, it shows as `expired`. An expired link returns `410 Gone`.

## Rules

- The token is shown once, in the create response. Only its SHA-256 hash is saved.
- A message is 1 to 2000 characters. One intake holds at most 100 messages (patient and AI together).
- A completed intake does not accept new messages (`409`). Completing needs at least one message.
- Patients never see `system` messages. Staff do.
- If the body has a `patient`, the patient row and the intake are saved together in one statement.

## Settings

| Variable | Meaning |
|---|---|
| `FRONTEND_URL` | Where the patient chat page runs. Link = `FRONTEND_URL/intake/<token>`. Default `http://localhost:3000` |
| `INTAKE_TTL` | How long a new link works. Default `24h` |

## Try it (PowerShell)

```powershell
# log in as staff
$login = @{ email = "ada@clinic.com"; password = "supersecret1" } | ConvertTo-Json
$token = (Invoke-RestMethod -Method Post -Uri http://localhost:8080/auth/login -ContentType "application/json" -Body $login).token
$h = @{ Authorization = "Bearer $token" }

# staff: create an intake for a patient
$body = @{ patient = @{ full_name = "Emeka Eze"; phone = "08012345678"; date_of_birth = "1990-05-17"; sex = "male" } } | ConvertTo-Json
$r = Invoke-RestMethod -Method Post -Uri http://localhost:8080/intakes -Headers $h -ContentType "application/json" -Body $body
$r.link
$t = $r.token

# patient: no login needed
$msg = @{ content = "I have had chest pain since morning" } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/public/intakes/$t/messages" -ContentType "application/json" -Body $msg
Invoke-RestMethod -Uri "http://localhost:8080/public/intakes/$t"
Invoke-RestMethod -Method Post -Uri "http://localhost:8080/public/intakes/$t/complete"

# staff: see it
Invoke-RestMethod -Uri http://localhost:8080/intakes -Headers $h
Invoke-RestMethod -Uri "http://localhost:8080/intakes/$($r.intake.id)" -Headers $h
```

## Not done yet

- AI replies to the patient (Day 8)
- Rate limiting by IP on the public routes
- CORS, so the browser app can call the API (Day 5)
