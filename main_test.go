package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReporterSourceIsEmbedded(t *testing.T) {
	if len(reporterSource) == 0 {
		t.Fatal("reporterSource is empty; go:embed did not include the reporter")
	}
	if !strings.Contains(reporterSource, "pimux reporter: report pi agent state") {
		t.Fatal("reporterSource does not look like the pimux reporter")
	}
}

func TestInstallExtensionWritesEmbeddedReporter(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "extensions")
	dest, err := installExtension(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(dir, reporterFileName)
	if dest != want {
		t.Fatalf("dest = %q, want %q", dest, want)
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != reporterSource {
		t.Fatal("written file does not match the embedded reporter source")
	}
}

func TestDefaultExtDirHonorsEnvOverride(t *testing.T) {
	t.Setenv("PIMUX_EXT_DIR", "/tmp/pimux-ext-test")
	if got := defaultExtDir(); got != "/tmp/pimux-ext-test" {
		t.Fatalf("defaultExtDir = %q, want the PIMUX_EXT_DIR override", got)
	}
}
