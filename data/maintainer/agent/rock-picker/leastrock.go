package main

import (
	"encoding/json"
	"fmt"
	"image"
	"strings"
	"sync"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// LeastRockPicker reads the five 圣石 counts from the resource bar and points
// the pipeline at the cave whose count is lowest.
type LeastRockPicker struct{}

var _ maa.CustomRecognitionRunner = &LeastRockPicker{}

// Run implements maa.CustomRecognitionRunner.
//
// On success it rewrites this node's `next` to the single chosen cave node and
// reports the five readings in the MXU run log.
//
// On failure it reports a loud, user-visible error and answers "no hit". The
// caller (GetRock) then has no candidate left in `next`, so its recognition loop
// times out and — because GetRock declares no `on_error` — MaaFramework marks the
// task failed (red cross), see PipelineTask::run. Note that an *empty* next list
// would NOT work: the framework exits the loop and reports success. Silently
// hunting the wrong cave would waste the player's limited daily hunts, so a hard
// failure is the intended behaviour.
func (r *LeastRockPicker) Run(ctx *maa.Context, arg *maa.CustomRecognitionArg) (*maa.CustomRecognitionResult, bool) {
	if ctx == nil || arg == nil {
		return nil, false
	}

	node := strings.TrimSpace(arg.CurrentTaskName)
	if node == "" {
		// Without a node name we cannot rewire `next`; refusing to hit is the
		// only safe answer (the parent will time out and fail the task).
		return nil, false
	}

	param, err := parseLeastRockParam(arg.CustomRecognitionParam)
	if err != nil {
		return fail(ctx, arg, fmt.Sprintf("圣石数量选择：参数无效（%v），已中止当前任务", err))
	}

	// One settle wait before the first read: the resource bar animates in when
	// 圣石洞穴 opens, and reading too early yields garbage.
	settle(param.SettleMS)

	var (
		lastErr    error
		lastCounts []int
		lastRaw    []string
	)
	for attempt := 1; attempt <= param.Attempts; attempt++ {
		counts, raw, err := readRows(ctx, arg, param.Rows)
		if err == nil {
			return choose(ctx, node, arg, param.Rows, counts)
		}
		lastErr, lastCounts, lastRaw = err, counts, raw
		if attempt < param.Attempts {
			settle(param.IntervalMS)
		}
	}

	// The framework will call us again (and the parent will keep polling) until
	// its `timeout` expires, so only the first failure is announced.
	return fail(ctx, arg, fmt.Sprintf("圣石数量选择：尝试 %d 次后仍无法可靠读取全部 %d 行数量。\n%s\n原因：%v\n本次已跳过刷取，请确认游戏窗口完整可见、分辨率比例为 16:9（未遮挡、未最小化）；若持续失败，请改用「固定刷某种圣石」。",
		param.Attempts, len(param.Rows), describeFailure(param.Rows, lastCounts, lastRaw), lastErr))
}

// choose picks the smallest count and rewires this node's `next` to that cave.
func choose(ctx *maa.Context, node string, arg *maa.CustomRecognitionArg, rows []rockRow, counts []int) (*maa.CustomRecognitionResult, bool) {
	idx, err := pickLeast(counts)
	if err != nil {
		return fail(ctx, arg, fmt.Sprintf("圣石数量选择：比较失败（%v），已中止当前任务", err))
	}

	labels := labelsOf(rows)
	msg := fmt.Sprintf("圣石数量：%s → 刷%s之石（当前最少）", formatCounts(labels, counts), labels[idx])

	if err := ctx.OverrideNext(node, []maa.NextItem{{Name: rows[idx].Node}}); err != nil {
		return fail(ctx, arg, fmt.Sprintf("圣石数量选择：改写流程失败（%v），已中止当前任务", err))
	}

	// Only now is the plan settled, so this is the line the user should see.
	focus(ctx, msg, logDisplay)

	// Report the chosen row's box so MXU can highlight the number we selected.
	return &maa.CustomRecognitionResult{Box: rectOf(rows[idx].Roi), Detail: msg}, true
}

// fail tells the user what went wrong and answers "no hit", which lets the
// parent node's recognition loop time out and fail the task.
//
// The toast/notification is emitted only once per task: the parent keeps
// re-polling us until its `timeout` expires, and repeating the same dialog would
// be noise. The log line is repeated on purpose — it shows the framework really
// did retry.
func fail(ctx *maa.Context, arg *maa.CustomRecognitionArg, msg string) (*maa.CustomRecognitionResult, bool) {
	if firstFailure(arg.TaskID) {
		focus(ctx, msg, errorDisplay)
	}
	logf("%s", msg)
	return nil, false
}

// firstFailure reports whether this task id has not been announced yet.
// The set is bounded: hunt tasks are short-lived and only a handful can be
// queued at a time, so keeping the most recent entries is enough to avoid
// unbounded growth in a long-running agent process.
var (
	announcedMu  sync.Mutex
	announced    = make(map[int64]struct{})
	maxAnnounced = 16
)

func firstFailure(taskID int64) bool {
	announcedMu.Lock()
	defer announcedMu.Unlock()

	if _, seen := announced[taskID]; seen {
		return false
	}
	if len(announced) >= maxAnnounced {
		announced = make(map[int64]struct{})
	}
	announced[taskID] = struct{}{}
	return true
}

// readRows performs one OCR pass over all rows.
//
// It returns the counts parsed so far, the raw OCR strings, and an error when at
// least one row could not be read reliably.
func readRows(ctx *maa.Context, arg *maa.CustomRecognitionArg, rows []rockRow) ([]int, []string, error) {
	img := currentImage(ctx, arg)
	if img == nil {
		return nil, nil, fmt.Errorf("没有可用截图")
	}

	counts := make([]int, len(rows))
	raw := make([]string, len(rows))
	var failures []string

	for i, row := range rows {
		text, err := ocrRow(ctx, img, row)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s(%v)", labelOf(row), err))
			continue
		}
		raw[i] = text
		value, cleaned, ok := parseRockCount(text)
		if !ok {
			failures = append(failures, fmt.Sprintf("%s=%q", labelOf(row), text))
			continue
		}
		counts[i] = value
		raw[i] = cleaned
	}

	if len(failures) > 0 {
		return counts, raw, fmt.Errorf("%s", strings.Join(failures, " "))
	}
	return counts, raw, nil
}

// ocrRow runs a single-ROI OCR through the framework and returns the text it saw.
func ocrRow(ctx *maa.Context, img image.Image, row rockRow) (string, error) {
	if len(row.Roi) != 4 {
		return "", fmt.Errorf("roi 需要 4 个整数")
	}

	// only_rec: the ROI is already tight around one number, so skip text
	// detection and decode the strip as a single line.
	param := &maa.OCRParam{
		ROI:     maa.NewTargetRect(rectOf(row.Roi)),
		OnlyRec: true,
	}

	detail, err := ctx.RunRecognitionDirect(maa.RecognitionTypeOCR, param, img)
	if err != nil {
		return "", err
	}
	return firstOCRText(detail), nil
}

// firstOCRText pulls the most trustworthy text out of an OCR recognition
// detail: the filtered (matched) results first, then everything the model saw,
// then the single best result.
func firstOCRText(detail *maa.RecognitionDetail) string {
	if detail == nil || detail.Results == nil {
		return ""
	}
	for _, group := range [][]*maa.RecognitionResult{detail.Results.Filtered, detail.Results.All} {
		for _, res := range group {
			if res == nil {
				continue
			}
			if ocr, ok := res.AsOCR(); ok && strings.TrimSpace(ocr.Text) != "" {
				return ocr.Text
			}
		}
	}
	if best := detail.Results.Best; best != nil {
		if ocr, ok := best.AsOCR(); ok {
			return ocr.Text
		}
	}
	return ""
}

// currentImage returns the frame to read from: a freshly captured one when
// possible, so successive attempts see updated numbers, else the frame the
// framework handed us.
func currentImage(ctx *maa.Context, arg *maa.CustomRecognitionArg) image.Image {
	if img := captureFresh(ctx); img != nil {
		return img
	}
	return arg.Img
}

// captureFresh asks the controller for a new screenshot. Returns nil when the
// controller is unavailable or the capture fails, in which case the caller
// reuses the framework's frame.
func captureFresh(ctx *maa.Context) image.Image {
	tasker := ctx.GetTasker()
	if tasker == nil {
		return nil
	}
	ctrl := tasker.GetController()
	if ctrl == nil {
		return nil
	}
	if !ctrl.PostScreencap().Wait().Success() {
		return nil
	}
	img, err := ctrl.CacheImage()
	if err != nil {
		return nil
	}
	return img
}

// focus surfaces a message in MXU. The payload uses the object form of the
// `focus` template (content + display channels) documented in the PI v2 and
// pipeline protocols, and implemented by MXU 2.5.3.
func focus(ctx *maa.Context, content string, display []string) {
	node := maa.NewNode(focusNodeName).
		SetFocus(map[string]any{
			maa.EventNodeAction.Starting(): map[string]any{
				"content": content,
				"display": display,
			},
		}).
		SetPreDelay(0).
		SetPostDelay(0)

	pipeline := maa.NewPipeline()
	pipeline.AddNode(node)

	if _, err := ctx.RunAction(focusNodeName, maa.Rect{}, "", pipeline); err != nil {
		logf("focus failed: %v\n%s", err, content)
	}
}

func parseLeastRockParam(raw string) (leastRockParam, error) {
	param := leastRockParam{}
	if strings.TrimSpace(raw) != "" {
		if err := json.Unmarshal([]byte(raw), &param); err != nil {
			return param, err
		}
	}
	if len(param.Rows) == 0 {
		return param, fmt.Errorf("rows 为空")
	}
	for i, row := range param.Rows {
		if strings.TrimSpace(row.Node) == "" {
			return param, fmt.Errorf("rows[%d].node 为空", i)
		}
		if len(row.Roi) != 4 {
			return param, fmt.Errorf("rows[%d].roi 需要 4 个整数", i)
		}
	}
	if param.SettleMS < 0 {
		param.SettleMS = 0
	}
	if param.Attempts <= 0 {
		param.Attempts = defaultAttempts
	}
	if param.IntervalMS < 0 {
		param.IntervalMS = defaultIntervalMS
	}
	return param, nil
}

func labelsOf(rows []rockRow) []string {
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, labelOf(row))
	}
	return labels
}

func labelOf(row rockRow) string {
	if strings.TrimSpace(row.Label) != "" {
		return row.Label
	}
	return row.Node
}

func rectOf(roi []int) maa.Rect {
	if len(roi) != 4 {
		return maa.Rect{}
	}
	return maa.Rect{roi[0], roi[1], roi[2], roi[3]}
}

// describeFailure renders per-row diagnostics for the failure message.
func describeFailure(rows []rockRow, counts []int, raw []string) string {
	parts := make([]string, 0, len(rows))
	for i, row := range rows {
		label := labelOf(row)
		switch {
		case i < len(counts) && counts[i] > 0:
			parts = append(parts, fmt.Sprintf("%s=%d", label, counts[i]))
		case i < len(raw) && strings.TrimSpace(raw[i]) != "":
			parts = append(parts, fmt.Sprintf("%s=?（OCR 读到 %q）", label, raw[i]))
		default:
			parts = append(parts, fmt.Sprintf("%s=读不到", label))
		}
	}
	return "本次读取结果：" + strings.Join(parts, " ")
}
