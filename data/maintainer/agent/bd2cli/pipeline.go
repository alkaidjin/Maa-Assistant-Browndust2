package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"bd2cli/internal/project"
)

// pipelineIndex 把 resource/pipeline 下所有节点合成一个索引（MaaFramework 是共享命名空间）。
type pipelineIndex struct {
	nodes map[string][]string // 节点名 -> next 列表
}

type rawNode struct {
	Next json.RawMessage `json:"next"`
}

// loadPipelines 读取全部 pipeline json，建立 节点 -> next 的图。
func loadPipelines(root string) *pipelineIndex {
	idx := &pipelineIndex{nodes: map[string][]string{}}
	files, _ := filepath.Glob(filepath.Join(root, "resource", "pipeline", "*.json"))
	more, _ := filepath.Glob(filepath.Join(root, "resource", "pipeline", "*", "*.json"))
	files = append(files, more...)
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		var m map[string]rawNode
		if err := json.Unmarshal(b, &m); err != nil {
			continue
		}
		for name, n := range m {
			if strings.HasPrefix(name, "$") {
				continue
			}
			idx.nodes[name] = parseNext(n.Next)
		}
	}
	return idx
}

// parseNext 解析 next 字段：字符串、字符串数组、或含 name 的对象数组。
func parseNext(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return []string{cleanNodeName(one)}
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		switch v := item.(type) {
		case string:
			out = append(out, cleanNodeName(v))
		case map[string]any:
			if n, ok := v["name"].(string); ok {
				out = append(out, cleanNodeName(n))
			}
		}
	}
	return out
}

// cleanNodeName 去掉 [JumpBack] / [Anchor] 之类前缀。
func cleanNodeName(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "[") {
		i := strings.Index(s, "]")
		if i < 0 {
			break
		}
		s = strings.TrimSpace(s[i+1:])
	}
	return s
}

// TaskEnds 为一批 entry 计算"各自的完成标志节点"：
// 先取可达的终端/End 节点，再剔除被 ≥4 个任务共用的全局兜底节点（维护公告、BackHome 之类）。
func (p *pipelineIndex) TaskEnds(entries []string) map[string][]string {
	raw := map[string][]string{}
	for _, e := range entries {
		raw[e] = p.EndNodes(e)
	}
	freq := map[string]int{}
	for _, list := range raw {
		seen := map[string]bool{}
		for _, n := range list {
			if seen[n] {
				continue
			}
			seen[n] = true
			freq[n]++
		}
	}
	out := map[string][]string{}
	for e, list := range raw {
		var keep []string
		for _, n := range list {
			if freq[n] >= 4 {
				continue // 全局兜底节点
			}
			keep = append(keep, n)
		}
		out[e] = keep
	}
	return out
}

// cmdEnds 打印每个任务的完成标志节点（供人工核对与排查）。
func cmdEnds(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	pi, err := project.Load(root)
	if err != nil {
		return fail(err, exitPreflight)
	}
	idx := loadPipelines(root)
	entries := make([]string, 0, len(pi.Task))
	for _, t := range pi.Task {
		entries = append(entries, t.Entry)
	}
	endsMap := idx.TaskEnds(entries)
	type row struct {
		Task  string   `json:"task"`
		Label string   `json:"label"`
		Entry string   `json:"entry"`
		Ends  []string `json:"ends"`
	}
	var out []row
	for _, t := range pi.Task {
		out = append(out, row{Task: t.Name, Label: t.Label, Entry: t.Entry, Ends: endsMap[t.Entry]})
	}
	return printJSON(out)
}

// EndNodes 从 entry 出发遍历，收集"终端节点"：
// 没有 next 的节点，或名字里带 end 的节点（大小写不敏感）。
func (p *pipelineIndex) EndNodes(entry string) []string {
	if entry == "" || p == nil {
		return nil
	}
	seen := map[string]bool{}
	var ends []string
	queue := []string{entry}
	visited := 0
	for len(queue) > 0 && visited < 5000 {
		cur := queue[0]
		queue = queue[1:]
		if cur == "" || seen[cur] {
			continue
		}
		seen[cur] = true
		visited++
		next, exists := p.nodes[cur]
		if !exists {
			continue
		}
		isEnd := strings.Contains(strings.ToLower(cur), "end")
		if isEnd || len(next) == 0 {
			ends = append(ends, cur)
		}
		for _, n := range next {
			if n != "" && !seen[n] {
				queue = append(queue, n)
			}
		}
	}
	return ends
}
