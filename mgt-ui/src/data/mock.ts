// CLI command reference rendered on the dashboard. Everything else moved
// from mock data to live data sourced from mgt-be.
export const mgtCommands = [
  { cmd: 'mgt login', desc: 'Authenticate with mgt-be via GitHub OAuth (browser device flow).' },
  { cmd: 'mgt sync-repos', desc: 'Pull your GitHub repos into mgt-be.' },
  { cmd: 'mgt up', desc: 'Move toward the tip of the current stack.' },
  { cmd: 'mgt down', desc: 'Move toward trunk.' },
  { cmd: 'mgt top', desc: 'Jump to the tip of your current stack.' },
  { cmd: 'mgt trunk', desc: 'Switch to the trunk branch.' },
  { cmd: 'mgt create <name>', desc: 'Create a new stacked branch (with optional prefix).' },
  { cmd: 'mgt submit [--ai]', desc: 'Push the current branch and create/update its PR.' },
  { cmd: 'mgt stack-submit [--ai]', desc: 'Submit every branch in the stack.' },
  { cmd: 'mgt describe', desc: 'Have the server fill the open PR description via LLM.' },
  { cmd: 'mgt sync', desc: 'Pull trunk and clean up merged branches.' },
  { cmd: 'mgt restack', desc: 'Pull trunk and rebase the current stack.' },
]
