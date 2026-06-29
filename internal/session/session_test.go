package session

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTranscriptTailParsesMessagesAndKeepsLastN(t *testing.T) {
	path := writeJSONL(t, `{"type":"session","version":3,"cwd":"/tmp"}
{"type":"message","message":{"role":"user","content":"review the diff"}}
{"type":"message","message":{"role":"assistant","content":[{"type":"text","text":"reading files"}]}}
{"type":"message","message":{"role":"toolResult","content":"git diff --stat"}}
{"type":"message","message":{"role":"assistant","content":"done"}}
`)

	got := TranscriptTail(path, 3)
	if len(got) != 3 {
		t.Fatalf("want 3 lines, got %d: %#v", len(got), got)
	}
	if got[0].Role != "assistant" || got[0].Text != "reading files" {
		t.Fatalf("bad first tail line: %#v", got[0])
	}
	if got[1].Role != "toolResult" || got[1].Text != "git diff --stat" {
		t.Fatalf("bad tool line: %#v", got[1])
	}
	if got[2].Role != "assistant" || got[2].Text != "done" {
		t.Fatalf("bad last line: %#v", got[2])
	}
}

func TestTranscriptTailToleratesBadFiles(t *testing.T) {
	path := writeJSONL(t, "not json\n"+`{"type":"message","message":{"role":"user","content":"ok"}}`+"\n")
	got := TranscriptTail(path, 10)
	if len(got) != 1 || got[0].Role != "user" || got[0].Text != "ok" {
		t.Fatalf("want one parseable line, got %#v", got)
	}
	if got := TranscriptTail(filepath.Join(t.TempDir(), "missing.jsonl"), 10); len(got) != 0 {
		t.Fatalf("missing file should return empty, got %#v", got)
	}
}

func writeJSONL(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
