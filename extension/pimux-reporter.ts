// pimux reporter: report pi agent state into tmux pane/window options.
//
// Why a tmux sink (not herdr's socket): pi already self-reports lifecycle
// state; writing it to tmux options keyed by $TMUX_PANE lets the tmux status
// line and the `pimux` binary see every agent without scraping terminal
// output. Reuses the herdr-agent-state.ts state model (working/blocked/idle)
// and adds a viewer-side "done" (finished, unseen) that the tmux
// pane-focus-in hook clears to "idle".
//
// @ts-nocheck

// --- Pure, testable core -------------------------------------------------

// tmux state values written to @pimux_state / @pimux_win_state.
// "done" means finished-but-unseen; the focus hook rewrites it to "idle".
export type TmuxState = "working" | "blocked" | "done" | "idle";

export type TmuxArgs = string[];

export interface ReporterDeps {
	pane: string; // $TMUX_PANE, e.g. "%300"
	project: string; // display name (sesh/session or cwd basename)
	model: string;
	pid: number;
	sessionPath: string;
	now: () => number;
	emit: (args: TmuxArgs) => void; // runs `tmux <args>` (or records, in tests)
	// Called once on each transition INTO "blocked" or "done". The wiring decides
	// (via $PIMUX_NOTIFY) whether to actually raise a desktop/tmux notification.
	notify?: (state: TmuxState, project: string, msg: string) => void;
}

export function basename(p: string): string {
	if (!p) return "";
	const parts = p.replace(/\/+$/, "").split("/");
	return parts[parts.length - 1] || p;
}

// Reporter owns the agent's tmux option state for one pi session.
// It only emits a write when an option value actually changes, so it is cheap
// to call publish() liberally.
export class Reporter {
	private deps: ReporterDeps;
	private agentActive = false;
	private hasWorked = false;
	private blocked = new Map<string, string>(); // toolCallId -> label
	private lastState: TmuxState | undefined;
	private lastMsg: string | undefined;
	private started = false;

	constructor(deps: ReporterDeps) {
		this.deps = deps;
	}

	// session_start: register the agent at rest and write static metadata once.
	start(): void {
		if (this.started) return;
		this.started = true;
		const d = this.deps;
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_project", d.project]);
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_model", d.model]);
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_pid", String(d.pid)]);
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_session", d.sessionPath]);
		this.publish(true);
	}

	agentStart(): void {
		this.agentActive = true;
		this.hasWorked = true;
		this.publish();
	}

	agentEnd(): void {
		this.agentActive = false;
		this.publish();
	}

	// A blocking tool (ask_user_question) started executing -> waiting on the user.
	blockStart(id: string, label = "needs input"): void {
		if (!id) return;
		this.blocked.set(id, label);
		this.publish();
	}

	blockEnd(id: string): void {
		if (!id) return;
		this.blocked.delete(id);
		this.publish();
	}

	// session_shutdown: clear options so a returning shell pane goes dark.
	shutdown(): void {
		const d = this.deps;
		for (const key of [
			"@pimux_state",
			"@pimux_project",
			"@pimux_model",
			"@pimux_pid",
			"@pimux_session",
			"@pimux_msg",
			"@pimux_ts",
		]) {
			d.emit(["set-option", "-p", "-u", "-t", d.pane, key]);
		}
		d.emit(["set-option", "-w", "-u", "-t", d.pane, "@pimux_win_state"]);
		this.lastState = undefined;
		this.lastMsg = undefined;
	}

	private desired(): TmuxState {
		if (this.blocked.size > 0) return "blocked";
		if (this.agentActive) return "working";
		return this.hasWorked ? "done" : "idle";
	}

	private currentMsg(state: TmuxState): string {
		if (state === "blocked") {
			// label of the most recently added blocking tool
			let label = "";
			for (const v of this.blocked.values()) label = v;
			return label;
		}
		return "";
	}

	private publish(force = false): void {
		const d = this.deps;
		const state = this.desired();
		const msg = this.currentMsg(state);
		if (!force && state === this.lastState && msg === this.lastMsg) return;
		const entered = state !== this.lastState;
		this.lastState = state;
		this.lastMsg = msg;
		if (entered && (state === "blocked" || state === "done")) {
			try {
				d.notify?.(state, d.project, msg);
			} catch {
				/* notifications must never break reporting */
			}
		}
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_state", state]);
		d.emit(["set-option", "-w", "-t", d.pane, "@pimux_win_state", state]);
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_msg", msg]);
		d.emit(["set-option", "-p", "-t", d.pane, "@pimux_ts", String(d.now())]);
	}
}

// Tools whose execution means "the agent is waiting on the user".
export function blockingTools(): Set<string> {
	const raw = (globalThis as any).process?.env?.PIMUX_BLOCKING_TOOLS;
	if (raw && typeof raw === "string") {
		return new Set(
			raw
				.split(",")
				.map((s) => s.trim())
				.filter(Boolean),
		);
	}
	return new Set(["ask_user_question"]);
}

// --- pi extension wiring -------------------------------------------------

function safeModel(ctx: any): string {
	try {
		const m = ctx?.model;
		if (!m) return "";
		if (typeof m === "string") return m;
		return m.id ?? m.modelId ?? "";
	} catch {
		return "";
	}
}

export default function (pi: any) {
	const env = (globalThis as any).process?.env ?? {};
	// No-op outside tmux: the entire feature is tmux-scoped.
	if (!env.TMUX || !env.TMUX_PANE) return;
	const pane: string = env.TMUX_PANE;
	const tools = blockingTools();
	// $PIMUX_NOTIFY: unset/"" = off, "1"|"blocked" = notify on blocked,
	// "all" = notify on blocked and done.
	const notifyLevel = String(env.PIMUX_NOTIFY ?? "").trim().toLowerCase();

	// Fire-and-forget tmux write. Never throw, never block the turn loop.
	const run = (args: TmuxArgs) => {
		try {
			const p = pi.exec?.("tmux", args);
			if (p && typeof p.catch === "function") p.catch(() => {});
		} catch {
			/* swallow: visibility must never break the agent */
		}
	};

	let reporter: Reporter | undefined;

	pi.on("session_start", (_event: any, ctx: any) => {
		if (ctx?.hasUI !== true) return;
		const cwd = ctx?.cwd ?? env.PWD ?? "";
		let sessionPath = "";
		try {
			sessionPath = ctx?.sessionManager?.getSessionFile?.() ?? "";
		} catch {
			sessionPath = "";
		}
		reporter = new Reporter({
			pane,
			project: basename(cwd),
			model: safeModel(ctx),
			pid: env.PID ? Number(env.PID) : (globalThis as any).process?.pid ?? 0,
			sessionPath,
			now: Date.now,
			emit: run,
			notify: (state, project, msg) => {
				if (!notifyLevel) return;
				if (state === "done" && notifyLevel !== "all") return;
				const body =
					state === "blocked" ? `${project}: ${msg || "needs input"}` : `${project}: done`;
				try {
					const p = pi.exec?.("notify-send", ["pi", body]);
					if (p && typeof p.catch === "function") p.catch(() => {});
				} catch {
					/* swallow */
				}
			},
		});
		reporter.start();
	});

	pi.on("agent_start", () => reporter?.agentStart());
	pi.on("agent_end", () => reporter?.agentEnd());

	pi.on("tool_execution_start", (event: any) => {
		if (event?.toolName && tools.has(event.toolName)) {
			reporter?.blockStart(event.toolCallId, "needs input");
		}
	});
	pi.on("tool_execution_end", (event: any) => {
		if (event?.toolName && tools.has(event.toolName)) {
			reporter?.blockEnd(event.toolCallId);
		}
	});

	pi.on("session_shutdown", () => reporter?.shutdown());
}
