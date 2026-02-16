# mgt - Improved Charcoal (gt) CLI

`mgt` is a powerful, **fully-transparent** wrapper for the [Charcoal (gt)](https://github.com/danerwilliams/charcoal) CLI. It implements several productivity-focused enhancements while seamlessly proxying any other commands directly to `gt`.

## Features

- **`mgt up`**: Move up the stack. If on trunk, opens an interactive stack selector.
- **`mgt down`**: Move down the stack.
- **`mgt trunk`**: Quickly switch to the main/trunk branch.
- **`mgt top`**: Jump to the tip of your current stack.
- **`mgt create [name]`**: Create a new stacked branch (passthrough to `gt branch create`). Optional branch prefix via `~/.mgt` or `MGT_BRANCH_PREFIX` (e.g. `santhosh/` → branch `my-feature` becomes `santhosh/my-feature`).
- **`mgt submit`**: Submit **ONLY** the current branch as a PR (avoids title loops).
- **`mgt stack-submit`**: Submit the **entire** stack as PRs when needed.
- **`mgt sync`**: Pull trunk and cleanup merged branches (alias: `cleanup`, `prune`).
- **`mgt restack`**: Pull trunk and restack the entire current chain.
- **`mgt config set/get`**: Set or show config (e.g. `branch_prefix`); creates `~/.mgt` if needed. Omit value to clear (no prefix).

## Prerequisites

- **Go 1.25.6** or higher
- **[Charcoal (gt)](https://github.com/danerwilliams/charcoal)**

```bash
# Install Charcoal via Homebrew
brew install danerwilliams/tap/charcoal
```

## Installation

```bash
# Build and install to /usr/local/bin
sudo make install
```

## Configuration

Settings are read from **`~/.mgt`** (in your home directory). Environment variables override the file.

**Set from the CLI** (creates `~/.mgt` if missing):

```bash
mgt config set branch_prefix santhosh    # use prefix santhosh/
mgt config set branch_prefix             # no prefix (clear)
mgt config get branch_prefix             # show current value
mgt config init                          # prompt for values (reads from stdin)
```

**Interactive or piped setup** — `mgt config init` asks questions and reads answers from stdin (works when piped, e.g. `echo 'santhosh' | mgt config init`).

Or edit **`~/.mgt`** by hand with `key=value` lines:

```ini
# Branch name prefix for "mgt create" (e.g. santhosh/my-feature). Omit or leave empty for no prefix.
branch_prefix=santhosh
```

| Source | Key / Variable | Description |
|--------|----------------|-------------|
| `~/.mgt` | `branch_prefix` | Prefix for new branch names when using `mgt create`. A trailing `/` is added if omitted. Leave empty for no prefix. |
| env | `MGT_BRANCH_PREFIX` | Same as above; overrides `~/.mgt` when set. |

## Why mgt?

Charcoal is great, but `mgt` adds:
1. **Interactive Selector from Trunk**: `gt branch up` usually fails on trunk; `mgt up` turns it into a stack picker.
2. **TTY Transparency**: Uses `syscall.Exec` so that Charcoal's interactive prompts work flawlessly inside the wrapper.
3. **Safe Submissions**: Protects you from the "infinite title prompt" loop by defaulting `submit` to just the current branch.
4. **Branch Prefix**: Optional `MGT_BRANCH_PREFIX` so new stacks get names like `santhosh/<branch_name>` (Charcoal has no built-in prefix).
5. **Transparent Proxying**: Any command not explicitly overriden by `mgt` is automatically passed through to `gt`.
