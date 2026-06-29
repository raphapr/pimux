# pimux

See every [pi](https://github.com/earendil-works/pi-mono) agent across your tmux sessions at a glance, and jump to the one that needs you.

```
tmux session  = project
  └─ pane running `pi` = an agent  →  working ⚙ | needs-you ⚠ | done ✓ | idle ○
```

Unlike tools that scrape terminal output to guess agent state, pimux uses pi's own lifecycle events: a small pi extension reports state into tmux pane options, so the signal is authoritative.

## Parts

1. **Reporter** (`extension/pimux-reporter.ts`) — a pi extension. On every pi session inside tmux it writes `@pimux_state` (and `@pimux_project`, `@pimux_model`, `@pimux_msg`, `@pimux_ts`, `@pimux_session`, `@pimux_pid`, and a window rollup `@pimux_win_state`) keyed by `$TMUX_PANE`. It is a no-op outside tmux and never blocks the agent turn.
2. **Status dots** (`pimux.conf`) — appends a colored dot to the tmux `window-status-format` based on `@pimux_state`, plus a `pane-focus-in` hook that clears the blue ✓ once you look at a finished agent.
3. **`pimux`** — a Go/Bubble Tea popup that lists agents grouped by session, previews the selected pane, and jumps to it.

## State model

| State   | When                                                | Dot      |
| ------- | --------------------------------------------------- | -------- |
| working | between `agent_start` and `agent_end`               | ⚙ yellow |
| blocked | a prompt-style tool is waiting on you (or `pimux:blocked`)  | ⚠ red    |
| done    | finished, not yet looked at                         | ✓ blue   |
| idle    | at rest / seen                                      | ○ dim    |
| stale   | option set but foreground process is no longer `pi` | ✗ struck |

## Install

```sh
# 1. Binary
make install                 # go build -> ~/.local/bin/pimux

# 2. Reporter extension (global, all pi sessions)
make install-extension       # copies extension/pimux-reporter.ts -> ~/.pi/agent/extensions/

# 3. tmux integration
cp pimux.conf ~/.tmux/pimux.conf
printf '\nsource-file ~/.tmux/pimux.conf\n' >> ~/.tmux.conf   # or your tmux.conf
tmux source-file ~/.tmux/pimux.conf
```

`pimux.conf` binds the dashboard to `prefix + g`:

```tmux
bind g display-popup -E -w 90% -h 80% pimux
```

Requires tmux ≥ 3.2 and Go ≥ 1.24 to build.

## Usage

- `prefix + g` — open the dashboard popup.
- In the dashboard: `enter` jump · `j`/`k` move · `/` filter · `d` mark seen ·
  `x` interrupt (confirm) · `r` refresh · `q` quit.
- `pimux --json` — print discovered agents as JSON (scripting / debugging).
- `pimux --version`.

## Blocked ("needs you") detection

The reporter is tool-name-agnostic. It flags ⚠ blocked two ways:

1. **Name patterns** — a tool whose name contains an interaction verb (`ask`,
   `question`, `confirm`, `permission`, `elicit`, `approve`) counts as waiting on
   you while it executes. Matching is token-aware, so machine tools like
   `task_runner` are not misfired by the substring `ask`.
2. **Explicit event** — any tool or extension can authoritatively flag itself:

   ```js
   pi.events.emit("pimux:blocked", { active: true, id: "deploy-gate", label: "approve deploy?" });
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

## Limitations

- **One agent per window** is assumed for the window rollup; with multiple panes the dot follows the window's active pane.
- **Built-in tool-permission prompts** are not event-exposed by pi (they are `ctx.ui.confirm` calls, not tools), so they do not raise ⚠ unless the gating code emits `pimux:blocked`. Prompt-style *tools* are caught by name.
- A crashed pi can leave a stale option until the pane closes; pimux marks such rows `stale` (foreground command is no longer `pi`).
