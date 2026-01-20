# mgt - Improved Charcoal (gt) CLI

`mgt` is a powerful wrapper for the [Charcoal (gt)](https://github.com/danerwilliams/charcoal) CLI, designed to streamline stacked branch workflows with better navigation, TTY transparency, and smarter submission defaults.

## Features

- **`mgt up`**: Move up the stack. If on trunk, opens an interactive stack selector.
- **`mgt down`**: Move down the stack.
- **`mgt trunk`**: Quickly switch to the main/trunk branch.
- **`mgt top`**: Jump to the tip of your current stack.
- **`mgt create [name]`**: Create a new stacked branch (passthrough to `gt branch create`).
- **`mgt submit`**: Submit **ONLY** the current branch as a PR (avoids title loops).
- **`mgt stack-submit`**: Submit the **entire** stack as PRs when needed.
- **`mgt sync`**: Pull trunk and cleanup merged branches (alias: `cleanup`, `prune`).
- **`mgt restack`**: Pull trunk and restack the entire current chain.

## Installation

```bash
# Build and install to /usr/local/bin
sudo make install
```

## Why mgt?

Charcoal is great, but `mgt` adds:
1. **Interactive Selector from Trunk**: `gt branch up` usually fails on trunk; `mgt up` turns it into a stack picker.
2. **TTY Transparency**: Uses `syscall.Exec` so that Charcoal's interactive prompts work flawlessly inside the wrapper.
3. **Safe Submissions**: Protects you from the "infinite title prompt" loop by defaulting `submit` to just the current branch.
