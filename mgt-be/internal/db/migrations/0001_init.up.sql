-- Users authenticated via GitHub OAuth.
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    github_id BIGINT NOT NULL UNIQUE,
    login TEXT NOT NULL,
    avatar_url TEXT,
    encrypted_oauth_token TEXT NOT NULL,
    scopes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Long-lived bearer tokens used by the CLI. The token itself is never stored:
-- token_hash is sha256(token).
CREATE TABLE api_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    label TEXT NOT NULL DEFAULT 'cli',
    last_used_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX api_tokens_user_idx ON api_tokens(user_id) WHERE revoked_at IS NULL;

-- Repositories registered with mgt-be (the user must have triggered a sync).
CREATE TABLE repositories (
    id BIGSERIAL PRIMARY KEY,
    github_id BIGINT NOT NULL UNIQUE,
    owner TEXT NOT NULL,
    name TEXT NOT NULL,
    default_branch TEXT NOT NULL DEFAULT 'main',
    webhook_secret_enc TEXT,
    installed_by_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (owner, name)
);

-- A "stack" represents a chain of branches a user is working on in a repo.
CREATE TABLE stacks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    repo_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    name TEXT,
    trunk_branch TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX stacks_user_repo_idx ON stacks(user_id, repo_id);

-- Branches in a stack form a tree (adjacency list). parent_id NULL means the
-- branch sits directly on top of trunk.
CREATE TABLE stack_branches (
    id BIGSERIAL PRIMARY KEY,
    stack_id BIGINT NOT NULL REFERENCES stacks(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    parent_id BIGINT REFERENCES stack_branches(id) ON DELETE SET NULL,
    position INT NOT NULL DEFAULT 0,
    head_sha TEXT,
    pr_id BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (stack_id, name)
);
CREATE INDEX stack_branches_parent_idx ON stack_branches(parent_id);

CREATE TABLE pull_requests (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    number INT NOT NULL,
    title TEXT NOT NULL,
    body TEXT,
    state TEXT NOT NULL,           -- open, closed
    merged BOOLEAN NOT NULL DEFAULT FALSE,
    draft BOOLEAN NOT NULL DEFAULT FALSE,
    author_login TEXT NOT NULL,
    author_avatar_url TEXT,
    head_branch TEXT NOT NULL,
    base_branch TEXT NOT NULL,
    head_sha TEXT,
    mergeable_state TEXT,
    ci_state TEXT,                 -- success, failure, pending, neutral, ...
    additions INT NOT NULL DEFAULT 0,
    deletions INT NOT NULL DEFAULT 0,
    comments INT NOT NULL DEFAULT 0,
    html_url TEXT NOT NULL,
    updated_at_gh TIMESTAMPTZ,
    synced_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (repo_id, number)
);
CREATE INDEX pull_requests_state_idx ON pull_requests(repo_id, state);
CREATE INDEX pull_requests_author_idx ON pull_requests(author_login);

-- Allow stack_branches.pr_id to reference pull_requests. Done after the
-- pull_requests table exists so the FK can be added.
ALTER TABLE stack_branches
    ADD CONSTRAINT stack_branches_pr_id_fkey
    FOREIGN KEY (pr_id) REFERENCES pull_requests(id) ON DELETE SET NULL;

CREATE TYPE merge_queue_state AS ENUM (
    'queued', 'integrating', 'awaiting_ci', 'merging', 'merged', 'failed', 'cancelled'
);

CREATE TABLE merge_queue_entries (
    id BIGSERIAL PRIMARY KEY,
    repo_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    pr_id BIGINT NOT NULL REFERENCES pull_requests(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    state merge_queue_state NOT NULL DEFAULT 'queued',
    position INT NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    last_error TEXT,
    last_event_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    enqueued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    merged_at TIMESTAMPTZ,
    UNIQUE (repo_id, pr_id)
);
CREATE INDEX merge_queue_active_idx ON merge_queue_entries(repo_id, state, position)
    WHERE state IN ('queued','integrating','awaiting_ci','merging');

CREATE TABLE webhook_events (
    id BIGSERIAL PRIMARY KEY,
    delivery_id TEXT NOT NULL UNIQUE,
    repo_id BIGINT REFERENCES repositories(id) ON DELETE SET NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);
CREATE INDEX webhook_events_unprocessed_idx ON webhook_events(received_at) WHERE processed_at IS NULL;
