# mgt-be

Self-hosted backend for [`mgt-cli`](../mgt-cli) and [`mgt-ui`](../mgt-ui). A small Go service that owns:

- GitHub OAuth (web flow for the UI, browser device-style flow for the CLI).
- Stack metadata (replaces `gt`'s local git refs — Postgres is the source of truth).
- A pull-request cache synced from GitHub via REST + webhooks, mirrored into OpenSearch for full-text search.
- Server-side LLM-backed PR description generation.
- A FIFO merge queue with retry/backoff, driven by webhook events.

## Two transports

| Listener | Default port | Consumed by | Notes |
|---|---|---|---|
| REST (chi) | `:8080` | `mgt-ui`, OAuth callbacks, GitHub webhooks | Browser-friendly, session cookies + CORS. |
| gRPC | `:9090` | `mgt-cli` | Bearer-token auth via `authorization` metadata. Schema lives in [`mgt-proto/`](../mgt-proto). |

Both listeners share the same `*core.Core` (db pool, encrypted token sealer, device-flow registry, merge-queue notifier, AI + search clients) so e.g. starting a CLI device flow over gRPC and completing it via the browser OAuth callback Just Works.

## Layout

```
cmd/server/main.go         # boots both REST and gRPC listeners
internal/
  api/                     # chi router + HTTP handlers (REST, used by UI)
  grpcserver/              # gRPC implementation of mgt.v1.MgtService (used by CLI)
  core/                    # shared dependency bundle (db, sealer, device flow, …)
  auth/                    # OAuth + sessions + CLI device flow + bearer tokens
  github/                  # tiny GitHub REST client
  stacks/                  # pure stack-tree planning logic
  ai/                      # LLM client (porting of mgt-cli/pkg/ai)
  search/                  # OpenSearch indexer + query
  mergequeue/              # in-process FIFO worker
  db/                      # pgx pool, embedded migrations, query helpers
  crypto/                  # AES-GCM sealing for OAuth tokens / webhook secrets
  config/                  # env loader
```

## Run locally

```bash
DATABASE_URL=postgres://mgt:mgt@localhost:5432/mgt?sslmode=disable \
OPENSEARCH_URL=http://localhost:9200 \
GITHUB_CLIENT_ID=... GITHUB_CLIENT_SECRET=... \
MGT_MASTER_KEY=$(openssl rand -base64 48 | tr -d '\n') \
SESSION_SECRET=$(openssl rand -base64 48 | tr -d '\n') \
go run ./cmd/server
```

Migrations run automatically on boot (embedded `internal/db/migrations/*.sql`).

## API surface

### REST (`:8080` — for `mgt-ui`)

- `GET  /healthz`
- OAuth + auth: `GET /auth/github/login`, `GET /auth/github/callback`, `POST /auth/logout`
- `POST /webhooks/github` (HMAC-verified via `GITHUB_WEBHOOK_SECRET`)
- `GET  /api/me`
- `GET  /api/repos`, `POST /api/repos/sync`
- Stacks: `GET /api/repos/{id}/stacks`, `GET .../by-branch?name=`, `POST .../stacks`, `POST .../branches`, `PATCH .../branches/{bid}` (returns a rebase plan), `DELETE`
- Pulls: `GET .../pulls?state=&author=&q=`, `GET .../pulls/{n}`, `POST .../pulls?ai=1`, `POST .../pulls/{n}/describe`
- Merge queue: `GET /merge-queue`, `POST /merge-queue`, `DELETE /merge-queue/{eid}`

### gRPC (`:9090` — for `mgt-cli`)

Service `mgt.v1.MgtService`. Schema in [`mgt-proto/proto/mgt/v1/mgt.proto`](../mgt-proto/proto/mgt/v1/mgt.proto). Methods mirror the REST surface (Me, ListRepos, SyncRepos, ListStacks, StackByBranch, CreateStack, AddBranch, MoveBranch, DeleteBranch, ListPullRequests, CreatePullRequest, DescribePullRequest, ListMergeQueue, EnqueueMerge, CancelMerge) plus the unauthenticated `DeviceStart` / `DevicePoll` used during `mgt login`.

Auth: pass `authorization: Bearer <token>` in metadata. The same API tokens (and the same device-flow handshake) work over both transports.

## Environment

| Variable | Default | Notes |
|---|---|---|
| `MGT_ADDR` | `:8080` | REST listen address. |
| `MGT_GRPC_ADDR` | `:9090` | gRPC listen address. |
| `MGT_BASE_URL` | `http://localhost:8080` | Public URL; used in OAuth redirect_uri. |
| `MGT_UI_BASE_URL` | `http://localhost:3000` | UI origin (CORS allow-list). |
| `DATABASE_URL` | — required — | Postgres DSN. |
| `OPENSEARCH_URL` | — | Set to enable PR search; otherwise `q=` is ignored. |
| `GITHUB_CLIENT_ID/SECRET` | — required for OAuth — | OAuth App credentials. |
| `GITHUB_WEBHOOK_SECRET` | — | If unset, signature checks are skipped (dev only). |
| `MGT_MASTER_KEY` | — required — | AES-256 key for sealing OAuth tokens. ≥32 chars. |
| `SESSION_SECRET` | — required — | HMAC-SHA256 secret for UI session cookies. ≥32 chars. |
| `LLM_API_KEY` | — | Enables AI describe / `?ai=1` on submit. |
| `LLM_BASE_URL` | `https://api.openai.com/v1` | OpenAI-compatible. |
| `LLM_MODEL` | `gpt-4o-mini` | |
| `MGT_QUEUE_POLL_SECONDS` | `30` | Backstop poll for the merge queue worker. |
| `MGT_WORKER_ENABLED` | `true` | Disable to run a read-only replica. |
