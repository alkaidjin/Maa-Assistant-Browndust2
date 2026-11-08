package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"bd2cli/internal/logx"
	"bd2cli/internal/mxucfg"
	"bd2cli/internal/runs"
)

func osName() string { return runtime.GOOS }
func osArch() string { return runtime.GOARCH }

// preserveEvidence 在重试（会重启 MXU，从而清空 debug/）之前，
// 把本次失败的截图与日志尾部复制到 logs/evidence/<runID>/ 保留下来。
func preserveEvidence(root, runID string) error {
	dst := filepath.Join(root, "logs", "evidence", runID)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	shots, _ := filepath.Glob(filepath.Join(root, "debug", "on_error", "*.png"))
	for _, s := range shots {
		b, err := os.ReadFile(s)
		if err != nil {
			continue
		}
		_ = os.WriteFile(filepath.Join(dst, filepath.Base(s)), b, 0o644)
	}
	if lines, err := logx.Lines(filepath.Join(root, "debug", "maafw.log"), 1<<20); err == nil {
		_ = os.WriteFile(filepath.Join(dst, "maafw.tail.log"), []byte(strings.Join(lines, "\n")+"\n"), 0o644)
	}
	return nil
}

// cmdDiagnose 手动打包某次失败的材料（默认取最近一次失败的运行）。
func cmdDiagnose(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	id := valueOf(args, "--id")
	store := runs.New(filepath.Join(root, "logs", "runs"))
	var rec *runs.Record
	if id != "" {
		if r, err := store.Get(id); err == nil {
			rec = r
		}
	}
	if rec == nil {
		recs, _ := store.Recent(50)
		for _, r := range recs {
			if r.Status != runs.StatusSuccess {
				rec = r
				break
			}
		}
	}
	if rec == nil {
		return fail(fmt.Errorf("没有失败记录可打包"), exitFailed)
	}
	p, err := collectDiagnosis(root, rec, rec.Task, rec.Status, rec.Reason)
	if err != nil {
		return fail(err, exitFailed)
	}
	rec.Diagnose = p
	_ = store.Save(rec)
	fmt.Printf("诊断包: %s\n", p)
	return printJSON(map[string]any{"zip": p, "id": rec.ID, "task": rec.Task, "status": rec.Status})
}

// collectDiagnosis 把定位一次失败所需的材料打包成一个 zip（用户直接发给开发者）。
//
// 内容：运行记录、失败截图、maafw.log 尾部、go-service 日志尾部、脱敏后的配置、环境状态。
func collectDiagnosis(root string, rec *runs.Record, task, state, reason string) (string, error) {
	out := filepath.Join(root, "logs", "diagnose-"+rec.ID+".zip")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	addBytes := func(name string, b []byte) {
		w, err := zw.Create(name)
		if err != nil {
			return
		}
		_, _ = w.Write(b)
	}
	addFile := func(name, path string, maxBytes int64) {
		st, err := os.Stat(path)
		if err != nil || st.IsDir() {
			return
		}
		if maxBytes > 0 && st.Size() > maxBytes {
			lines, err := logx.Lines(path, maxBytes)
			if err != nil {
				return
			}
			addBytes(name, []byte(strings.Join(lines, "\n")+"\n"))
			return
		}
		in, err := os.Open(path)
		if err != nil {
			return
		}
		defer in.Close()
		w, err := zw.Create(name)
		if err != nil {
			return
		}
		_, _ = io.Copy(w, in)
	}

	// 1) 摘要
	summary := fmt.Sprintf("运行编号: %s\n任务: %s\n状态: %s\n原因: %s\n开始: %s\n耗时: %.1fs\n退出码: %d\n",
		rec.ID, task, state, reason, rec.StartedAt.Format(time.RFC3339), rec.DurationS, rec.ExitCode)
	addBytes("summary.txt", []byte(summary))

	// 2) 运行记录
	addFile("run-record.json", filepath.Join(root, "logs", "runs", rec.ID+".json"), 0)

	// 3) 失败截图：优先取已固化的证据目录，其次取 debug/on_error 最近 3 张
	evid := filepath.Join(root, "logs", "evidence", rec.ID)
	if st, err := os.Stat(evid); err == nil && st.IsDir() {
		entries, _ := os.ReadDir(evid)
		for _, e := range entries {
			addFile("evidence/"+e.Name(), filepath.Join(evid, e.Name()), 0)
		}
	}
	shots, _ := filepath.Glob(filepath.Join(root, "debug", "on_error", "*.png"))
	sort.Sort(sort.Reverse(sort.StringSlice(shots)))
	for i, s := range shots {
		if i >= 3 {
			break
		}
		addFile("screenshots/"+filepath.Base(s), s, 0)
	}

	// 4) 日志尾部
	addFile("logs/maafw.log.head-tail.txt", filepath.Join(root, "debug", "maafw.log"), 2<<20)
	addFile("logs/go-service.log.tail.txt", filepath.Join(root, "debug", "go-service.log"), 256<<10)
	dated, _ := filepath.Glob(filepath.Join(root, "debug", "2026-*.log"))
	sort.Sort(sort.Reverse(sort.StringSlice(dated)))
	if len(dated) > 0 {
		addFile("logs/mxu-"+filepath.Base(dated[0]), dated[0], 512<<10)
	}

	// 5) 脱敏后的 MXU 配置（去掉 cdk / token / password）
	if cfgPath, err := mxucfg.Find(root); err == nil {
		if b, err := os.ReadFile(cfgPath); err == nil {
			addBytes("config/mxu-redacted.json", redactJSON(b))
		}
	}

	// 6) 环境状态
	addBytes("env.json", []byte(envSnapshot(root)))
	return out, nil
}

// redactJSON 把敏感键的值替换成 "***"（结构化处理，避免字符串手术出错）。
func redactJSON(b []byte) []byte {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return []byte(`{"error":"配置解析失败，已省略"}`)
	}
	mask(v)
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte(`{"error":"配置序列化失败，已省略"}`)
	}
	return out
}

func mask(v any) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "cdk") || strings.Contains(lk, "token") ||
				strings.Contains(lk, "password") || strings.Contains(lk, "secret") {
				t[k] = "***"
				continue
			}
			mask(val)
		}
	case []any:
		for _, item := range t {
			mask(item)
		}
	}
}

// envSnapshot 收集最小环境信息，便于开发者判断是否环境问题。
func envSnapshot(root string) string {
	wd, _ := os.Getwd()
	return fmt.Sprintf(`{
  "cwd": %q,
  "root": %q,
  "os": %q,
  "arch": %q,
  "time": %q,
  "cli": %q
}`, wd, root, osName(), osArch(), time.Now().Format(time.RFC3339), cliVersion)
}
