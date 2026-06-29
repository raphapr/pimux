package agent

import "testing"

func mk(id, session string, state State, ts int64) Agent {
	return Agent{PaneID: id, Session: session, Project: session, WindowName: "main", State: state, TS: ts, Cmd: "pi"}
}

func TestOrderGroupedRanksSessionsByMostRecent(t *testing.T) {
	agents := []Agent{
		mk("%1", "old", Working, 100),
		mk("%2", "new", Idle, 300),
		mk("%3", "old", Blocked, 200),
	}

	got := Order(agents, Grouped)
	want := []string{"%2", "%3", "%1"}
	assertPaneOrder(t, got, want)
}

func TestOrderPriorityUsesHerdrAttentionThenRecent(t *testing.T) {
	agents := []Agent{
		mk("%1", "a", Working, 400),
		mk("%2", "b", Done, 100),
		mk("%3", "c", Blocked, 50),
		mk("%4", "d", Idle, 999),
	}

	got := Order(agents, PriorityMode)
	want := []string{"%3", "%2", "%1", "%4"}
	assertPaneOrder(t, got, want)
}

func TestOrderRecentIgnoresPriority(t *testing.T) {
	agents := []Agent{
		mk("%1", "a", Blocked, 10),
		mk("%2", "b", Idle, 30),
		mk("%3", "c", Working, 20),
	}

	got := Order(agents, Recent)
	want := []string{"%2", "%3", "%1"}
	assertPaneOrder(t, got, want)
}

func TestFilterKeepsOrderAndReturnsHighlightIndexes(t *testing.T) {
	agents := []Agent{
		{PaneID: "%1", Session: "alpha", Project: "alpha", WindowName: "main", Model: "sonnet", State: Working},
		{PaneID: "%2", Session: "beta", Project: "beta", WindowName: "review", Model: "opus", State: Blocked},
	}

	got, marks := Filter(agents, "alp")
	assertPaneOrder(t, got, []string{"%1"})
	if len(marks["%1"]) == 0 {
		t.Fatal("expected match highlight indexes for matched pane")
	}
}

func assertPaneOrder(t *testing.T, got []Agent, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len mismatch: got %d want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i].PaneID != want[i] {
			t.Fatalf("at %d got %s want %s (full=%v)", i, got[i].PaneID, want[i], got)
		}
	}
}
