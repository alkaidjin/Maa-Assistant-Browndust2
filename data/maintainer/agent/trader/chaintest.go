package main

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// TradeChainTest is the "全链路测试" custom action.
//
// The pipeline (Tradecaravan_Start → … → Bargain_TC_1/2 → Trade_ShopMenu)
// has already navigated from the main screen to the shop and run bargain by
// the time this action fires on Trade_ShopMenu. This action then:
//
//	1. opens the BUY page and buys exactly 1 item (lowest %-marker if visible,
//	   otherwise the first item row as a mechanical fallback);
//	2. opens the SELL page and sells exactly 1 item (120%-marker if visible,
//	   otherwise today's calendar peak-item name as fallback).
//
// Everything is OCR-driven: no not-yet-captured template is required, and all
// click coordinates come straight out of recognized boxes so it works on any
// frame resolution. Frames at each key step are dumped to
// <workspace>/debug/tradetest/ for post-run diagnosis.
type TradeChainTest struct{}

var _ maa.CustomActionRunner = &TradeChainTest{}

// key ROIs / points in the 720P design space.
var (
	// left shop menu where 购买 / 出售 live — same ROI the pipeline uses.
	tcMenuROI = maa.Rect{65, 62, 86, 159}
	// broad area covering the scrollable item list on buy/sell pages.
	tcListROI = maa.Rect{170, 80, 1080, 600}
	// popup area of the quantity dialog.
	tcPopupROI = maa.Rect{300, 170, 680, 380}
	// center band where result toasts appear.
	tcToastROI = maa.Rect{400, 280, 480, 140}
	// fallbacks if OCR cannot locate a button.
	tcConfirmFallback = image.Point{875, 485}
	tcCloseFallback   = image.Point{947, 215}
)

var (
	pctRe   = regexp.MustCompile(`(\d{1,3})\s*%?`)
	noiseRe = regexp.MustCompile(`[0-9%\s,，.。:：;；<>《》()\[\]【】/\\|+\-*#]`)
)

// Run implements maa.CustomActionRunner. Always returns true so that the
// pipeline proceeds to the ReturnHome chain even if a half fails.
func (a *TradeChainTest) Run(ctx *maa.Context, arg *maa.CustomActionArg) bool {
	tasker := ctx.GetTasker()
	if tasker == nil || tasker.GetController() == nil {
		focus(ctx, "链路测试：拿不到 Controller，已中止", errorDisplay)
		return false
	}
	ctrl := tasker.GetController()

	img0, err := capture(ctrl, nil)
	if err != nil {
		focus(ctx, "链路测试：截图失败，已中止", errorDisplay)
		return false
	}
	b := img0.Bounds()
	sx := float64(b.Dx()) / float64(designW)
	sy := float64(b.Dy()) / float64(designH)
	logf("chain test: frame=%dx%d scale=(%.3f,%.3f)", b.Dx(), b.Dy(), sx, sy)
	saveDebug(img0, "00_shopmenu")

	focus(ctx, "链路测试开始：进店与砍价已完成，现在买 1 件、再卖 1 件", logDisplay)

	// calendar is best-effort fallback for the sell half only.
	cal, day, calErr := loadCalendar(defaultTradeParam())
	if calErr != nil {
		logf("calendar unavailable (sell fallback disabled): %v", calErr)
	}

	buyOK := tcBuyOne(ctx, ctrl, sx, sy)
	sellOK := tcSellOne(ctx, ctrl, sx, sy, cal, day)

	summary := fmt.Sprintf("链路测试结束：买 1 件=%v，卖 1 件=%v。见 debug/tradetest/ 截图", buyOK, sellOK)
	logf("%s", summary)
	focus(ctx, summary, logDisplay)
	return true
}

// ---- buy half -----------------------------------------------------------

func tcBuyOne(ctx *maa.Context, ctrl *maa.Controller, sx, sy float64) bool {
	focus(ctx, "【1/4】打开购买页（点左侧“购买”）", logDisplay)
	if !tcClickTab(ctx, ctrl, sx, sy, "购买") {
		return false
	}
	sleep2(1200)

	frame, err := capture(ctrl, nil)
	if err != nil {
		return false
	}
	saveDebug(frame, "01_buypage")
	boxes := tcDumpOCR(ctx, frame, scaleRect(tcListROI, sx, sy), "buy page")

	// choose lowest percentage marker; fall back to first name-like row.
	target := pickLowestPct(boxes)
	how := "lowest % marker"
	if target == nil {
		target = pickFirstNameRow(boxes)
		how = "first row fallback (no % marker seen)"
	}
	if target == nil {
		focus(ctx, "【购买】列表里没有识别到任何商品行，购买未执行", errorDisplay)
		return false
	}
	logf("buy target: %q @ %v (%s)", target.Text, target.Box, how)
	focus(ctx, fmt.Sprintf("【2/4】购买：%s（%s），数量默认 1", strings.TrimSpace(target.Text), how), logDisplay)

	if !tcClickBox(ctx, target) {
		return false
	}
	return tcPopupConfirm(ctx, ctrl, sx, sy, "02_buypopup", "购买")
}

// ---- sell half ----------------------------------------------------------

func tcSellOne(ctx *maa.Context, ctrl *maa.Controller, sx, sy float64, cal *PriceCalendar, day int) bool {
	focus(ctx, "【3/4】打开发售页（点左侧“出售”）", logDisplay)
	if !tcClickTab(ctx, ctrl, sx, sy, "出售") {
		return false
	}
	sleep2(1200)

	frame, err := capture(ctrl, nil)
	if err != nil {
		return false
	}
	saveDebug(frame, "03_sellpage")
	boxes := tcDumpOCR(ctx, frame, scaleRect(tcListROI, sx, sy), "sell page")

	target := pickPeakPct(boxes)
	how := "120% marker"
	if target == nil && cal != nil {
		for _, it := range cal.sellList(day) {
			for _, r := range boxes {
				if it.matchName(r.Text) {
					target = r
					how = "calendar fallback: " + it.Item
					break
				}
			}
			if target != nil {
				break
			}
		}
	}
	if target == nil {
		focus(ctx, "【发售】未发现峰值（120%）商品，出售未执行（不硬卖）", errorDisplay)
		return false
	}
	logf("sell target: %q @ %v (%s)", target.Text, target.Box, how)
	focus(ctx, fmt.Sprintf("【4/4】发售：%s（%s），数量默认 1", strings.TrimSpace(target.Text), how), logDisplay)

	if !tcClickBox(ctx, target) {
		return false
	}
	return tcPopupConfirm(ctx, ctrl, sx, sy, "04_sellpopup", "出售")
}

// ---- shared mechanics ---------------------------------------------------

// tcClickTab OCRs the left menu and clicks the box containing tabWord.
func tcClickTab(ctx *maa.Context, ctrl *maa.Controller, sx, sy float64, tabWord string) bool {
	const attempts = 3
	for i := 0; i < attempts; i++ {
		frame, err := capture(ctrl, nil)
		if err != nil {
			return false
		}
		boxes, err := runOCR(ctx, frame, scaleRect(tcMenuROI, sx, sy), tabWord)
		if err != nil {
			logf("menu OCR failed: %v", err)
			sleep2(500)
			continue
		}
		if box := findTextBox(boxes, tabWord); box != nil {
			return tcClickBox(ctx, box)
		}
		logf("tab %q not seen in left menu (%d/3)", tabWord, i+1)
		sleep2(600)
	}
	return false
}

// tcPopupConfirm waits for the quantity popup, leaves quantity at default
// (1), clicks 确认, and waits for the popup to close / a 完成 toast.
func tcPopupConfirm(ctx *maa.Context, ctrl *maa.Controller, sx, sy float64, debugName, label string) bool {
	popupROI := scaleRect(tcPopupROI, sx, sy)

	// wait for popup: "拥有" / "MAX" / slider text appears.
	var popup image.Image
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		f, err := capture(ctrl, nil)
		if err == nil {
			boxes, err := runOCR(ctx, f, popupROI, "拥有", "MAX")
			if err == nil && (findTextBox(boxes, "拥有") != nil || findTextBox(boxes, "MAX") != nil) {
				popup = f
				break
			}
		}
		sleep2(300)
	}
	if popup == nil {
		logf("%s: quantity popup did not appear", label)
		focus(ctx, label+"：数量弹窗未出现，未确认", errorDisplay)
		return false
	}
	saveDebug(popup, debugName)

	// default quantity is 1 — do not touch MIN/MAX/slider.

	// find the confirm button by OCR, fall back to fixed point.
	confirmBoxes, _ := runOCR(ctx, popup, popupROI, "确认", "确定")
	btn := findTextBox(confirmBoxes, "确认")
	if btn == nil {
		btn = findTextBox(confirmBoxes, "确定")
	}
	if btn != nil {
		tcClickBox(ctx, btn)
	} else {
		p := scalePoint(tcConfirmFallback, sx, sy)
		logf("%s: confirm button not OCR'd, fallback click (%d,%d)", label, p.X, p.Y)
		clickPoint(ctx, ctrl, p.X, p.Y)
	}

	// wait completion: popup gone or toast 完成.
	finish := time.Now().Add(8 * time.Second)
	for time.Now().Before(finish) {
		if stopping(ctx) {
			return false
		}
		f, err := capture(ctrl, nil)
		if err != nil {
			sleep2(200)
			continue
		}
		toast, _ := runOCR(ctx, f, scaleRect(tcToastROI, sx, sy), "完成")
		if findTextBox(toast, "完成") != nil {
			logf("%s confirmed: toast 完成", label)
			return true
		}
		still, _ := runOCR(ctx, f, popupROI, "拥有", "MAX")
		if findTextBox(still, "拥有") == nil && findTextBox(still, "MAX") == nil {
			logf("%s confirmed: popup closed", label)
			return true
		}
		sleep2(300)
	}
	logf("%s: not confirmed within 8s", label)
	focus(ctx, label+"：已点确认但未检测到完成，请人工核对", errorDisplay)
	return false
}

// tcClickBox clicks the center of an OCR box, using frame-space coordinates.
func tcClickBox(ctx *maa.Context, box *maa.OCRResult) bool {
	cx := box.Box.X() + box.Box.Width()/2
	cy := box.Box.Y() + box.Box.Height()/2
	self := maa.Rect{cx, cy, 1, 1}
	_, err := ctx.RunActionDirect(maa.ActionTypeClick, &maa.ClickParam{
		Target: maa.NewTargetRect(self),
	}, self, nil)
	return err == nil
}

// tcDumpOCR OCRs a region, logs every box, and returns them.
func tcDumpOCR(ctx *maa.Context, img image.Image, roi maa.Rect, tag string) []*maa.OCRResult {
	boxes, err := runOCR(ctx, img, roi)
	if err != nil {
		logf("%s OCR failed: %v", tag, err)
		return nil
	}
	logf("%s: %d OCR box(es)", tag, len(boxes))
	for i, r := range boxes {
		logf("  [%02d] %q box=[%d,%d,%d,%d]", i, r.Text,
			r.Box.X(), r.Box.Y(), r.Box.Width(), r.Box.Height())
	}
	return boxes
}

// ---- selection heuristics ----------------------------------------------

// pickLowestPct chooses the box with the smallest percentage (50..130),
// preferring boxes whose raw text contains '%'.
func pickLowestPct(boxes []*maa.OCRResult) *maa.OCRResult {
	var best *maa.OCRResult
	bestVal := 0
	bestPct := false
	for _, r := range boxes {
		v, ok := readPct(r.Text)
		if !ok {
			continue
		}
		hasPct := strings.Contains(r.Text, "%")
		if best == nil || v < bestVal || (v == bestVal && hasPct && !bestPct) {
			best, bestVal, bestPct = r, v, hasPct
		}
	}
	return best
}

// pickPeakPct chooses a box with percentage >= 118 (i.e. 120 peak), highest
// wins.
func pickPeakPct(boxes []*maa.OCRResult) *maa.OCRResult {
	var best *maa.OCRResult
	bestVal := 0
	for _, r := range boxes {
		v, ok := readPct(r.Text)
		if !ok || v < 118 {
			continue
		}
		if best == nil || v > bestVal {
			best, bestVal = r, v
		}
	}
	return best
}

// pickFirstNameRow returns the topmost (then leftmost) box that looks like an
// item name: at least 2 meaningful chars and not pure punctuation/numbers.
func pickFirstNameRow(boxes []*maa.OCRResult) *maa.OCRResult {
	var best *maa.OCRResult
	for _, r := range boxes {
		text := strings.TrimSpace(r.Text)
		if len([]rune(text)) < 2 {
			continue
		}
		if _, isNum := readPct(text); isNum && len(noiseRe.ReplaceAllString(text, "")) == 0 {
			continue
		}
		if noiseRe.ReplaceAllString(text, "") == "" {
			continue
		}
		switch text {
		case "购买", "出售", "商店":
			continue
		}
		if best == nil || r.Box.Y() < best.Box.Y() ||
			(r.Box.Y() == best.Box.Y() && r.Box.X() < best.Box.X()) {
			best = r
		}
	}
	return best
}

// readPct extracts a percentage-like integer (50..130) from text.
func readPct(text string) (int, bool) {
	m := pctRe.FindStringSubmatch(text)
	if m == nil {
		return 0, false
	}
	v, err := strconv.Atoi(m[1])
	if err != nil || v < 50 || v > 130 {
		return 0, false
	}
	return v, true
}

// ---- scale / debug helpers ----------------------------------------------

func scaleRect(r maa.Rect, sx, sy float64) maa.Rect {
	return maa.Rect{
		int(float64(r[0]) * sx),
		int(float64(r[1]) * sy),
		int(float64(r[2]) * sx),
		int(float64(r[3]) * sy),
	}
}

type pt2 struct{ X, Y int }

func scalePoint(p image.Point, sx, sy float64) pt2 {
	return pt2{int(float64(p.X) * sx), int(float64(p.Y) * sy)}
}

func sleep2(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

// saveDebug writes a frame to <workspace>/debug/tradetest/<name>.png.
func saveDebug(img image.Image, name string) {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Join(filepath.Dir(exe), "..", "debug", "tradetest")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	path := filepath.Join(dir, name+".png")
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		return
	}
	logf("debug frame: %s", path)
}
