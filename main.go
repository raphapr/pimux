// Command pimux is a tmux-native dashboard for pi agents: it shows every pi
// agent across your tmux sessions (working / blocked / done / idle) and lets
// you jump to one. State is reported by the pimux-reporter pi extension into
// tmux pane options; nothing here scrapes terminal output.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/raphapr/pimux/internal/agent"
	"github.com/raphapr/pimux/internal/tmux"
	"github.com/raphapr/pimux/internal/ui"
)

const version = "0.1.0"

func main() {
	jsonOut := flag.Bool("json", false, "print discovered agents as JSON and exit")
	showVer := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVer {
		fmt.Println("pimux", version)
		return
	}

	if *jsonOut {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		agents, err := tmux.List(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "pimux:", err)
			os.Exit(1)
		}
		sort.SliceStable(agents, func(i, j int) bool { return agent.Less(agents[i], agents[j]) })
		if agents == nil {
			agents = []agent.Agent{} // emit [] not null when no agents report
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(agents); err != nil {
			fmt.Fprintln(os.Stderr, "pimux:", err)
			os.Exit(1)
		}
		return
	}

	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "pimux:", err)
		os.Exit(1)
	}
}
