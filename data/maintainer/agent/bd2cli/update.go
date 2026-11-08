package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"bd2cli/internal/logx"
	"bd2cli/internal/project"
)

const repoAPI = "https://api.github.com/repos/alkaidjin/Maa-Assistant-Browndust2/releases/latest"

// releaseInfo 是从 GitHub API 拿到的"最新正式版"（只取展示用信息）。
type releaseInfo struct {
	Tag   string `json:"tag"`
	Name  string `json:"name"`
	Asset string `json:"asset"`
	Size  int64  `json:"size"`
	Body  string `json:"body"`
}

// updateState 是给 AI 看的更新状态（含 MXU 自己检测到的结果）。
type updateState struct {
	Current    string `json:"current"`
	Latest     string `json:"latest"`
	Newer      bool   `json:"newer"`
	Asset      string `json:"asset,omitempty"`
	Size       int64  `json:"size,omitempty"`
	MxuSaid    string `json:"mxu_update_line,omitempty"`
	MirrorRID  string `json:"mirrorchyan_rid,omitempty"`
	CachedAt   string `json:"cached_at,omitempty"`
	CheckError string `json:"check_error,omitempty"`
	HowTo      string `json:"how_to_update"`
}

const updateGuidance = "请让用户在 MXU 界面里更新（设置 → 更新）。" +
	"AI 不要代为下载，也不要提供其他下载渠道；用户主动要求解决下载问题时，先建议 Mirror酱，再建议开启自己的代理。"

func updateCachePath(root string) string { return filepath.Join(root, "logs", "update-check.json") }

// localVersion 读 interface.json 的 version（如 v26.11.1）。
func localVersion(root string) string {
	if pi, err := project.Load(root); err == nil {
		return pi.Version
	}
	return ""
}

// mxuUpdateLine 从 MXU 自身日志里取它"检查更新"的结论——这是 MXU 检测到新版本的直接证据。
func mxuUpdateLine(root string) string {
	files, _ := filepath.Glob(filepath.Join(root, "debug", "2026-*.log"))
	best := ""
	var bestTime time.Time
	for _, f := range files {
		lines, err := logx.Lines(f, 512<<10)
		if err != nil {
			continue
		}
		for _, ln := range lines {
			if !strings.Contains(ln, "更新检查完成") {
				continue
			}
			if ts, ok := mafwTime(ln); ok && ts.After(bestTime) {
				bestTime = ts
				best = ln
			} else if best == "" {
				best = ln
			}
		}
	}
	return strings.TrimSpace(best)
}

// fetchLatest 请求 GitHub API 拿最新正式版信息（只读，不下载）。
func fetchLatest(repo string) (*releaseInfo, error) {
	req, _ := http.NewRequest("GET", repo, nil)
	req.Header.Set("User-Agent", "bd2cli")
	req.Header.Set("Accept", "application/vnd.github+json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API HTTP %d", resp.StatusCode)
	}
	var raw struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		Assets  []struct {
			Name  string `json:"name"`
			Size  int64  `json:"size"`
			State string `json:"state"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, err
	}
	ri := &releaseInfo{Tag: raw.TagName, Name: raw.Name, Body: raw.Body}
	for _, a := range raw.Assets {
		if strings.HasSuffix(a.Name, "-win-x64.zip") && a.State == "uploaded" {
			ri.Asset, ri.Size = a.Name, a.Size
			break
		}
	}
	return ri, nil
}

// cmdUpdate 只做"检查 + 提示"，不下载。
//
//	bd2cli update check   查最新版并与本地比较（同时带上 MXU 的判断）
//	bd2cli update state   读上次检查缓存（不联网）
func cmdUpdate(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	sub := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			sub = a
			break
		}
	}
	switch sub {
	case "", "state":
		b, err := os.ReadFile(updateCachePath(root))
		if err != nil {
			fmt.Println("还没有检查过更新；执行 bd2cli update check")
			return exitOK
		}
		var st updateState
		if json.Unmarshal(b, &st) != nil {
			return fail(fmt.Errorf("缓存损坏"), exitFailed)
		}
		return printJSON(st)

	case "check":
		st := updateState{Current: localVersion(root), MxuSaid: mxuUpdateLine(root), HowTo: updateGuidance}
		if pi, err := project.Load(root); err == nil {
			st.MirrorRID = pi.MirrorChyanRID
		}
		ri, err := fetchLatest(repoAPI)
		if err != nil {
			st.CheckError = err.Error()
			_ = saveUpdateCache(root, st)
			fmt.Printf("检查失败：%v\n", err)
			if st.MxuSaid != "" {
				fmt.Printf("MXU 自己的检查结果：%s\n", st.MxuSaid)
			}
			fmt.Println(updateGuidance)
			return exitFailed
		}
		st.Latest, st.Asset, st.Size = ri.Tag, ri.Asset, ri.Size
		st.Newer = versionLess(st.Current, st.Latest)
		st.CachedAt = time.Now().Format(time.RFC3339)
		_ = saveUpdateCache(root, st)

		fmt.Printf("本地版本：%s\n最新版本：%s（%s）\n", st.Current, st.Latest, st.Asset)
		if st.MxuSaid != "" {
			fmt.Printf("MXU 检测：%s\n", st.MxuSaid)
		}
		if st.Newer {
			fmt.Printf("→ 有新版本。%s\n", updateGuidance)
		} else {
			fmt.Printf("→ 已是最新\n")
		}
		return exitOK

	case "fetch", "download":
		// 刻意不支持：更新统一交给 MXU（含 Mirror酱 / GitHub 的既有策略）
		return fail(fmt.Errorf("本工具不提供下载：%s", updateGuidance), exitPermission)
	}
	return fail(fmt.Errorf("用法: bd2cli update check|state"), exitUsage)
}

func saveUpdateCache(root string, st updateState) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Join(root, "logs"), 0o755)
	return os.WriteFile(updateCachePath(root), b, 0o644)
}

// versionLess 比较 v26.11.1 形式；a<b 返回 true。
func versionLess(a, b string) bool {
	pa, pb := parseVer(a), parseVer(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func parseVer(s string) [3]int {
	var out [3]int
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	m := re.FindStringSubmatch(s)
	if len(m) == 4 {
		fmt.Sscanf(strings.Join(m[1:], " "), "%d %d %d", &out[0], &out[1], &out[2])
	}
	return out
}

// bdConfig 是 config/bd2cli.json 里与本工具相关的配置。
type bdConfig struct {
	AutomationName string              `json:"automation_name"`
	TaskEnds       map[string][]string `json:"task_ends"`
	NoRetry        []string            `json:"no_retry_tasks"`
}

func loadBDConfig(root string) bdConfig {
	var c bdConfig
	b, err := os.ReadFile(filepath.Join(root, "config", "bd2cli.json"))
	if err != nil {
		return c
	}
	_ = json.Unmarshal(b, &c)
	return c
}
