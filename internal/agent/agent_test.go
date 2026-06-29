package agent

import "testing"

func line(state, sess, cmd string) string {
	// pane|session|window|window_name|pid|cmd|path|state|project|model|msg|ts|sessionpath
	return "%1|" + sess + "|1|main|123|" + cmd + "|/p|" + state + "|proj|sonnet|msg|1000|/s.jsonl"
}

func TestParseLineSkipsNonAgents(t *testing.T) {
	if _, ok := ParseLine(line("", "s", "fish")); ok {
		t.Fatal("empty @pimux_state must be skipped")
	}
	if _, ok := ParseLine("too|few|fields"); ok {
		t.Fatal("malformed line must be skipped")
	}
	if _, ok := ParseLine(""); ok {
		t.Fatal("empty line must be skipped")
	}
}

func TestParseLineFields(t *testing.T) {
	a, ok := ParseLine(line("working", "alpha", "pi"))
	if !ok {
		t.Fatal("valid agent line should parse")
	}
	if a.PaneID != "%1" || a.Session != "alpha" || a.Window != 1 || a.WindowName != "main" || a.PID != 123 {
		t.Fatalf("bad fields: %+v", a)
	}
	if a.State != Working || a.Project != "proj" || a.Model != "sonnet" || a.TS != 1000 {
		t.Fatalf("bad fields: %+v", a)
	}
	if a.SessionPath != "/s.jsonl" {
		t.Fatalf("bad session path: %q", a.SessionPath)
	}
	if a.Stale {
		t.Fatal("pi foreground should not be stale")
	}
}

func TestParseLineStaleWhenNotPi(t *testing.T) {
	a, ok := ParseLine(line("working", "s", "fish"))
	if !ok || !a.Stale {
		t.Fatalf("non-pi foreground with state should be stale: %+v", a)
	}
}

func TestParseMultiline(t *testing.T) {
	out := line("working", "a", "pi") + "\n" +
		line("", "b", "fish") + "\n" + // skipped
		line("done", "c", "pi")
	got := Parse(out)
	if len(got) != 2 {
		t.Fatalf("want 2 agents, got %d", len(got))
	}
}

func TestPriorityAndLess(t *testing.T) {
	blocked, _ := ParseLine(line("blocked", "z", "pi"))
	working, _ := ParseLine(line("working", "a", "pi"))
	done, _ := ParseLine(line("done", "a", "pi"))
	idle, _ := ParseLine(line("idle", "a", "pi"))
	stale, _ := ParseLine(line("working", "a", "fish"))

	if !(Priority(blocked) < Priority(done) &&
		Priority(done) < Priority(working) &&
		Priority(working) < Priority(idle) &&
		Priority(idle) < Priority(stale)) {
		t.Fatal("priority order wrong")
	}
	// blocked sorts before working even though its session sorts later.
	if !Less(blocked, working) {
		t.Fatal("blocked should sort before working regardless of session")
	}
}

func TestHumanize(t *testing.T) {
	const now = int64(10_000_000)
	cases := map[int64]string{
		now - 5_000:     "5s",
		now - 120_000:   "2m",
		now - 7_200_000: "2h",
	}
	for ts, want := range cases {
		if got := Humanize(now, ts); got != want {
			t.Fatalf("Humanize(%d): want %s got %s", ts, want, got)
		}
	}
	if Humanize(now, 0) != "" {
		t.Fatal("zero ts should humanize to empty")
	}
}
