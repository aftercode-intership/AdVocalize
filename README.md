# 🎙️ Vocalize — AI-Powered Audio Advertisement Platform

> From product description to broadcast-ready audio in Arabic, French, and English.  
> Powered by **GLM-4.7 Flash**, **Chatterbox TTS**, and a Go/Next.js full-stack.

## Overview

Vocalize is a SaaS platform for generating professional audio advertisements using AI. Users describe their product, choose a language and tone, and receive a broadcast-ready script — with audio synthesis and video generation coming in later sprints.

**Current features (Sprints 1–3):**
- User registration, login, JWT auth, Google OAuth
- AI chat interface (the landing page for authenticated users)
- Script generation via GLM-4.7 Flash
- Arabic diacritization (tashkeel) for TTS quality
- Credit system, session management, prompt enhancement

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        DOCKER NETWORK                        │
│                                                             │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐  │
│  │  Next.js 15  │───▶│  Go/Fiber v3 │───▶│ Python/FastAPI│ │
│  │  :3000       │    │  :8081       │    │  :8001        │  │
│  │  (Frontend)  │    │  (Backend)   │    │  (script-gen) │  │
│  └──────────────┘    └──────┬───────┘    └──────────────┘  │
│                             │                               │
│             ┌───────────────┼───────────────┐               │
│             ▼               ▼               ▼               │
│        ┌─────────┐   ┌──────────┐   ┌──────────┐           │
│        │Postgres │   │  Redis   │   │  MinIO   │           │
│        │  :5432  │   │  :6379   │   │  :9000   │           │
│        └─────────┘   └──────────┘   └──────────┘           │
└─────────────────────────────────────────────────────────────┘
```

**Tech stack:**

| Layer | Technology |
|---|---|
| Frontend | Next.js 15, React 18, Tailwind CSS v3, TypeScript |
| Backend | Go 1.25, Fiber v3, fasthttp WebSocket |
| Script Gen | Python 3.9, FastAPI, camel-tools, GLM API |
| Database | PostgreSQL 16 |
| Cache / Queue | Redis 7 |
| File Storage | MinIO (S3-compatible) |
| AI | ZhipuAI GLM-4.7 Flash |
| Auth | JWT + Google OAuth 2.0 |
| Observability | Prometheus, Grafana, Loki, Promtail |
| Proxy | Traefik v2 |

---

## Prerequisites

Install these before starting:

| Tool | Version | Install |
|---|---|---|
| **Go** | 1.21+ | https://go.dev/dl/ |
| **Node.js** | 20+ | https://nodejs.org |
| **Docker Desktop** | Latest | https://www.docker.com/products/docker-desktop |
| **Python** | 3.9 (for local dev only) | https://www.python.org/downloads/ |
| **Git** | Any | https://git-scm.com |

> ⚠️ Python **3.9 specifically** is required for `camel-tools` locally. Python 3.10+ will fail to build `camel-kenlm`. In production, Docker handles this automatically.

---

## Project Structure

```
vocalize/
├── backend/                    # Go/Fiber API
│   ├── main.go
│   ├── go.mod
│   ├── .env                    # Backend environment variables
│   ├── migrations/
│   │   └── sql/
│   │       ├── 1_initial_schema.sql
│   │       ├── 2_password_reset_tokens.sql
│   │       └── 3_missing_tables.sql
│   └── internal/
│       ├── config/             # JWT, env loading
│       ├── database/           # DB connection
│       ├── handlers/           # HTTP handlers
│       ├── middleware/         # Auth, RBAC, rate limiting
│       ├── models/             # DB models, DTOs
│       └── services/           # Business logic
│
├── frontend/                   # Next.js app
│   ├── src/
│   │   ├── app/                # Next.js App Router pages
│   │   ├── components/         # React components
│   │   │   ├── Auth/           # LoginPage, RegisterPage
│   │   │   ├── Chat/           # ChatWindow, ChatInput, ChatSidebar
│   │   │   └── Generate/       # ProductForm, ScriptResult
│   │   ├── hooks/              # useAuth, useChat
│   │   └── lib/                # utils
│   ├── .env.local              # Frontend environment variables
│   └── package.json
│
├── services/
│   └── script-gen/             # Python FastAPI service
│       ├── main.py
│       ├── config.py
│       ├── Dockerfile
│       ├── requirements.txt        # Docker/Linux (camel-tools included)
│       ├── requirements-dev.txt    # Windows dev (no camel-tools)
│       └── services/
│           ├── script_generator.py
│           └── diacritizer.py
│
├── infra/                      # Prometheus, Grafana, Loki configs
├── docker-compose.yml          # Full stack orchestration
└── .env                        # Root environment (Docker Compose)
```

---

## Environment Setup

### Step 1 — Clone the repository

```bash
git clone https://github.com/your-org/vocalize.git
cd vocalize
```

---

## Infrastructure (Docker)

Start all infrastructure services (database, cache, storage, monitoring):

```bash
# Start infrastructure only (recommended first)
docker compose up -d postgres redis minio

# Wait ~10 seconds for Postgres to be ready, then verify:
docker compose ps
```

Expected output — all should show `healthy`:
```
NAME                    STATUS
vocalize-postgres       running (healthy)
vocalize-redis          running (healthy)
vocalize-minio          running
```

### Start optional monitoring stack

```bash
docker compose up -d loki promtail prometheus grafana traefik
```

Access Grafana at http://localhost:3001 (admin / admin)

---

## Database Migrations

Migrations run automatically on first `docker compose up` via the `docker-entrypoint-initdb.d/` mount. For subsequent changes, run them manually:

```bash
# Run all migrations in order
docker compose exec postgres psql -U vocalize -d vocalize_db \
  -f /docker-entrypoint-initdb.d/1_initial_schema.sql

docker compose exec postgres psql -U vocalize -d vocalize_db \
  -f /docker-entrypoint-initdb.d/2_password_reset_tokens.sql

docker compose exec postgres psql -U vocalize -d vocalize_db \
  -f /docker-entrypoint-initdb.d/3_missing_tables.sql
```

> 💡 All migrations use `CREATE TABLE IF NOT EXISTS` — safe to re-run.

### Verify tables exist

```bash
docker compose exec postgres psql -U vocalize -d vocalize_db -c "\dt"
```

You should see:

```
 Schema |            Name             |
--------+-----------------------------+
 public | campaigns                   |
 public | chat_message_versions       |
 public | chat_messages               |
 public | chat_sessions               |          ← required for chat
 public | credit_transactions         |
 public | email_verification_tokens   |          ← required for register
 public | generated_ads               |
 public | password_reset_tokens       |          ← required for forgot password
 public | sessions                    |
 public | support_escalations         |
 public | users                       |
```

---

## Backend (Go)

### Install dependencies

```bash
cd backend
go mod download
```

### Run locally

```bash
go run main.go
# Expected: 🚀 Vocalize backend running on http://localhost:8081
```

### Verify it works

```bash
curl http://localhost:8081/health
# Expected: {"status":"ok"}
```

### Build for production

```bash
go build -o bin/vocalize main.go
./bin/vocalize
```

---

## Script Generation Service (Python)

### Option A — Run inside Docker (recommended)

```bash
# Build the Docker image (first time: ~5–10 minutes, downloads camel-tools)
docker compose build script-gen

# Start the service
docker compose up -d script-gen

# Watch startup logs (model download happens here on first run)
docker compose logs -f script-gen
```

Wait for:
```
INFO: Uvicorn running on http://0.0.0.0:8001
```

> ⚠️ The first build compiles `camel-kenlm` from C++ source inside the Linux container. This takes 5–10 minutes. Subsequent builds use Docker's layer cache and are instant.

### Option B — Run locally on Windows (dev only)

> ⚠️ Do **not** use `requirements.txt` on Windows — `camel-tools` requires a C++ compiler. Use `requirements-dev.txt` instead.

```bash
cd services/script-gen

# Create a virtual environment with Python 3.9
py -3.9 -m venv .venv
.venv\Scripts\activate

# Install dev dependencies (no camel-tools, no C++ required)
pip install -r requirements-dev.txt

# Run the service
python main.py
# Expected: Uvicorn running on http://0.0.0.0:8001
```

### Test the service

```bash
curl -X POST http://localhost:8001/api/generate/script \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "Premium Headphones",
    "product_description": "Wireless noise-canceling headphones with 30h battery",
    "target_audience": "Tech professionals aged 25-40",
    "tone": "FORMAL",
    "language": "en"
  }'
```

Expected response:
```json
{
  "script_text": "...",
  "language": "en",
  "tone": "FORMAL",
  "word_count": 95,
  "estimated_duration_seconds": 32
}
```

---

## Frontend (Next.js)

### Install dependencies

```bash
cd frontend
npm install
```

### Run in development mode

```bash
npm run dev
# Expected: ▲ Next.js 15 ready on http://localhost:3000
```

### Build for production

```bash
npm run build
npm start
```

---

## Google OAuth Setup

OAuth requires a Google Cloud project. Follow these steps once:

1. Go to https://console.cloud.google.com
2. Create a new project (or select existing)
3. Navigate to **APIs & Services → Credentials**
4. Click **Create Credentials → OAuth 2.0 Client ID**
5. Application type: **Web application**
6. Add to **Authorized JavaScript origins**:
   ```
   http://localhost:3000
   ```
7. Add to **Authorized redirect URIs**:
   ```
   http://localhost:8081/api/auth/google/callback
   ```
   > ⚠️ This **must** point to port `8081` (the Go backend), **not** port `3000`. The backend handles the OAuth exchange and then redirects the user back to the frontend.

8. Copy the **Client ID** and **Client Secret** into `backend/.env`:
   ```env
   GOOGLE_CLIENT_ID=xxxxx.apps.googleusercontent.com
   GOOGLE_CLIENT_SECRET=GOCSPX-xxxxx
   GOOGLE_REDIRECT_URL=http://localhost:8081/api/auth/google/callback
   ```

---

## Running Everything Together

### Quick start (all services)

Open 3 terminal windows:

**Terminal 1 — Infrastructure + script-gen:**
```bash
cd vocalize
docker compose up -d postgres redis minio
docker compose up script-gen
```

**Terminal 2 — Go backend:**
```bash
cd vocalize/backend
go run main.go
```

**Terminal 3 — Next.js frontend:**
```bash
cd vocalize/frontend
npm run dev
```

Then open http://localhost:3000

### Full Docker stack (alternative)

Uncomment the `backend` and `frontend` services in `docker-compose.yml`, then:

```bash
docker compose up -d
```

---

## Verifying the Setup

Run through this checklist in order. Each step depends on the previous one.

**1. Infrastructure**
```bash
docker compose ps
# postgres, redis, minio should all be "healthy"
```

**2. Database tables**
```bash
docker compose exec postgres psql -U vocalize -d vocalize_db -c "\dt"
# Should list ~11 tables including chat_sessions and email_verification_tokens
```

**3. Backend health**
```bash
curl http://localhost:8081/health
# {"status":"ok"}
```

**4. Script-gen health**
```bash
curl http://localhost:8001/health
# {"status":"ok","service":"vocalize-script-gen"}
```

**5. Frontend**
```
Open http://localhost:3000
→ Should show the landing page (unauthenticated)
```

**6. Registration**
```
Go to http://localhost:3000/register
→ Create an account
→ Should redirect to /verify-email page
```

**7. Login**
```
Go to http://localhost:3000/login
→ Log in with your new account
→ Should redirect to / and show the chat interface
```

**8. Chat**
```
On the chat page:
→ Click "New Chat" in the sidebar
→ Select a topic
→ Type a message and press Ctrl+Enter
→ AI response should appear after ~2 seconds
```

**9. Script generation**
```
Go to http://localhost:3000/generate
→ Fill in the product form
→ Click "Generate Ad Script"
→ Script should appear in the right panel
```

---

## Common Issues & Fixes

### `chat_sessions table does not exist` (500 on all chat endpoints)

The migration hasn't run. Fix:
```bash
docker compose exec postgres psql -U vocalize -d vocalize_db \
  -f /docker-entrypoint-initdb.d/3_missing_tables.sql
```

### `CORS policy: No 'Access-Control-Allow-Origin'`

The backend CORS middleware isn't registering. Check that `app.Use(cors.New(...))` is the **first** middleware in `main.go`, before all route groups.

### `redirect_uri_mismatch` on Google OAuth

Your Google Cloud Console redirect URI is wrong. It must be:
```
http://localhost:8081/api/auth/google/callback
```
**Not** `localhost:3000`. Also verify `GOOGLE_REDIRECT_URL` in `backend/.env` matches exactly.

### `camel-kenlm build error` on Windows

Do not run `pip install -r requirements.txt` on Windows. Use:
```bash
pip install -r requirements-dev.txt
```
The full `requirements.txt` is for Docker (Linux) only.

### Backend starts but `GLM_API_KEY` fatal error

Add your ZhipuAI key to `backend/.env`:
```env
GLM_API_KEY=your_actual_key_here
```
Get a key at https://open.bigmodel.cn

### `POST /api/chat/sessions 400 Bad Request`

The topic sent by the frontend (`"general"`) was rejected by the validator. Fix: replace `chat_handler.go` with the version that includes `general` in the `oneof` validator:
```go
Topic string `json:"topic" validate:"required,oneof=general script_refinement creative_ideas support"`
```

### Frontend shows landing page after login

The `useAuth` hook was reading `data` instead of `data.user` from the `/api/auth/me` response. Fix: update `useAuth.ts` to use `setUser(data.user || data)`.

### Password reset emails not sending

Set `SEND_EMAILS=false` in `backend/.env` during development. The backend will log the reset token to stdout instead of emailing it. Check backend logs:
```bash
# Look for a line like:
# Email sending disabled. Would send reset email to: user@example.com
```
You can then call `POST /api/auth/reset-password` directly with the token from the logs.

---

## Services & Ports

| Service | Port | URL | Credentials |
|---|---|---|---|
| Frontend (Next.js) | 3000 | http://localhost:3000 | — |
| Backend (Go API) | 8081 | http://localhost:8081 | — |
| Script-gen (Python) | 8001 | http://localhost:8001 | — |
| PostgreSQL | 5432 | — | vocalize / localpassword |
| Redis | 6379 | — | — |
| MinIO Console | 9001 | http://localhost:9001 | minioadmin / minioadmin |
| MinIO S3 API | 9000 | http://localhost:9000 | — |
| Grafana | 3001 | http://localhost:3001 | admin / admin |
| Prometheus | 9090 | http://localhost:9090 | — |
| Loki | 3100 | http://localhost:3100 | — |
| Traefik Dashboard | 8090 | http://localhost:8090 | — |

---

## Sprint Roadmap

| Sprint | Feature | Status |
|---|---|---|
| 1 | Infrastructure (Docker, Postgres, Redis, MinIO, monitoring) | ✅ Done |
| 2 | Authentication (register, login, JWT, Google OAuth) | ✅ Done |
| 3 | Script generation (Python service → GLM → Go API → Frontend form) | ✅ Done |
| 3.5 | Chat page (AI conversations, session history, prompt enhancement) | ✅ Done |
| 4 | Arabic Khaliji localization (full tashkeel pipeline) | 🔜 Next |
| 5 | TTS audio synthesis (Chatterbox-Turbo or edge-tts fallback) | 🔜 Planned |
| 6 | Audio player & download | 🔜 Planned |
| 7 | Voice cloning | 🔜 Planned |
| 8 | Campaign management | 🔜 Planned |
| 9 | Video generation | 🔜 Planned |
| 10 | Billing (Stripe), EU AI Act compliance | 🔜 Planned |

---

## API Quick Reference

```
# Auth
POST   /api/auth/register
POST   /api/auth/login
POST   /api/auth/logout
GET    /api/auth/me                     (protected)
GET    /api/auth/google/login
GET    /api/auth/google/callback
POST   /api/auth/forgot-password
POST   /api/auth/reset-password

# Profile
GET    /api/profile                     (protected)
PATCH  /api/profile                     (protected)

# Chat
GET    /api/chat/sessions               (protected)
POST   /api/chat/sessions               (protected)
GET    /api/chat/sessions/:id/history   (protected)
POST   /api/chat/sessions/:id/message   (protected)
DELETE /api/chat/sessions/:id           (protected)
POST   /api/chat/enhance                (protected)
WS     /api/chat/sessions/:id/ws        (protected)

# Script Generation
POST   /api/generate/script             (protected)
GET    /api/generate/script/:id         (protected)

# Library
GET    /api/library/history             (protected)

# Health
GET    /health                          (public)
```

---

## License

Proprietary — Sonic Azure © 2024. All rights reserved.
