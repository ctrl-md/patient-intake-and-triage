# Frontend

Next.js (App Router), TypeScript and Tailwind. It talks to the Go API with a small client in
`src/lib/api.ts`.

## Run it (PowerShell)

```powershell
cd frontend
npm install
npm run dev
```

Open http://localhost:3000. The Go API must be running on port 8080. To point at another API,
copy `.env.example` to `.env.local` and change `NEXT_PUBLIC_API_URL`.

Other commands: `npm test` (API client tests), `npm run lint`, `npm run typecheck`, `npm run build`.

## Pages

| Route | Needs login | What it is |
|---|---|---|
| `/` | no | Sends you to `/dashboard` or `/login` |
| `/login` | no | Staff login |
| `/register` | no | Create a staff account (starts as nurse) |
| `/dashboard` | yes | Intake list and a "New intake" button that shows the patient link |

Everything inside `src/app/(app)/` is behind the login gate. Add new staff pages there.

## How login works

1. `/auth/login` returns a token. We keep it in `localStorage` under `triagely.token`.
2. On page load, `AuthProvider` calls `/auth/me` with that token. If it works, you are logged in.
3. `AuthGate` (in `(app)/layout.tsx`) sends anyone who is not logged in to `/login`.
4. If any logged-in request gets a `401` (token expired, account removed), the app logs out by itself.

The gate only hides pages. The real protection is the Go API, which checks the token on every
request.

`localStorage` is simple, but any script on the page can read it, so an XSS bug would leak the
token. A safer setup later is an httpOnly cookie set by the server. Fine for this demo.

## Files

```
src/
  app/
    layout.tsx            root layout, wraps everything in AuthProvider
    page.tsx              redirect page
    login/  register/     public pages
    (app)/layout.tsx      login gate + header
    (app)/dashboard/      first protected page
  components/
    auth-provider.tsx     login state, login/register/logout, useAuth() and useAuthed()
    auth-gate.tsx         redirects when logged out
    app-header.tsx        top bar with name and log out
    ui.tsx                Button, Field, Alert, Spinner
  lib/
    api.ts                the API client (all fetch calls live here)
    types.ts              API response types
    api.test.ts           tests for the client
```

## CORS

The browser blocks calls from `localhost:3000` to `localhost:8080` unless the API allows it.
The Go server now does this. Allowed origins come from `CORS_ORIGINS` (default: `FRONTEND_URL`).
`localhost` and `127.0.0.1` count as different origins, so open the app at the exact address
you put in `CORS_ORIGINS`.
