package mcp

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type LogEntry struct {
	Name     string `json:"name"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	Mtime    string `json:"mtime"`
}

func ListLogs(logDir string) ([]LogEntry, error) {
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return nil, err
	}
	out := []LogEntry{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".log") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, LogEntry{
			Name:     strings.TrimSuffix(e.Name(), ".log"),
			Filename: e.Name(),
			Size:     info.Size(),
			Mtime:    info.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Mtime > out[j].Mtime })
	return out, nil
}

func resolveLogPath(logDir, name string) string {
	fn := name
	if !strings.HasSuffix(fn, ".log") {
		fn += ".log"
	}
	return filepath.Join(logDir, fn)
}

type TailResult struct {
	Path    string
	Bytes   int64
	Content string
}

func TailLog(logDir, name string, lines int) (*TailResult, error) {
	path := resolveLogPath(logDir, name)
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	size := info.Size()
	const chunkSize = 64 * 1024
	pos := size
	buf := []byte{}
	count := 0
	for pos > 0 && count <= lines {
		readSize := int64(chunkSize)
		if pos < readSize {
			readSize = pos
		}
		pos -= readSize
		chunk := make([]byte, readSize)
		if _, err := f.ReadAt(chunk, pos); err != nil {
			return nil, err
		}
		buf = append(chunk, buf...)
		count = 0
		for _, b := range buf {
			if b == '\n' {
				count++
			}
		}
	}
	parts := strings.Split(string(buf), "\n")
	start := 0
	if len(parts) > lines+1 {
		start = len(parts) - lines - 1
	}
	return &TailResult{
		Path:    path,
		Bytes:   size,
		Content: strings.Join(parts[start:], "\n"),
	}, nil
}

type TruncateResult struct {
	Path       string
	BytesFreed int64
}

func TruncateLog(logDir, name string) (*TruncateResult, error) {
	path := resolveLogPath(logDir, name)
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if err := os.Truncate(path, 0); err != nil {
		return nil, err
	}
	return &TruncateResult{Path: path, BytesFreed: info.Size()}, nil
}

type SearchOpts struct {
	MaxMatches int
	IgnoreCase bool
	Context    int
}

type MatchLine struct {
	LineNo int
	Line   string
}

type Match struct {
	LineNo int
	Line   string
	Before []MatchLine
	After  []MatchLine
}

type SearchResult struct {
	Path         string
	TotalMatches int
	Shown        int
	Matches      []Match
}

func SearchLog(logDir, name, pattern string, opts SearchOpts) (*SearchResult, error) {
	path := resolveLogPath(logDir, name)
	flags := ""
	if opts.IgnoreCase {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	res := &SearchResult{Path: path}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	before := []MatchLine{}
	lineNo := 0
	pendingAfter := 0
	var current *Match
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		if pendingAfter > 0 && current != nil {
			current.After = append(current.After, MatchLine{LineNo: lineNo, Line: line})
			pendingAfter--
			if pendingAfter == 0 {
				current = nil
			}
		}
		if re.MatchString(line) {
			res.TotalMatches++
			if len(res.Matches) < opts.MaxMatches {
				beforeCopy := make([]MatchLine, len(before))
				copy(beforeCopy, before)
				m := Match{LineNo: lineNo, Line: line, Before: beforeCopy}
				res.Matches = append(res.Matches, m)
				current = &res.Matches[len(res.Matches)-1]
				pendingAfter = opts.Context
			}
		}
		before = append(before, MatchLine{LineNo: lineNo, Line: line})
		if len(before) > opts.Context {
			before = before[1:]
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return nil, err
	}
	res.Shown = len(res.Matches)
	return res, nil
}

func FormatSearchResult(r *SearchResult, pattern string, opts SearchOpts) string {
	caseFlag := ""
	if opts.IgnoreCase {
		caseFlag = "i"
	}
	header := fmt.Sprintf("# %s\n# pattern: /%s/%s  matches: %d", r.Path, pattern, caseFlag, r.TotalMatches)
	if r.TotalMatches > r.Shown {
		header += fmt.Sprintf(" (showing first %d)", r.Shown)
	}
	header += "\n"
	if len(r.Matches) == 0 {
		return header + "\n(no matches)"
	}
	blocks := make([]string, 0, len(r.Matches))
	for _, m := range r.Matches {
		lines := []string{}
		if opts.Context > 0 {
			for _, b := range m.Before {
				lines = append(lines, fmt.Sprintf("  %d: %s", b.LineNo, b.Line))
			}
		}
		lines = append(lines, fmt.Sprintf("> %d: %s", m.LineNo, m.Line))
		if opts.Context > 0 {
			for _, a := range m.After {
				lines = append(lines, fmt.Sprintf("  %d: %s", a.LineNo, a.Line))
			}
		}
		blocks = append(blocks, strings.Join(lines, "\n"))
	}
	return header + "\n" + strings.Join(blocks, "\n---\n")
}
