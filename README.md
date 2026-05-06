# mgt

A self-hosted, graphite.dev-style toolkit for stacked pull requests.

```
mgt/
├── mgt-be/    # Go backend (chi REST + gRPC + Postgres + OpenSearch). OAuth, stacks, PR cache, merge queue, AI, webhooks.
├── mgt-cli/   # Thin Go CLI. Talks to mgt-be over gRPC; runs `git` locally.
├── mgt-ui/    # React/Vite dashboard. Talks to mgt-be over REST.
├── mgt-proto/ # Shared protobuf schema for the gRPC surface (used by mgt-be + mgt-cli).
└── docker-compose.yml
```

## Architecture

```
mgt-cli ──gRPC :9090 (Bearer)──┐                    ┌── postgres
                                ├──▶  mgt-be  ──────┤
mgt-ui  ──REST :8080 (cookie)──┘                    └── opensearch
                                       │
                                       ├──▶ GitHub REST + OAuth
                                       └──◀ GitHub webhooks
```

Stacks, PRs, the merge queue, and the LLM-backed PR-description writer all live on `mgt-be`. The CLI keeps doing local `git` (rebase/checkout/push); everything else is an RPC call. The UI uses REST so any browser/SDK can hit it without a gRPC-Web layer; the CLI uses gRPC for typed stubs and lower latency.

## Quickstart

```bash
cp .env.example .env
# fill in GITHUB_CLIENT_ID / SECRET / WEBHOOK_SECRET, generate random
# MGT_MASTER_KEY and SESSION_SECRET (≥32 chars each, e.g.
# `openssl rand -base64 48 | tr -d '\n'`).

docker compose up --build
```

When the stack is healthy:

- UI:       http://localhost:3000
- REST:     http://localhost:8080 (`/healthz`)
- gRPC:     localhost:9090 (`mgt.v1.MgtService`; CLI dials this)

Then install the CLI from `mgt-cli/`:

```bash
cd mgt-cli && sudo make install
mgt config set server_url       http://localhost:8080
mgt config set server_grpc_addr localhost:9090
mgt login         # opens GitHub OAuth in a browser
mgt sync-repos    # imports your GitHub repos
```

## GitHub OAuth setup

Create an OAuth App at https://github.com/settings/developers with:

- Homepage URL: `$MGT_UI_BASE_URL` (defaults to `http://localhost:3000`)
- Authorization callback URL: `$MGT_BASE_URL/auth/github/callback`

Copy the client ID + secret into `.env`.

## GitHub webhooks (optional but recommended)

For each repo you want server-managed merging on, add a webhook:

- Payload URL: `$MGT_BASE_URL/webhooks/github`
- Content type: `application/json`
- Secret: same as `GITHUB_WEBHOOK_SECRET`
- Events: Pull requests, Check runs, Check suites, Statuses

Without webhooks the merge queue still works, but it falls back to a 30s
polling loop.

## Components

| Component | Doc |
|---|---|
| Backend  | [mgt-be/README.md](mgt-be/README.md) |
| CLI      | [mgt-cli/README.md](mgt-cli/README.md) |
| UI       | [mgt-ui/README.md](mgt-ui/README.md)  |
| Proto    | [mgt-proto/](mgt-proto/) — `mgt.v1` schema; regenerate stubs with `make gen`. |
