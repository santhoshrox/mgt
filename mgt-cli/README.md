# mgt — Stacked-PR CLI

`mgt` is a thin Go CLI that drives [`mgt-be`](../mgt-be), a self-hosted replacement for graphite.dev. The CLI runs `git` locally; everything else (stack metadata, GitHub OAuth, PR creation, AI descriptions, the merge queue) lives on the server.

## Install

```bash
sudo make install
```

## Configure

```bash
mgt config init                                          # interactive
mgt config set server_url      http://localhost:8080     # used for OAuth verification URLs
mgt config set server_grpc_addr localhost:9090           # gRPC endpoint (this is what the CLI dials)
mgt config set branch_prefix   santhosh                  # optional
mgt login                                                # opens GitHub OAuth, stores a bearer token
mgt sync-repos                                           # registers your GitHub repos with mgt-be
```

Settings live in `~/.mgt/config`; the bearer token lives in `~/.mgt/credentials` (mode 0600). Env overrides: `MGT_SERVER_URL`, `MGT_SERVER_GRPC_ADDR`, `MGT_SERVER_GRPC_INSECURE` (default `1`), `MGT_BRANCH_PREFIX`, `MGT_TOKEN`.

## Commands

| Command | What it does |
|---|---|
| `mgt login` / `mgt logout` | Authenticate with mgt-be via GitHub OAuth (browser device flow). |
| `mgt sync-repos` | Pull your GitHub repos into mgt-be. |
| `mgt status` | Show the stack the current branch belongs to. |
| `mgt create <name>` | `git checkout -b <prefix><name>` and register it with mgt-be. |
| `mgt up` / `mgt down` / `mgt top` / `mgt trunk` | Move within the stack. |
| `mgt switch [branch]` | Interactive picker (or direct checkout). |
| `mgt move <parent>` | Move the current branch onto a new parent (server returns the rebase plan). |
| `mgt extract` | Detach the current branch onto trunk. |
| `mgt reorder` | Interactive TUI to rearrange the stack. |
| `mgt restack` | Pull trunk and rebase every branch in the stack. |
| `mgt sync` | Pull trunk and delete branches whose PRs are merged. |
| `mgt submit [--ai]` | Push the current branch and create/update its PR. With `--ai` the server fills the body via LLM. |
| `mgt stack-submit [--ai]` | Same, for every branch in the stack. |
| `mgt describe` | Server (LLM) generates a fresh PR description for the current branch's open PR. |

## How it talks to mgt-be

- All commands speak gRPC (`mgt.v1.MgtService`, schema in [`mgt-proto/`](../mgt-proto)) to `MGT_SERVER_GRPC_ADDR`. The bearer token is sent as the `authorization: Bearer <token>` metadata header.
- The repo for the current working tree is resolved by parsing `git remote get-url origin` and matching against `ListRepos`. The server-side `repo_id` is cached in `<git-root>/.git/mgt-repo-id` to avoid an extra round-trip per command.
- Stack mutations (`move`, `extract`, `reorder`) call `MoveBranch`, which returns a `RebasePlan`. The CLI executes each step with `git rebase --onto`.

Connections default to plaintext (`MGT_SERVER_GRPC_INSECURE=1`) for local self-hosted setups. Set it to `0` once you put mgt-be behind a TLS-terminating proxy (Caddy, nginx, Cloud Run, …); the CLI will then dial with the system trust store.
