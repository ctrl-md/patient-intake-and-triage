# Staff auth

Staff log in with email and password and get a JWT. The token goes in the
`Authorization: Bearer <token>` header on every protected route.

| Method | Route | Needs token | What it does |
|---|---|---|---|
| POST | `/auth/register` | no | Create a staff account (role is always `nurse`) |
| POST | `/auth/login` | no | Log in, get a token |
| GET | `/auth/me` | yes | Who am I (reads the account from the database) |

## Try it (PowerShell)

```powershell
$body = @{ email = "ada@clinic.com"; password = "supersecret1"; full_name = "Ada Obi" } | ConvertTo-Json
$r = Invoke-RestMethod -Method Post -Uri http://localhost:8080/auth/register -ContentType "application/json" -Body $body
$token = $r.token

Invoke-RestMethod -Uri http://localhost:8080/auth/me -Headers @{ Authorization = "Bearer $token" }
```

Log in again later:

```powershell
$body = @{ email = "ada@clinic.com"; password = "supersecret1" } | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri http://localhost:8080/auth/login -ContentType "application/json" -Body $body
```

## Roles

New accounts are always `nurse`. A request cannot choose its own role, or anyone could make
themselves an admin. To make someone a doctor or admin for now, change it in the database:

```powershell
docker exec -it triagely-db psql -U triagely -d triagely -c "UPDATE staff SET role='doctor' WHERE email='ada@clinic.com';"
```

The role is saved inside the token, so the person must log in again to get the new role.

Use `auth.RequireRole("doctor", "admin")` after `auth.Middleware` to protect a route by role.

## Settings

| Variable | Meaning |
|---|---|
| `JWT_SECRET` | Required. At least 32 characters. The server will not start without it. |
| `JWT_TTL` | How long a login lasts. Default `12h`. |

## Safety notes

- Passwords are hashed with bcrypt. Only the hash is stored.
- Login gives the same error for a wrong password and an unknown email.
- Tokens are HS256 and only tokens with our own exact header are accepted, so `alg: none` tricks fail.
- Passwords must be 8 to 72 characters (bcrypt only reads the first 72 bytes).
- Not done yet: login rate limiting and token refresh.
