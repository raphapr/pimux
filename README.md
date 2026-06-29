# pimux

See every [pi](https://github.com/earendil-works/pi-mono) agent across your tmux sessions at a glance, and jump to the one that needs you.

```
tmux session  = project
  └─ pane running `pi` = an agent  →  working ● | needs-you ● | done ● | idle ○
```

Unlike tools that scrape terminal output to guess agent state, pimux uses pi's own lifecycle events: a small pi extension reports state into tmux pane options, so the signal is authoritative.

## Parts

1. **Reporter** (`extension/pimux-reporter.ts`) — a pi extension. On every pi session inside tmux it writes `@pimux_state` (and `@pimux_project`, `@pimux_model`, `@pimux_msg`, `@pimux_ts`, `@pimux_session`, `@pimux_pid`, and a window rollup `@pimux_win_state`) keyed by `$TMUX_PANE`. It is a no-op outside tmux and never blocks the agent turn.
2. **Status dots** (`pimux.conf`) — appends a colored dot to the tmux `window-status-format` based on `@pimux_state`, plus a `pane-focus-in` hook that clears the done marker once you look at a finished agent.
3. **`pimux`** — a Go/Bubble Tea popup that fuzzy-filters agents, ranks by recency, previews session content, and jumps to a pane.

## State model

| State   | When                                                       | Popup dot |
| ------- | ---------------------------------------------------------- | --------- |
| blocked | a prompt-style tool is waiting on you (or `pimux:blocked`) | ● red     |
| working | between `agent_start` and `agent_end`                      | ● yellow  |
| done    | finished, not yet looked at                                | ● teal    |
| idle    | at rest / seen                                             | ○ green   |
| stale   | option set but foreground process is no longer `pi`        | ✗ struck  |

The popup mirrors herdr's dot language. `done` means finished but unseen; tmux's `pane-focus-in` hook turns `done` into `idle` after you look.

## Install

```sh
# 1. Binary
make install                 # go build -> ~/.local/bin/pimux
# or: go install github.com/raphapr/pimux@latest

# 2. Reporter extension (global, all pi sessions)
pimux install-extension      # writes the embedded reporter -> ~/.pi/agent/extensions/

# 3. tmux integration
cp pimux.conf ~/.tmux/pimux.conf
printf '\nsource-file ~/.tmux/pimux.conf\n' >> ~/.tmux.conf   # or your tmux.conf
tmux source-file ~/.tmux/pimux.conf
```

`pimux install-extension` writes `pimux-reporter.ts` into the pi extensions dir. The reporter is embedded in the binary, so `go install` users do not need the source tree. Override the target with `--dir` or `$PIMUX_EXT_DIR`. Restart pi sessions afterward to load it.

`pimux.conf` binds the dashboard to `prefix + a`:

```tmux
bind a display-popup -E -w 60% -h 60% pimux
```

Requires tmux ≥ 3.2 and Go ≥ 1.24 to build.

## Usage

- `prefix + a` — open the dashboard popup.
- `pimux install-extension` — install the reporter pi extension (`--dir` to override the target).
- `pimux --json` — print discovered agents as JSON (scripting / debugging).
- `pimux --version`.

The popup is non-modal. The search input is always active, so printable keys edit the query. Vim-style control chords handle movement and actions.

| Key                   | Action                                                             |
| --------------------- | ------------------------------------------------------------------ |
| type                  | fuzzy-filter agents (filter-only; match score never reorders rows) |
| `Backspace`           | delete one char                                                    |
| `Ctrl-W`              | delete one word                                                    |
| `Ctrl-U`              | clear query                                                        |
| `Ctrl-J` / `Ctrl-N`   | move down                                                          |
| `Ctrl-K` / `Ctrl-P`   | move up                                                            |
| `Enter`               | jump to selected pane and close                                    |
| `Ctrl-D`              | mark selected agent seen (`done` → `idle`)                         |
| `Ctrl-X` then `y`/`n` | interrupt selected pane with confirm                               |
| `Ctrl-R`              | refresh now                                                        |
| `Tab`                 | cycle sort mode (`grouped` → `priority` → `recent`)                |
| `Esc`                 | clear query; if query is empty, quit                               |
| `Ctrl-C`              | quit                                                               |

`Ctrl-S` is intentionally unused because terminal flow control can freeze output on some systems.

## Popup sorting and preview

Sort modes:

- `grouped` (default): tmux sessions are ordered by their most-recent agent. Single-agent sessions collapse to one row; sessions with multiple agents expand to indented per-window rows.
- `priority`: flat list by herdr attention order: `blocked > done > working > idle`, with recency as the tiebreaker.
- `recent`: flat list by `@pimux_ts` newest first.

The right pane shows selected-agent details (project, model, pane id, cwd, state, elapsed time) and a read-only preview of the pi session JSONL transcript tail. It does not scrape or embed the live pi TUI.

## Blocked ("needs you") detection

The reporter is tool-name-agnostic. It flags ⚠ blocked two ways:

1. **Name patterns** — a tool whose name contains an interaction verb (`ask`,
   `question`, `confirm`, `permission`, `elicit`, `approve`) counts as waiting on
   you while it executes. Matching is token-aware, so machine tools like
   `task_runner` are not misfired by the substring `ask`.
2. **Explicit event** — any tool or extension can authoritatively flag itself:

   ```js
   pi.events.emit("pimux:blocked", {
     active: true,
     id: "deploy-gate",
     label: "approve deploy?",
   });
   // ...once the prompt is answered:
   pi.events.emit("pimux:blocked", { active: false, id: "deploy-gate" });
   ```

   Pass a stable `id` so overlapping blocks clear independently. This is the
   only reliable path for prompts pimux cannot see by name.

## Configuration (env vars, read by the reporter)

- `PIMUX_BLOCKING_TOOLS` — comma-separated patterns that override the default
  verb list above. Single tokens match a name token by prefix (`question` →
  `questions`); multi-token entries (`request_input`) match a contiguous run.
- `PIMUX_NOTIFY` — `blocked` (or `1`) to `notify-send` when an agent needs you;
  `all` to also notify on done. Unset = no notifications.
