// Package session reads pi session JSONL files for the pimux preview pane.
package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
)

const maxReadBytes = 64 * 1024

// Line is one previewable transcript line.
type Line struct {
	Role string
	Text string
}

// TranscriptTail returns the last max transcript messages from a pi session
// JSONL file. It is best-effort and read-only: malformed lines, unsupported
// shapes, and missing files produce no error and are skipped.
func TranscriptTail(path string, max int) []Line {
	if max <= 0 || path == "" {
		return nil
	}
	body, err := readTail(path, maxReadBytes)
	if err != nil {
		return nil
	}
	var lines []Line
	s := bufio.NewScanner(bytes.NewReader(body))
	for s.Scan() {
		if line, ok := parseLine(s.Bytes()); ok {
			lines = append(lines, line)
		}
	}
	if len(lines) > max {
		lines = lines[len(lines)-max:]
	}
	return lines
}

func readTail(path string, limit int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	start := info.Size() - limit
	if start < 0 {
		start = 0
	}
	if _, err := f.Seek(start, 0); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(f)
	if err != nil || start == 0 {
		return body, err
	}
	if idx := bytes.IndexByte(body, '\n'); idx >= 0 {
		body = body[idx+1:]
	}
	return body, nil
}

func parseLine(raw []byte) (Line, bool) {
	var entry struct {
		Type    string `json:"type"`
		Message struct {
			Role    string      `json:"role"`
			Content interface{} `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &entry); err != nil || entry.Type != "message" || entry.Message.Role == "" {
		return Line{}, false
	}
	text := strings.TrimSpace(contentText(entry.Message.Content))
	if text == "" {
		return Line{}, false
	}
	return Line{Role: entry.Message.Role, Text: compact(text)}, true
}

func contentText(v interface{}) string {
	switch c := v.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, item := range c {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if text, ok := m["text"].(string); ok {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}

func compact(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
