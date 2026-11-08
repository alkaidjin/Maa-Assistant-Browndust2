// Package logx 读取 MaaFramework 日志尾部，给 AI 提供"最后报错 + 最后几行"。
package logx

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

// Tail 是日志尾部摘要。
type Tail struct {
	Path       string   `json:"path"`
	Bytes      int64    `json:"bytes"`
	LastLines  []string `json:"last_lines"`
	LastError  string   `json:"last_error,omitempty"`
	LastUpdate string   `json:"last_update,omitempty"`
}

// Lines 读 path 尾部 window 字节，返回全部非空行（丢弃被截断的首行）。
func Lines(path string, window int64) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if window <= 0 {
		window = 4 << 20
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	offset := info.Size() - window
	if offset < 0 {
		offset = 0
	}
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return nil, err
	}
	if offset > 0 {
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}
	raw := strings.Split(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, ln := range raw {
		if strings.TrimSpace(ln) != "" {
			out = append(out, strings.TrimRight(ln, "\r"))
		}
	}
	return out, nil
}

// Read 读取 path 最后 window 字节，保留最后 lines 条非空行，并找最后一条 ERR。
func Read(path string, lines int, window int64) (*Tail, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	t := &Tail{Path: path, Bytes: info.Size(), LastUpdate: info.ModTime().Format("2006-01-02T15:04:05-07:00")}
	if lines <= 0 {
		lines = 20
	}
	if window <= 0 {
		window = 256 << 10
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	offset := info.Size() - window
	if offset < 0 {
		offset = 0
	}
	buf := make([]byte, info.Size()-offset)
	if _, err := f.ReadAt(buf, offset); err != nil {
		return nil, err
	}
	if offset > 0 {
		// 丢弃首条被截断的半行。
		if i := bytes.IndexByte(buf, '\n'); i >= 0 {
			buf = buf[i+1:]
		}
	}
	all := strings.Split(strings.ReplaceAll(string(buf), "\r\n", "\n"), "\n")
	nonEmpty := make([]string, 0, len(all))
	for _, ln := range all {
		if strings.TrimSpace(ln) != "" {
			nonEmpty = append(nonEmpty, strings.TrimRight(ln, "\r"))
			if strings.Contains(ln, "[ERR]") || strings.Contains(ln, "[Error]") {
				t.LastError = strings.TrimSpace(ln)
			}
		}
	}
	if len(nonEmpty) > lines {
		nonEmpty = nonEmpty[len(nonEmpty)-lines:]
	}
	t.LastLines = nonEmpty
	return t, nil
}
