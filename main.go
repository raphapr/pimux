// Command pimux is a tmux-native dashboard for pi agents: it shows every pi
// agent across your tmux sessions (working / blocked / done / idle) and lets
// you jump to one. State is reported by the pimux-reporter pi extension into
// tmux pane options; nothing here scrapes terminal output.
package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/raphapr/pimux/internal/agent"
	"github.com/raphapr/pimux/internal/tmux"
	"github.com/raphapr/pimux/internal/ui"
)

var version = "0.1.0"

const reporterFileName = "pimux-reporter.ts"

// reporterSource is the pimux reporter pi extension, embedded so the installed
// binary can write it without the source tree present (e.g. after go install).
//
//go:embed extension/pimux-reporter.ts
var reporterSource string

// defaultExtDir is where pi loads global extensions. PIMUX_EXT_DIR overrides it
// (useful for rebranded pi config dirs or tests).
func defaultExtDir() string {
	if d := os.Getenv("PIMUX_EXT_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".pi", "agent", "extensions")
	}
	return filepath.Join(home, ".pi", "agent", "extensions")
}

// installExtension writes the embedded reporter into dir, creating it if needed,
// and returns the destination path.
func installExtension(dir string) (string, error) {
	if dir == "" {
		dir = defaultExtDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, reporterFileName)
	if err := os.WriteFile(dest, []byte(reporterSource), 0o644); err != nil {
		return "", err
	}
	return dest, nil
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install-extension" {
		fs := flag.NewFlagSet("install-extension", flag.ExitOnError)
		dir := fs.String("dir", "", "target extensions dir (default $PIMUX_EXT_DIR or ~/.pi/agent/extensions)")
		_ = fs.Parse(os.Args[2:])
		dest, err := installExtension(*dir)
		if err != nil {
			fmt.Fprintln(os.Stderr, "pimux:", err)
			os.Exit(1)
		}
		fmt.Println("installed", dest)
		fmt.Println("restart pi sessions to load the reporter")
		return
	}

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
