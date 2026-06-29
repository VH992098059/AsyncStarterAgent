# Phase 0 / T005 交接文档 — 给下一窗口

> **生成时间**: 2026-06-18 (Asia/Taipei)
> **生成方式**: superpowers:subagent-driven-development（AI 助手按 ai-coding-boundary.md 执行）
> **当前任务**: T005 — Docker 开发环境配置（Dockerfile / docker-compose / .dockerignore / scripts/init-db.sql / Makefile）
> **状态**: ✅ **DONE**（静态 + Go 回归全部通过；**docker 端到端验证由用户在主窗口跑**）
> **基线 commit**: `bfbc4f0` (T004)
> **用户红线**: 本次不启动 docker（不跑 `docker compose up` / `docker build` / `docker exec` / `curl`）

---

## 1. 上一窗口做了什么

按 `doc/plans/01-phase0-foundation.md` §Task T005 (L980–L1130) 执行了 11 个 step 中的 **1–5**。**Step 6–11 主动跳过**（按用户红线：docker 端到端验证在主窗口跑）。

T005 **不涉及任何 Go 代码改动**（R4 + T005 范围），全部为 Docker / Shell / Makefile 配置。

### 1.1 新建/修改文件清单

| 文件 | 操作 | 大小 | 说明 |
|---|---|---|---|
| `Dockerfile` | 新建 | 14 行 | 多阶段构建：`golang:1.22-alpine AS build` → `alpine:3.19` runtime；CGO_ENABLED=0；EXPOSE 8080；ENTRYPOINT `/app/api` |
| `docker-compose.yml` | 新建 | 48 行 | 3 个 service：`postgres`（pgvector/pgvector:pg16 + healthcheck pg_isready） / `redis`（redis:7-alpine + healthcheck redis-cli ping） / `api`（build .，depends_on service_healthy，env 透传 APP_ENV/APP_PORT/DATABASE_URL/REDIS_URL）+ named volume `pgdata` |
| `.dockerignore` | 新建 | 10 行 | 排除 `.git` / `.idea` / `.vscode` / `bin` / `coverage.*` / `*.log` / `.env` / `.env.local` / `test` / `docs` |
| `scripts/init-db.sql` | 新建 | 3 行 | `CREATE EXTENSION IF NOT EXISTS pgcrypto;` + `CREATE EXTENSION IF NOT EXISTS vector;`（挂到 postgres 容器 `/docker-entrypoint-initdb.d/init.sql`） |
| `Makefile` | 修改 | +9/-1 | `.PHONY` 追加 `docker-build docker-logs`；`help` 文本追加 2 行说明；末尾追加 `docker-build` / `docker-logs` 目标（保留 T001 的 `docker-up` / `docker-down`） |

> **新建 4 个文件 + 修改 1 个文件 = T005 总变更 5 个对象**。

### 1.2 验证结果（仅静态 + Go 回归）

| 命令 | 结果 | 备注 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 0 错误（Go 代码无回归） |
| `go vet ./...` | ✅ exit 0 | 0 警告 |
| `go test ./...` | ✅ PASS | `ok internal/handler` + `ok internal/queue` + `ok internal/repository`（含 T002/T003/T004 全部单元测试） |
| `make -n docker-build` | ✅ OK | dry-run 展开为 `docker build -t asyncstarter/api:dev .` |
| `make -n docker-logs` | ✅ OK | dry-run 展开为 `docker compose logs -f api` |
| `make -n docker-up` | ✅ OK | dry-run 展开为 `docker compose up -d`（T001 旧目标，未变） |
| `make -n docker-down` | ✅ OK | dry-run 展开为 `docker compose down`（T001 旧目标，未变） |
| `make -n help` | ✅ OK | help 文本含新目标：`docker-build` / `docker-logs` |
| `git status` | 见 §1.4 | 1 modified (Makefile) + 3 untracked (Dockerfile / docker-compose.yml / .dockerignore) + 1 untracked dir (scripts/) |
| `git diff --stat` | 见 §1.5 | `Makefile \| 10 +++++++++-`（CRLF 警告属 Windows 正常） |

> ⚠️ **未跑**（用户红线）：
> - `docker compose up -d`
> - `docker compose config`（**本机无 docker 守护进程**——按用户红线也不该跑）
> - `docker build -t asyncstarter/api:dev .`
> - `docker exec asyncstarter_postgres psql ...`
> - `curl http://localhost:8080/health`

### 1.3 与计划/规范的小偏差

| # | 偏差 | 原因 | 是否合规 |
|---|---|---|---|
| 1 | `docker-build` / `docker-logs` 目标带 `## ` 注释（plan 锁风格），其余目标不带 | plan 原文就这么写，**保留 plan 字面** | ✅ 与 plan 一致 |
| 2 | `.PHONY` 列表中 `docker-up` / `docker-down` **保留**，与新加的 `docker-build` / `docker-logs` 并存 | T001 已存在，plan 明确"保留" | ✅ 与 plan 一致 |
| 3 | `docker-compose.yml` 用 Compose V2 格式（`docker compose` 带空格），非旧 `docker-compose` 带连字符 | plan 用了新格式 | ✅ 与 plan 一致 |
| 4 | **未**自动 `git add` / `git commit` | 按 P1 红线 + 用户决定 | ✅ 合规 |
| 5 | **未**修改任何 `.go` 文件 | R4 + T005 范围明确 | ✅ 合规 |

**没有**触发 `ai-coding-boundary.md` 的任何红线：未越界 FR、未加未声明依赖、未写 Go 代码、未自动 commit、未硬编码密钥（compose 里的 `starter/starter` 是 dev 默认密码，非生产密钥）。

### 1.4 git status 输出

```
On branch main
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  (use "git restore <file>..." to discard changes in working directory)
        modified:   Makefile

Untracked files:
  (use "git add <file>..." to include in what will be committed)
        .dockerignore
        Dockerfile
        doc/
        docker-compose.yml
        scripts/

no changes added to commit (use "git add" and/or "git commit" -a)
```

> **T005 新增的 4 个对象**（`Dockerfile` / `docker-compose.yml` / `.dockerignore` / `scripts/init-db.sql`）+ `Makefile` 修改 = 5 个变动对象待 commit。
> `doc/` 仍 untracked 是 T001 起的状态（user 一直选 NO commit handoff doc），不是 T005 引入。

### 1.5 git diff --stat

```
 Makefile | 10 +++++++++-
 1 file changed, 9 insertions(+), 1 deletion(-)
```

加上 4 个新文件 = T005 总变更。

---

## 2. 上一窗口**没做**什么

- ❌ **没有**启动 docker（用户红线）—— 不跑 `docker compose up -d` / `docker compose config` / `docker build`
- ❌ **没有**跑端到端冒烟（Step 6–11 全部跳过）—— 不验 `curl /health` / `make migrate-up` / `go test -tags=integration` / `docker exec psql`
- ❌ **没有**改任何 `.go` 文件（T005 范围明确）
- ❌ **没有**改 `go.mod` / `go.sum`（T005 不引入新 Go 依赖）
- ❌ **没有**改 `README.md`（plan 列为 optional，且 T001 已写 docker 启动说明）
- ❌ **没有**加 production k8s manifests / nginx 反代 / docker secrets / 多阶段优化（**不属于 Phase 0 范围**）
- ❌ **没有**自动 `git add` / `git commit`（按 P1 红线等用户决定）
- ❌ **没有** `docker compose config` 静态语法检查（本机无 docker；按用户红线不安装）

> ⚠️ **本机 docker 状态**：未在 PowerShell 验证 `docker --version`（避免不必要的环境探测），但即使本机有 docker，按用户红线也**不**跑 compose 验证。**用户在主窗口自行决定**要不要起 docker。

---

## 3. 下一窗口需要做的（**Phase 0 末 / Phase 1 入口**）

> **重要**：下一窗口**不**在当前 T005 流程内。T005 完成后**没有**"T006"任务；下一窗口是 **Phase 0 末的端到端验证 + Phase 1 启动**。

### 3.1 Phase 0 退出标准（M0）

来源：`doc/plans/01-phase0-foundation.md` L1134+ §"Phase 0 退出标准验证"

| # | 项 | 状态 | 由谁跑 |
|---|---|---|---|
| 1 | `go build ./...` → 0 错误 | ✅ **已验证** | T005 窗口 |
| 2 | `docker compose ps` → 3 容器 healthy | ⏳ 待用户跑 | 用户主窗口 |
| 3 | `curl http://localhost:8080/health` → 200 | ⏳ 待用户跑 | 用户主窗口 |
| 4 | `make migrate-up` → 6 张表 | ⏳ 待用户跑 | 用户主窗口 |
| 5 | `make test` → 全部 PASS | ✅ **已验证**（`go test ./...` 全过） | T005 窗口 |
| 6 | `go test -tags=integration ./internal/queue/...` → PASS | ⏳ 待用户跑（需 docker redis） | 用户主窗口 |
| 7 | `docker exec asyncstarter_postgres psql ... "SELECT count(*) FROM agent_runs;"` → 0 | ⏳ 待用户跑 | 用户主窗口 |

### 3.2 建议执行顺序（用户主窗口跑）

1. **在主窗口跑** `cd "k:\go_projects\AsyncStarterAgent" && docker compose up -d`（一次性起 3 个容器，等 healthcheck）
2. 验 `docker compose ps` → 3 容器都是 `(healthy)`
3. 跑 `curl http://localhost:8080/health` → 期望 `{"code":0,"message":"ok","data":{"status":"ok","env":"development"}}`
4. 跑 `make migrate-up` → 期望 6 张表创建
5. 跑 `go test -tags=integration ./internal/queue/...` → 期望 PASS（T004 集成测试）
6. 跑 `docker exec asyncstarter_postgres psql -U starter -d starter -c "SELECT count(*) FROM agent_runs;"` → 期望 `0`
7. 全部通过 → 更新 `doc/task-tracker.html` 中 T001–T005 状态为"已完成"
8. 写 `doc/handoff/phase0-final-handoff.md`（Phase 0 完成总结 + Phase 1 启动准备）
9. （可选）`make docker-down` 关掉容器

### 3.3 Phase 1 入口

- 计划文件：`doc/plans/02-phase1-trigger.md`（T006–T009 触发引擎）
- Phase 1 任务范围：agent_runs 表的 trigger 模型 / cron 调度 / webhook trigger / 手动 trigger UI
- 启动前置：M0 全部 ✅ + 6 张表已迁移 + `/health` 返回 200

---

## 4. 给下一窗口的提示

1. **不要**用 T005 的静态检查当 Phase 0 完成的证据——M0 退出标准里的 docker 端到端验证（容器 healthy / `/health` 200 / 6 张表 / agent_runs count=0 / 集成测试 PASS）**必须**真跑。
2. **集成测试**是 T004 留下的 `//go:build integration` 文件 `internal/queue/queue_integration_test.go`；docker redis 起来后跑 `go test -tags=integration ./internal/queue/... -v` 即可。
3. **不要**回填 T005 集成测试（如 `docker exec go test ...`）—— 那是范围外。
4. **`init-db.sql` 的执行时机**：postgres 容器**第一次启动**才会执行 `/docker-entrypoint-initdb.d/` 里的 SQL；如果之前 `docker compose up` 留下过 `pgdata` volume 且已初始化过，则 SQL 不会重跑。**建议**首次跑前先 `docker compose down -v` 清掉 volume。
5. **本机 redis 状态**（T004 留下）：之前检测过本机无 `redis-server.exe` / Windows Service / 注册表记录，**只有 docker redis 可用**。
6. **本机 docker 状态**：未在 T005 验证（避免不必要的探测）。用户在主窗口跑前可先 `docker --version` 确认。
7. **Phase 0 末**写 `phase0-final-handoff.md` 时建议包含：6 张表 schema 摘要、T001–T005 commit 哈希汇总、Phase 1 入口链接、本机环境状态、已知遗留问题。
8. **不要自动 commit** T005 的 5 个变动对象。改完跑通测试后只展示 diff，**用户在主窗口决定** `git add` + `git commit`。
9. **`Makefile` 已含** 4 个 docker-* 目标（`docker-up` / `docker-down` / `docker-build` / `docker-logs`），可一站式使用。

---

## 5. 当前文件结构（Phase 0 / T005 末 = M0 退出标准全部就位）

```
AsyncStarterAgent/
├── bin/                          [gitignored] 构建产物
│   └── api.exe                   (~7.5 MB, T002 验证用)
├── cmd/
│   └── api/
│       └── main.go               ✅ T002 (启动 HTTP server，**未连 DB / 未启 Worker**)
├── doc/
│   ├── ai-coding-boundary.md
│   ├── handoff/
│   │   ├── T001-handoff.md
│   │   ├── T002-handoff.md
│   │   ├── T003-handoff.md
│   │   ├── T004-handoff.md
│   │   └── T005-handoff.md       ← 本文件
│   ├── plans/                    (00~05)
│   ├── mvp-definition.html
│   ├── plan-boundary.md
│   ├── prototype.html
│   ├── requirement-spec.html
│   └── task-tracker.html
├── internal/
│   ├── config/
│   │   └── config.go             ✅ T001 (5 env vars，未动)
│   ├── handler/
│   │   ├── health.go             ✅ T002
│   │   └── health_test.go        ✅ T002
│   ├── middleware/
│   │   ├── logger.go             ✅ T002
│   │   └── recovery.go           ✅ T002
│   ├── queue/                    ✅ T004
│   │   ├── handler.go
│   │   ├── queue.go
│   │   ├── queue_integration_test.go  (//go:build integration)
│   │   └── queue_test.go
│   ├── repository/               ✅ T003
│   │   ├── db.go
│   │   └── db_test.go
│   └── server/
│       └── server.go             ✅ T002
├── pkg/
│   └── httpx/
│       └── response.go           ✅ T002
├── migrations/                   ✅ T003
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── scripts/                      ✅ T005
│   └── init-db.sql               (pgcrypto + vector 扩展)
├── test/
│   └── integration/              (空，Phase 0 末才用)
├── .dockerignore                 ✅ T005 (10 行)
├── .env.example                  ✅ T001
├── .gitignore                    ✅ T001
├── Dockerfile                    ✅ T005 (14 行多阶段)
├── Makefile                      ✅ T001+T003+T005 (新增 docker-build / docker-logs)
├── docker-compose.yml            ✅ T005 (48 行，3 service + healthcheck + volume)
├── go.mod                        ✅ T002+T003+T004 (gin v1.12.0 + pgx v5.10.0 + asynq v0.26.0)
├── go.sum                        ✅ T002+T003+T004
└── README.md                     ✅ T001
```

---

## 6. 待用户决定（必须问，不可以自动做）

按 ai-coding-boundary P1 / §3.6：

1. **是否 `git add` 当前 T005 的 5 个变动对象并提交？**
   - 建议命令（**用户在主窗口执行**）：
     ```bash
     cd "k:\go_projects\AsyncStarterAgent"
     git add Dockerfile docker-compose.yml .dockerignore scripts/init-db.sql Makefile
     git commit -m "feat(phase0/T005): docker dev environment (postgres + redis + api)

     - Add Dockerfile: multi-stage build (golang:1.22-alpine build ->
       alpine:3.19 runtime), CGO_ENABLED=0, EXPOSE 8080, ENTRYPOINT /app/api
     - Add docker-compose.yml: 3 services
       (postgres: pgvector/pgvector:pg16 with healthcheck pg_isready;
        redis: redis:7-alpine with healthcheck redis-cli ping;
        api: build . with depends_on service_healthy,
        env DATABASE_URL/REDIS_URL/APP_ENV/APP_PORT)
       + named volume pgdata
     - Add .dockerignore: exclude .git / .idea / .vscode / bin / coverage.* /
       *.log / .env / .env.local / test / docs
     - Add scripts/init-db.sql: CREATE EXTENSION IF NOT EXISTS pgcrypto
       and vector (mounted to /docker-entrypoint-initdb.d/init.sql)
     - Update Makefile: add docker-build (docker build -t asyncstarter/api:dev .)
       and docker-logs (docker compose logs -f api) targets; keep
       T001's docker-up / docker-down intact
     - Did NOT start docker (per user red line); verified Go regression
       (go build / vet / test all pass) and make -n dry runs only"
     ```
   - 如果用户说 NO，则保持当前未提交状态。

2. **Phase 0 末端到端验证时机**：现在就跑（用户在主窗口执行 §3.2 的 1-6 步）还是稍后跑？
   - 建议**现在**跑（趁热打铁），跑完即可进入 Phase 1。

3. **Dockerfile base 镜像版本**：plan 锁 `golang:1.22-alpine`（T005 严格按 plan 写），本机 `go 1.25.5`。
   - **方案 A（按 plan）**：保持 `golang:1.22-alpine`（**当前 T005 状态**）
   - **方案 B（对齐本机）**：改成 `golang:1.25-alpine` 或 `golang:1.25.5-alpine`（需新 commit）
   - 默认建议 A（plan 一致性）。如选 B，需再提交一个 chore 提交。

4. **T004 集成测试 + Phase 0 末**：用户在主窗口跑 §3.2 第 5 步后，T004 留下的 `//go:build integration` 测试可正式从"compile-only"升级为"端到端 PASS"。

5. **Phase 0 末要不要写 `phase0-final-handoff.md`**？建议写（含 commit 哈希汇总、Phase 1 启动 checklist、6 张表 schema 摘要），方便 Phase 1 开局读。

> **下一窗口（Phase 0 末）开场建议**：
> 1. 读 `doc/handoff/T005-handoff.md`（本文件）
> 2. 读 `doc/plans/01-phase0-foundation.md` L1134+ §"Phase 0 退出标准验证"
> 3. 在主窗口跑 §3.2 的 1-6 步
> 4. 全过 → 写 `doc/handoff/phase0-final-handoff.md`
> 5. 打开 `doc/plans/02-phase1-trigger.md` 进入 Phase 1
