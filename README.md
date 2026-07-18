# AsyncStarterAgent

**Do the first 30% before you even open the task.**

AsyncStarterAgent watches for the moment a task is *about to* start — a keyword mentioned in chat, a deadline creeping closer, a webhook from your tools — then automatically gathers context from everywhere that task lives (GitHub, calendar, IM, notes), drafts it with an LLM, and streams the draft back to you or writes it straight into Notion, Obsidian, or Feishu docs. You open the task to find it already started.

[**English**](./README.md) · [简体中文](./README.zh-CN.md)

---

## Table of Contents

- [Why](#why)
- [How it works](#how-it-works)
- [Features](#features)
- [Tech stack](#tech-stack)
- [Quick start](#quick-start)
- [Project layout](#project-layout)
- [Documentation](#documentation)
- [Status](#status)
- [Contributing](#contributing)

## Why

Most "AI task" tools wait for you to open a chat window and describe what you need. By the time you do that, you've already spent the effort of remembering the context, finding the links, and framing the ask.

AsyncStarterAgent flips that: it watches for trigger signals in the background, harvests the relevant context *before* you ask, and hands you a draft that's already 30% done. You review, edit the gaps, and ship — instead of starting from a blank page.

## How it works

A four-stage pipeline, streamed end-to-end over SSE:

```
 Trigger              Harvest                Synthesize             Deliver
┌─────────────┐      ┌───────────────┐      ┌──────────────┐      ┌─────────────────┐
│ keyword hit  │ ──▶  │ GitHub        │ ──▶  │ RAG retrieval│ ──▶  │ Notion page      │
│ DDL approach │      │ Calendar      │      │ Eino agent   │      │ Obsidian vault   │
│ webhook event│      │ IM messages   │      │ LLM drafting │      │ Feishu doc       │
└─────────────┘      │ Notes         │      │ SSE stream   │      │ comment writeback│
                      └───────────────┘      └──────────────┘      └─────────────────┘
```

- **Trigger** (`internal/trigger`) — keyword matching, DDL polling scheduler, signed webhook ingestion (Todoist, Feishu)
- **Harvest** (`internal/harvesting`) — pluggable source adapters + incremental ETL into embeddings
- **Synthesize** (`internal/synthesis`) — vector search (pgvector) + Eino DAG/Workflow agent orchestration + LLM drafting, streamed via SSE
- **Deliver** (`internal/delivery`) — writes the draft back to Notion, Obsidian, or Feishu, including comment writeback on the originating thread

## Features

- **Multi-channel triggers** — keyword rules, deadline polling, and signed webhooks, all feeding one dedupe'd run queue
- **Context harvesting** — adapters for GitHub, calendar, IM, and notes, synced incrementally instead of re-fetched every run
- **RAG + agent drafting** — pgvector retrieval feeds an [Eino](https://github.com/cloudwego/eino) DAG/Workflow agent that drafts with an OpenAI-compatible model
- **Streamed output** — drafts arrive over SSE as they're generated, not after a long blocking call
- **Write-back delivery** — lands directly in Notion, Obsidian, or Feishu docs, with `[TODO]`-style markers where the agent wasn't confident
- **Per-user Feishu credentials** — each user can register their own Feishu self-built app (App ID/Secret encrypted at rest via pgcrypto) instead of sharing one global app
- **Desktop client** — Tauri + React shell for reviewing and editing drafts locally

## Tech stack

| Layer | Stack |
|---|---|
| Backend | Go 1.26 · Gin · [Eino](https://github.com/cloudwego/eino) (LLM/agent orchestration) · PostgreSQL 16 + pgvector · Redis 7 · Asynq |
| Integrations | GitHub API · Feishu (Lark) OpenAPI SDK · Google APIs (Calendar) · OpenAI-compatible models |
| Auth & security | JWT · pgcrypto-encrypted credential storage · OAuth2 |
| Desktop client | Tauri v2 (Rust shell) · React 19 · TypeScript · Tailwind CSS 4 · assistant-ui · Tiptap |

## Quick start

```bash
make docker-up    # start PostgreSQL + Redis
make build        # compile
make run          # run the API server
```

Health check: `curl http://localhost:8080/health`

Other useful targets: `make test`, `make migrate-up`, `make docker-logs` — see `Makefile` for the full list.

## Project layout

```
cmd/            entry points (API server, wiring)
internal/
  trigger/      keyword / DDL / webhook trigger engine
  harvesting/   source adapters + ETL
  synthesis/    RAG + Eino agent + LLM drafting
  delivery/     Notion / Obsidian / Feishu write-back
  feishu/       Feishu OAuth, token & per-user client management
  handler/      HTTP handlers
migrations/     SQL migrations
web/            Tauri + React desktop client
doc/            requirement spec, MVP definition, task tracker, decision log
docs/superpowers/  spec-driven plans for individual feature branches
```

## Documentation

- Requirements: `doc/requirement-spec.html`
- MVP definition: `doc/mvp-definition.html`
- Implementation plans: `doc/plans/00-index.md`
- AI coding boundary (rules for AI-assisted changes): `doc/ai-coding-boundary.md`
- Task tracker (authoritative progress source): `doc/task-tracker.html`

## Status

Core pipeline (trigger → harvest → synthesize → deliver) and the Feishu integration are implemented and covered by passing unit tests. The Flutter alternative frontend (evaluated alongside Tauri) hasn't been started. Per-user Feishu app credentials are implemented on a feature branch pending final review. See `doc/task-tracker.html` for the up-to-date, task-by-task breakdown — it supersedes any progress notes elsewhere in this repo.

## Contributing

This project follows a spec-driven workflow: design specs and implementation plans live under `docs/superpowers/`, changes go through task-by-task diffs, and AI-assisted edits must follow `doc/ai-coding-boundary.md`.
