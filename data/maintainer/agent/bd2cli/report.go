package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bd2cli/internal/project"
	"bd2cli/internal/runs"
)

const reportCSS = `<style>
:root{--fg:#111827;--muted:#6b7280;--line:#e5e7eb;--ok:#16a34a;--bad:#dc2626;--warn:#d97706}
@media (prefers-color-scheme:dark){:root{--fg:#e5e7eb;--muted:#9ca3af;--line:#374151}}
:root[data-theme='dark']{--fg:#e5e7eb;--muted:#9ca3af;--line:#374151}
:root[data-theme='light']{--fg:#111827;--muted:#6b7280;--line:#e5e7eb}
*{box-sizing:border-box}
html,body{margin:0;background:transparent}
body{color:var(--fg);font:14px/1.6 -apple-system,"Segoe UI","Microsoft YaHei",sans-serif}
.wrap{max-width:640px;margin:0 auto;padding:14px 16px}
.title{font-size:17px;font-weight:700;margin:0 0 10px}
.items{list-style:none;margin:0;padding:0}
.items li{display:flex;align-items:flex-start;gap:8px;padding:4px 0;font-size:14px}
.dot{flex:0 0 18px;width:18px;height:18px;border-radius:50%;display:inline-flex;align-items:center;justify-content:center;font-size:11px;color:#fff;margin-top:2px}
.ok{background:var(--ok)}.bad{background:var(--bad)}.warn{background:var(--warn)}
.dim{color:var(--muted)}
.foot{margin-top:12px;padding-top:10px;border-top:1px solid var(--line);color:var(--muted);font-size:12px}
table.rows{width:100%;border-collapse:collapse;font-size:13px}
table.rows td{padding:6px 4px;border-bottom:1px dashed var(--line)}
</style>`

// cmdReport 出**最近一轮**的简洁结果卡片（不列节点流程）。
func cmdReport(args []string) int {
	root, err := rootFrom(args)
	if err != nil {
		return fail(err, exitPreflight)
	}
	embed := hasFlag(args, "--embed")
	history := hasFlag(args, "--history")
	id := valueOf(args, "--id")

	store := runs.New(filepath.Join(root, "logs", "runs"))
	recs, err := store.Recent(50)
	if err != nil {
		return fail(err, exitFailed)
	}
	if len(recs) == 0 {
		return fail(fmt.Errorf("还没有运行记录（先用 bd2cli run <任务>）"), exitFailed)
	}

	if history {
		out := filepath.Join(root, "logs", "report-history-"+time.Now().Format("20060102-150405")+".html")
		if err := os.WriteFile(out, []byte(buildHistoryHTML(recs, embed)), 0o644); err != nil {
			return fail(err, exitFailed)
		}
		fmt.Printf("历史记录: %s（%d 轮）\n", out, len(recs))
		return exitOK
	}

	var rec *runs.Record
	if id != "" {
		if r, err := store.Get(id); err == nil {
			rec = r
		}
	}
	if rec == nil {
		rec = recs[0]
	}

	label := taskLabel(root, rec.Task)
	out := filepath.Join(root, "logs", "report-"+rec.ID+".html")
	if err := os.WriteFile(out, []byte(buildCard(label, rec, embed)), 0o644); err != nil {
		return fail(err, exitFailed)
	}

	fmt.Printf("最近一轮：%s %s，耗时 %s\n", label, statusLabel(rec.Status), humanDuration(rec.DurationS))
	if rec.Reason != "" {
		fmt.Printf("原因：%s\n", rec.Reason)
	}
	fmt.Println("看板: " + out)
	return printJSON(map[string]any{
		"html":    out,
		"id":      rec.ID,
		"task":    label,
		"status":  rec.Status,
		"seconds": int(rec.DurationS),
		"diagnose": rec.Diagnose,
	})
}

// emitReport 在 run 结束后自动生成这一轮的结果卡片。
func emitReport(root string, rec *runs.Record, _ []string) (string, error) {
	label := taskLabel(root, rec.Task)
	out := filepath.Join(root, "logs", "report-"+rec.ID+".html")
	if err := os.WriteFile(out, []byte(buildCard(label, rec, false)), 0o644); err != nil {
		return "", err
	}
	return out, nil
}

// automationName 是一次完整日常组合的名字（区别于单个任务名）。
// 可在 config/bd2cli.json 的 automation_name 里改，默认「棕色尘埃2日常」。
func automationName(root string) string {
	b, err := os.ReadFile(filepath.Join(root, "config", "bd2cli.json"))
	if err == nil {
		var w struct {
			AutomationName string `json:"automation_name"`
		}
		if json.Unmarshal(b, &w) == nil && w.AutomationName != "" {
			return w.AutomationName
		}
	}
	return "棕色尘埃2日常"
}

func taskLabel(root, name string) string {
	if name == "" {
		return "（未命名任务）"
	}
	if pi, err := project.Load(root); err == nil {
		if t, ok := pi.TaskByName(name); ok && t.Label != "" {
			return t.Label
		}
	}
	return name
}

func humanDuration(sec float64) string {
	s := int(sec + 0.5)
	if s < 60 {
		return fmt.Sprintf("%d 秒", s)
	}
	return fmt.Sprintf("%d 分 %d 秒", s/60, s%60)
}

func statusColor(s string) string {
	switch s {
	case runs.StatusSuccess:
		return "#16a34a"
	case runs.StatusFailed:
		return "#dc2626"
	case runs.StatusTimeout:
		return "#d97706"
	case runs.StatusAborted:
		return "#6b7280"
	default:
		return "#2563eb"
	}
}

func statusLabel(s string) string {
	switch s {
	case runs.StatusSuccess:
		return "成功"
	case runs.StatusFailed:
		return "失败"
	case runs.StatusTimeout:
		return "超时"
	case runs.StatusAborted:
		return "中止"
	default:
		return s
	}
}

// lastNode 从失败原因里取"最后节点：A → B → C"的最后一个。
func lastNode(reason string) string {
	const marker = "最后节点："
	i := strings.Index(reason, marker)
	if i < 0 {
		return ""
	}
	tail := reason[i+len(marker):]
	if j := strings.Index(tail, "；"); j >= 0 {
		tail = tail[:j]
	}
	parts := strings.Split(tail, "→")
	if len(parts) == 0 {
		return strings.TrimSpace(tail)
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

// buildCard 生成用户看的简洁卡片：标题 + 3~5 行结论 + 一行页脚，不列节点流程。
func buildCard(label string, rec *runs.Record, embed bool) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>任务结果</title>`)
	b.WriteString(reportCSS)
	b.WriteString(`</head><body><div class="wrap"`)
	if embed {
		b.WriteString(` style="max-width:none;padding:2px 4px 8px"`)
	}
	b.WriteString(`>`)

	ok := rec.Status == runs.StatusSuccess
	if ok {
		fmt.Fprintf(&b, `<div class="title">搞定啦！%s 跑完了 ✨</div>`, html.EscapeString(label))
	} else {
		fmt.Fprintf(&b, `<div class="title">出错了：%s %s</div>`, html.EscapeString(label), statusLabel(rec.Status))
	}

	b.WriteString(`<ul class="items">`)
	row := func(cls, text string) {
		fmt.Fprintf(&b, `<li><span class="dot %s">%s</span><span>%s</span></li>`,
			cls, map[string]string{"ok": "✓", "bad": "✕", "warn": "!"}[cls], text)
	}
	row(pick(ok, "ok", "bad"), "任务："+html.EscapeString(label))
	row("ok", "用时："+humanDuration(rec.DurationS))
	if rec.FinishedAt != nil {
		row("ok", "结束时间："+rec.FinishedAt.Format("15:04"))
	}
	if len(rec.Tasks) > 0 {
		done := 0
		for _, t := range rec.Tasks {
			if t.OK {
				done++
			}
		}
		if ok && done == len(rec.Tasks) {
			row("ok", fmt.Sprintf("全部任务完成：%d/%d", done, len(rec.Tasks)))
		} else {
			row("bad", fmt.Sprintf("任务完成：%d/%d", done, len(rec.Tasks)))
		}
	}
	if !ok {
		if n := lastNode(rec.Reason); n != "" {
			row("bad", "卡在节点："+html.EscapeString(n))
		}
		reason := rec.Reason
		if len([]rune(reason)) > 90 {
			reason = string([]rune(reason)[:90]) + "…"
		}
		if reason != "" {
			row("bad", "原因："+html.EscapeString(reason))
		}
		if rec.Diagnose != "" {
			row("warn", "已打包诊断：<code>"+html.EscapeString(filepath.Base(rec.Diagnose))+"</code>")
		}
	}
	// 逐个任务列结果（用户最关心的就是"哪个成功哪个失败"）
	for _, t := range rec.Tasks {
		row(pick(t.OK, "ok", "bad"), html.EscapeString(t.Name)+pick(t.OK, " · 成功", " · 失败"))
	}
	b.WriteString(`</ul>`)

	fmt.Fprintf(&b, `<div class="foot">用时 %s · 退出码 %d · logs\runs\%s.json</div>`,
		humanDuration(rec.DurationS), rec.ExitCode, html.EscapeString(rec.ID))
	b.WriteString(`</div></body></html>`)
	return b.String()
}

func pick(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func buildHistoryHTML(recs []*runs.Record, embed bool) string {
	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>历史记录</title>`)
	b.WriteString(reportCSS)
	b.WriteString(`</head><body><div class="wrap" style="max-width:none;padding:4px"><table class="rows">`)
	for _, r := range recs {
		fmt.Fprintf(&b, `<tr><td style="white-space:nowrap;color:var(--muted)">%s</td><td>%s</td><td><span class="dot %s" style="display:inline-flex">%s</span></td><td style="text-align:right;white-space:nowrap">%s</td></tr>`,
			r.StartedAt.Format("01-02 15:04"), html.EscapeString(r.Task),
			pick(r.Status == runs.StatusSuccess, "ok", "bad"),
			pick(r.Status == runs.StatusSuccess, "✓", "✕"),
			humanDuration(r.DurationS))
	}
	b.WriteString(`</table></div></body></html>`)
	return b.String()
}
