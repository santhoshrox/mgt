# mgt-ui

A React/Vite dashboard for [`mgt-be`](../mgt-be), the self-hosted graphite-style backend. The UI no longer talks to GitHub directly — it consumes mgt-be's REST API over a session cookie.

## Pages

- **Dashboard** — open PRs grouped by ownership, repo picker, CLI cheatsheet.
- **Stacks** — every stack tracked by mgt-be for the active repo.
- **Inbox** — searchable, filterable PR list (q goes through OpenSearch when configured).
- **AI Reviews** — re-generate any open PR's description via the server-configured LLM.
- **Merge Queue** — enqueue/cancel PRs against the FIFO merge queue.
- **Insights** — basic throughput metrics derived from cached PRs.

## Auth

Login redirects to `/auth/github/login` on the backend. The backend handles GitHub's OAuth round-trip and sets a `mgt_session` HTTP-only cookie. All API calls use `credentials: 'include'`.

## Configure

```bash
cp .env.local.example .env.local
# VITE_API_URL=http://localhost:8080
npm install
npm run dev
```

## Build

```bash
npm run build
npm run preview
```
