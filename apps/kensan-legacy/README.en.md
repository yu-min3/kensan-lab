# Kensan

<p align="center">
  <img src="docs/design/kensan-logo-dark.svg" alt="Kensan Logo" width="200">
</p>

<p align="center">
  <strong>Self-Improving AI Agent for Engineer Productivity</strong>
</p>

<p align="center">
  <a href="README.md">日本語</a> | English
</p>

<p align="center">
  <a href="#overview">Overview</a> |
  <a href="#features">Features</a> |
  <a href="#tech-stack">Tech Stack</a> |
  <a href="#getting-started">Getting Started</a> |
  <a href="#architecture">Architecture</a> |
  <a href="#development-with-claude-code">Claude Code</a> |
  <a href="#project-structure">Project Structure</a>
</p>

---

## Overview

Kensan is a personal productivity platform for software engineers, featuring an AI agent that **co-evolves with its user**. It integrates goal management, time block planning, learning notes, and an AI chat agent with 39 tools. The agent evaluates its own prompts weekly using Gemini, generates improvements, and lets the user approve changes via blind A/B testing — creating a feedback loop where the agent gets smarter the more you use it.

---

## Features

- **Goal & Task Management** - Hierarchical goals with milestones and tasks, Kanban board view
- **Time Block Planning** - Visual daily/weekly time block planning with drag & drop
- **Rich Note Editor** - TipTap-based rich text editor with image support and semantic search
- **AI Chat Agent** - ADK-based Gemini agent with 39 tools and Read/Write separation (reads execute immediately, writes require user approval)
- **Prompt Self-Evaluation** - Weekly batch where Gemini evaluates the agent's own prompts, identifies weaknesses, and generates improved versions
- **Blind A/B Testing** - Users compare current vs improved prompts in a blind test and vote to adopt or reject changes
- **AI Weekly Review** - Automated structured weekly review generation
- **Fact Extraction** - Automatic extraction of user preferences, habits, and skills from conversations
- **Analytics Dashboard** - Visualize productivity trends and goal progress
- **Data Lakehouse** - Medallion Architecture (Bronze/Silver/Gold) with Apache Iceberg, Dagster, and Polaris REST Catalog
- **Observability** - Full OpenTelemetry integration (Grafana, Prometheus, Loki, Tempo) with AI interaction explorer

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Frontend** | React 18, TypeScript, Vite, Zustand, Tailwind CSS 4, shadcn/ui, TipTap |
| **Backend** | Go 1.24, Chi router, pgx, JWT authentication |
| **AI Service** | Python 3.12, FastAPI, Google ADK (Agent Development Kit), Gemini 2.0 Flash |
| **Database** | PostgreSQL 16 + pgvector |
| **Storage** | MinIO (S3-compatible object storage) |
| **Data Pipeline** | Apache Iceberg, Dagster, Polaris REST Catalog |
| **Observability** | OpenTelemetry, Grafana, Prometheus, Loki, Tempo |
| **Infrastructure** | Docker Compose, GCE (Google Compute Engine) |

---

## Getting Started

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/) & Docker Compose
- [Google AI Studio API Key](https://aistudio.google.com/apikey) (for Gemini)

### 1. Clone the repository

```bash
git clone https://github.com/yu-min3/kensan-mockup.git
cd kensan-mockup
```

### 2. Configure environment variables

```bash
cp .env.example .env
```

Edit `.env` and set your Google API key:

```bash
GOOGLE_API_KEY=your-google-api-key-here
```

### 3. Start all services

```bash
make up
```

This starts all services via Docker Compose:
- Frontend: http://localhost:5173
- Backend APIs: http://localhost:8081-8091
- Grafana: http://localhost:3000

### 4. Login

Use the demo account:
- Email: `test@kensan.dev`
- Password: `password123`

### Local Development (without Docker)

```bash
# Frontend
cd frontend
npm install
npm run dev

# Backend
cd backend && make build && make test

# AI Service
cd kensan-ai
pip install -e .
uvicorn kensan_ai.main:app --reload --port 8089
```

---

## Architecture

Kensan follows a **React SPA + Go Microservices + Python AI Service** architecture.

```
Browser (React SPA)
  ├── user-service     (Go, :8081) - Auth, Settings
  ├── task-service     (Go, :8082) - Goals, Tasks
  ├── timeblock-service(Go, :8084) - Time Planning
  ├── analytics-service(Go, :8088) - Analytics
  ├── memo-service     (Go, :8090) - Memo
  ├── note-service     (Go, :8091) - Notes + MinIO
  └── kensan-ai        (Py, :8089) - AI Chat (ADK + Gemini 2.0 Flash)
                            │
                      PostgreSQL 16 + pgvector
```

### Clean Architecture & Microservices

Each Go service follows a strict **layered architecture** with explicit separation of concerns:

```
Handler (HTTP) → Service (Business Logic) → Repository (Data Access) → PostgreSQL
```

- Each service is **300–500 lines**, small enough to fit entirely in an AI coding agent's context window
- Interfaces at each boundary (ISP) allow independent testing and replacement
- Adding a new service is a single command: `claude /new-service <name>` (see below)
- Shared concerns (auth middleware, error handling, response envelope) live in `backend/shared/`

This structure is intentionally designed for **AI-assisted development** — Claude Code can read an entire service, understand its contract, and make safe changes without risk of unintended side effects across services.

### Documentation

For detailed architecture documentation, see:
- [Overall Architecture](ARCHITECTURE.md)
- [Backend Architecture](backend/ARCHITECTURE.md)
- [Frontend Architecture](frontend/src/ARCHITECTURE.md)
- [AI Service Architecture](kensan-ai/ARCHITECTURE.md)

---

## Development with Claude Code

Kensan is built to be developed **with** AI coding agents. The project includes a comprehensive [Claude Code](https://docs.anthropic.com/en/docs/claude-code) configuration that enforces conventions, automates repetitive tasks, and keeps documentation in sync.

### Rules (`.claude/rules/`)

7 rule files define the project's conventions. Claude Code loads these automatically and follows them in every interaction.

| Rule | What it enforces |
|------|-----------------|
| `backend-go.md` | Layered architecture, service directory structure, bootstrap pattern |
| `frontend-react.md` | Component hierarchy, Zustand stores, timezone handling |
| `api-design.md` | Response envelope format, error codes, URL patterns |
| `database.md` | Multi-tenancy (`user_id` on every table), UUID PKs, migration naming |
| `security.md` | JWT auth, SQL parameterization, no hardcoded secrets |
| `testing.md` | Table-driven tests, mock via interfaces, multi-tenancy test cases |
| `workflow.md` | Auto-run tests after changes, auto-update ARCHITECTURE.md |

### Skills (`.claude/skills/`)

6 slash commands scaffold new code following all project conventions:

| Command | What it does |
|---------|-------------|
| `/new-service <name>` | Scaffolds a full Go microservice (cmd, handler, service, repository, Dockerfile, Makefile) |
| `/new-page <Page> <prefix>` | Creates a React page with route registration and store |
| `/new-endpoint <svc> <method> <path>` | Adds an API endpoint with handler, service method, and repository query |
| `/go-test` | Runs Go tests and auto-fixes failures |
| `/build-check` | Runs frontend build + backend build in parallel |
| `/code-review` | Reviews uncommitted changes against project rules |

### Workflow Automation

The `workflow.md` rule enforces two automatic behaviors that Claude Code follows **without being asked**:

1. **Auto-testing**: Any Go change triggers `make test`; any frontend change triggers `cd frontend && npm run build`. Failures are fixed before reporting completion.
2. **Auto-documentation**: Structural changes (new services, endpoints, pages, schema) automatically update the relevant `ARCHITECTURE.md`.

This means the codebase stays consistent — tests pass, docs are current, and conventions are followed — even during rapid iteration.

---

## Project Structure

```
kensan-mockup/
├── frontend/             # React/TypeScript frontend
├── backend/              # Go microservices
│   ├── services/         # Individual service implementations
│   ├── shared/           # Shared middleware, auth, errors
│   └── migrations/       # Database migrations
├── kensan-ai/            # Python AI service (ADK + Gemini 2.0 Flash)
├── lakehouse/            # Data pipeline (Dagster + Iceberg)
├── observability/        # Monitoring config (Grafana, Prometheus)
├── docs/                 # Documentation
│   ├── spec/             # API specifications
│   ├── adr/              # Architecture Decision Records
│   ├── guides/           # Setup & development guides
│   └── design/           # Brand guidelines & logos
├── .claude/              # Claude Code configuration
│   ├── rules/            # 7 convention rule files
│   └── skills/           # 6 slash command skills
├── e2e/                  # Playwright end-to-end tests
├── k8s/                  # Kubernetes manifests
├── docker-compose.yml    # Local development orchestration
└── ARCHITECTURE.md       # Overall architecture documentation
```

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GOOGLE_API_KEY` | Yes | - | Google GenAI API key for Gemini |
| `AI_PROVIDER` | No | `google` | AI provider |
| `GOOGLE_MODEL` | No | `gemini-2.0-flash` | Gemini model |
| `JWT_SECRET` | Production | `dev-secret-key-...` | JWT signing key |
| `DB_PASSWORD` | No | `kensan` | PostgreSQL password |

See `.env.example` for the full list.

---

## License

This project is part of a hackathon submission. All rights reserved.
