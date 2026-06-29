import test from "node:test";
import assert from "node:assert/strict";
import { Reporter, basename, blockingTools } from "./pimux-reporter.ts";

// Build a Reporter with a spy that records the tmux state it last wrote.
function make() {
	const writes = [];
	const r = new Reporter({
		pane: "%1",
		project: "demo",
		model: "sonnet",
		pid: 123,
		sessionPath: "/s.jsonl",
		now: () => 1000,
		emit: (args) => writes.push(args),
	});
	const stateOf = () => {
		// last @pimux_state value written
		let v;
		for (const a of writes) {
			const i = a.indexOf("@pimux_state");
			if (i !== -1 && a[0] === "set-option" && !a.includes("-u")) v = a[i + 1];
		}
		return v;
	};
	const winStateOf = () => {
		let v;
		for (const a of writes) {
			const i = a.indexOf("@pimux_win_state");
			if (i !== -1 && a[0] === "set-option" && !a.includes("-u")) v = a[i + 1];
		}
		return v;
	};
	return { r, writes, stateOf, winStateOf };
}

test("starts idle and writes metadata once", () => {
	const { r, writes, stateOf } = make();
	r.start();
	assert.equal(stateOf(), "idle");
	assert.ok(writes.some((a) => a.includes("@pimux_project") && a.includes("demo")));
	assert.ok(writes.some((a) => a.includes("@pimux_model")));
	// pane + window state both written
	assert.ok(writes.some((a) => a.includes("@pimux_win_state")));
});

test("working during a turn, done after it finishes", () => {
	const { r, stateOf, winStateOf } = make();
	r.start();
	r.agentStart();
	assert.equal(stateOf(), "working");
	assert.equal(winStateOf(), "working");
	r.agentEnd();
	assert.equal(stateOf(), "done"); // finished, unseen
});

test("blocked has priority over working and clears on tool end", () => {
	const { r, stateOf } = make();
	r.start();
	r.agentStart();
	r.blockStart("call_1", "needs input");
	assert.equal(stateOf(), "blocked");
	r.blockEnd("call_1");
	assert.equal(stateOf(), "working"); // turn still active
	r.agentEnd();
	assert.equal(stateOf(), "done");
});

test("multiple concurrent blocks require all to clear", () => {
	const { r, stateOf } = make();
	r.start();
	r.agentStart();
	r.blockStart("a");
	r.blockStart("b");
	r.blockEnd("a");
	assert.equal(stateOf(), "blocked"); // b still pending
	r.blockEnd("b");
	assert.equal(stateOf(), "working");
});

test("idle before any work stays idle (no false done)", () => {
	const { r, stateOf } = make();
	r.start();
	assert.equal(stateOf(), "idle");
});

test("publish only writes on change", () => {
	const { r, writes } = make();
	r.start();
	const n1 = writes.length;
	r.agentStart();
	const n2 = writes.length;
	r.agentStart(); // no state change
	const n3 = writes.length;
	assert.ok(n2 > n1, "agentStart writes");
	assert.equal(n3, n2, "redundant agentStart writes nothing");
});

test("shutdown unsets pane and window options", () => {
	const { r, writes } = make();
	r.start();
	r.agentStart();
	r.shutdown();
	assert.ok(writes.some((a) => a.includes("-u") && a.includes("@pimux_state")));
	assert.ok(writes.some((a) => a.includes("-u") && a.includes("@pimux_win_state")));
});

test("basename handles paths and trailing slashes", () => {
	assert.equal(basename("/home/raphael/repos/ls-n8n"), "ls-n8n");
	assert.equal(basename("/home/raphael/repos/ls-n8n/"), "ls-n8n");
	assert.equal(basename(""), "");
});

test("blockingTools defaults to ask_user_question", () => {
	const t = blockingTools();
	assert.ok(t.has("ask_user_question"));
});

test("notify fires only on entering blocked and done", () => {
	const notes = [];
	const r = new Reporter({
		pane: "%1",
		project: "demo",
		model: "m",
		pid: 1,
		sessionPath: "/s",
		now: () => 0,
		emit: () => {},
		notify: (state, project, msg) => notes.push([state, project, msg]),
	});
	r.start(); // idle -> no notify
	r.agentStart(); // working -> no notify
	r.blockStart("c1", "approve"); // -> blocked notify
	r.blockEnd("c1"); // back to working -> no notify
	r.agentEnd(); // -> done notify
	assert.deepEqual(
		notes.map((n) => n[0]),
		["blocked", "done"],
	);
	assert.equal(notes[0][1], "demo");
	assert.equal(notes[0][2], "approve");
});
