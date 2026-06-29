package agent

import (
	"sort"
	"strings"

	"github.com/sahilm/fuzzy"
)

// SortMode controls how agents are ordered in the popup.
type SortMode string

const (
	Grouped      SortMode = "grouped"
	PriorityMode SortMode = "priority"
	Recent       SortMode = "recent"
)

// Order returns a sorted copy of agents for the requested mode. Recency is the
// universal tiebreaker, so equally important rows never fall back to arbitrary
// map or tmux order.
func Order(agents []Agent, mode SortMode) []Agent {
	out := append([]Agent(nil), agents...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch mode {
		case PriorityMode:
			if pa, pb := Priority(a), Priority(b); pa != pb {
				return pa < pb
			}
		case Recent:
			// handled by recency fallback below
		default:
			if a.Session != b.Session {
				return sessionRecent(agents, a.Session) > sessionRecent(agents, b.Session)
			}
		}
		if a.TS != b.TS {
			return a.TS > b.TS
		}
		if a.Session != b.Session {
			return a.Session < b.Session
		}
		if a.Window != b.Window {
			return a.Window < b.Window
		}
		return a.PaneID < b.PaneID
	})
	return out
}

func sessionRecent(agents []Agent, session string) int64 {
	var latest int64
	for _, a := range agents {
		if a.Session == session && a.TS > latest {
			latest = a.TS
		}
	}
	return latest
}

// Filter returns agents whose haystack fuzzy-matches query. It preserves input
// order and returns rune indexes for highlighting each matched pane label.
func Filter(agents []Agent, query string) ([]Agent, map[string][]int) {
	query = strings.TrimSpace(query)
	if query == "" {
		return append([]Agent(nil), agents...), map[string][]int{}
	}
	var out []Agent
	marks := map[string][]int{}
	for _, a := range agents {
		hay := SearchHaystack(a)
		matches := fuzzy.Find(query, []string{hay})
		if len(matches) == 0 {
			continue
		}
		out = append(out, a)
		marks[a.PaneID] = matches[0].MatchedIndexes
	}
	return out, marks
}

// SearchHaystack collects stable, user-visible fields for fuzzy matching.
func SearchHaystack(a Agent) string {
	return strings.Join([]string{
		a.Session,
		a.WindowName,
		a.Project,
		a.Model,
		a.Msg,
		string(a.State),
		a.PaneID,
		a.Path,
	}, " ")
}
