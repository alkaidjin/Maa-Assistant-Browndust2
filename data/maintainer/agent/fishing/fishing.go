package main

import (
	"fmt"
	"image"
	"math"
	"sync"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// FishingMinigame plays ONE round of the 棕色尘埃2 fishing QTE.
//
// Scope: this action is entered after the pipeline has already cast the rod
// (AutoFish_Space) and set the hook (UpFish). It therefore only does:
//
//	wait for the bar → (read bar → predict → click) * N → bar gone →
//	dismiss the result popup
//
// Why one round per invocation: the user wants every finished minigame — win or
// loss — to count as one fishing attempt, and the next cast to go back through
// AutoFish_Space. That round-trip has to be driven by the pipeline, so the
// action returns control after each fish and rewrites its own `next`:
//
//	rounds left → next.cast   (AutoFish_Space, default)
//	cap reached → next.finish (AutoFish_Finish)
//
// Rounds are counted per task id inside this process, which avoids the usual
// `max_hit` pitfall (hit counts are not reset between task runs).
//
// Unlike the upstream Python bot, which derives the cursor's direction from a
// frame counter (`frame % (half_cycle*2)`) and a hard-coded 60 FPS, this
// implementation measures the cursor's real speed from consecutive frames
// (px/ms). That removes the ref_fps / cursor_speed / cursor_half_cycle
// assumptions entirely, so it works the same under FramePool (≈20 fps) and
// DXGI (≈60 fps), and it keeps working when a fish uses its "speed up / slow
// down / stop" skill mid-round.
type FishingMinigame struct{}

var _ maa.CustomActionRunner = &FishingMinigame{}

// Run implements maa.CustomActionRunner.
//
// One invocation = one fish. On return this node's `next` is rewritten: back to
// next.cast while rounds remain, otherwise next.finish.
func (a *FishingMinigame) Run(ctx *maa.Context, arg *maa.CustomActionArg) bool {
	if ctx == nil || arg == nil {
		return false
	}

	p, err := parseFishingParam(arg.CustomActionParam)
	if err != nil {
		focus(ctx, fmt.Sprintf("钓鱼：参数无效（%v），已中止", err), errorDisplay)
		logf("param error: %v", err)
		return false
	}

	tasker := ctx.GetTasker()
	if tasker == nil {
		focus(ctx, "钓鱼：拿不到 Tasker，已中止", errorDisplay)
		return false
	}
	ctrl := tasker.GetController()
	if ctrl == nil {
		focus(ctx, "钓鱼：拿不到 Controller（游戏未连接？），已中止", errorDisplay)
		return false
	}

	if p.Mode == "calibrate" {
		p.LogSamples = true
		p.DumpColors = true
	}

	logf("run: %s", p.describe())

	var frame image.RGBA // reused across captures

	// Coordinates arrive in the 720P design space; convert them to the real
	// capture size before touching pixels. The frame size is also logged so
	// calibration runs reveal which resolution the controller actually
	// delivers (native window size, or 1280x720 when MXU scales screencaps).
	if img, err := capture(ctrl, &frame); err != nil {
		logf("initial screencap failed (%v); assuming %dx%d design space", err, designW, designH)
	} else {
		b := img.Bounds()
		p = scaleToFrame(p, img)
		logf("frame %dx%d (scale %.3f,%.3f) -> roi=%v judge=(%d,%d) settle=(%d,%d)",
			b.Dx(), b.Dy(),
			float64(b.Dx())/designW, float64(b.Dy())/designH,
			p.Bar.Roi, p.Judge.X, p.Judge.Y, p.Settle.X, p.Settle.Y)
	}

	var stats runStats
	played, clicks := a.playOnce(ctx, p, ctrl, &frame, &stats)

	n := rounds.inc(arg.TaskID, played)

	if !played {
		streak := rounds.noBarStreak(arg.TaskID)
		logf("  no bar this round (consecutive %d/%d)", streak, p.MaxNoBar)
		if streak >= p.MaxNoBar {
			msg := fmt.Sprintf("钓鱼：连续 %d 次进入小游戏后都没等到进度条，任务中止。请确认 UpFish 真的触发了小游戏，并按日志里的 COLOR DUMP 校准 bar.roi / colors。", streak)
			logf("%s", msg)
			focus(ctx, msg, errorDisplay)
			return false
		}
	}

	if p.Mode == "calibrate" {
		msg := fmt.Sprintf("标定结束：本次采样 %d 帧，其中 %d 帧(%.0f%%)识别出进度条。若命中率过低，请按日志里的 COLOR DUMP 调整 bar.roi / colors。",
			stats.Frames, stats.Valid, stats.hitRate())
		logf("%s", msg)
		focus(ctx, msg, logDisplay)
		// 标定只看一轮，不循环回抛竿。
		return a.redirect(ctx, p, p.Next.Finish)
	}

	target := p.Next.Cast
	if n >= p.MaxCount {
		target = p.Next.Finish
		focus(ctx, fmt.Sprintf("钓鱼完成 %d 次，进入收尾", n), logDisplay)
	}
	logf("round %d/%d done (%d judgement clicks), next=%s", n, p.MaxCount, clicks, target)

	return a.redirect(ctx, p, target)
}

// redirect rewrites this node's `next` so the pipeline either casts again or
// wraps up. A failure here must abort: silently falling through to the default
// `next` would keep casting forever.
func (a *FishingMinigame) redirect(ctx *maa.Context, p fishingParam, target string) bool {
	if err := ctx.OverrideNext(p.Next.Node, []maa.NextItem{{Name: target}}); err != nil {
		logf("OverrideNext(%s -> %s) failed: %v", p.Next.Node, target, err)
		focus(ctx, fmt.Sprintf("钓鱼：改写下一节点 %s → %s 失败（%v）。为避免无限抛竿已中止；请检查 pipeline 里这两个节点是否都存在。",
			p.Next.Node, target, err), errorDisplay)
		return false
	}
	return true
}

// playOnce waits for the bar, plays it out and dismisses the result popup.
//
// A round in which no bar appeared still counts as one fishing attempt, but too
// many in a row abort the task (see MaxNoBar).
func (a *FishingMinigame) playOnce(ctx *maa.Context, p fishingParam, ctrl *maa.Controller, frame *image.RGBA, stats *runStats) (bool, int) {
	if !a.waitForBar(ctx, p, ctrl, frame) {
		return false, 0
	}
	clicks, _ := a.play(ctx, p, ctrl, frame, stats)

	a.dismissResult(ctx, p, ctrl)
	return true, clicks
}

// dismissResult clicks the lower center of the screen several times to close
// the "fish caught" result popup. The popup fade-in can ignore early taps, so
// we wait a moment then click repeatedly for a configurable window.
func (a *FishingMinigame) dismissResult(ctx *maa.Context, p fishingParam, ctrl *maa.Controller) {
	if !p.Settle.Enabled || !p.clicks() {
		settle(p.Settle.DelayMS)
		return
	}
	// Give the result popup time to appear before the first click.
	if p.Settle.DelayMS > 0 {
		settle(p.Settle.DelayMS / 2)
	}
	for i := 0; i < p.Settle.Clicks; i++ {
		if stopping(ctx) {
			return
		}
		ctrl.PostClick(int32(p.Settle.X), int32(p.Settle.Y)).Wait()
		settle(p.Settle.IntervalMS)
	}
}

// tap performs one judgement click on the QTE.
func (a *FishingMinigame) tap(ctx *maa.Context, p fishingParam, ctrl *maa.Controller) {
	if !p.clicks() {
		logf("    [dry-run] would click now")
		return
	}
	if p.Judge.Key > 0 {
		ctrl.PostClickKey(int32(p.Judge.Key)).Wait()
		return
	}
	ctrl.PostClick(int32(p.Judge.X), int32(p.Judge.Y)).Wait()
}

// runStats counts analysed frames, for the calibration report.
type runStats struct {
	Frames int
	Valid  int
}

func (s *runStats) hitRate() float64 {
	if s.Frames == 0 {
		return 0
	}
	return float64(s.Valid) / float64(s.Frames) * 100
}

// roundCounter tracks how many rounds each task has played and how many in a row
// failed to show a bar. Keyed by task id, so a new run starts from zero without
// needing an explicit reset node — the usual `max_hit` residue problem.
type roundCounter struct {
	mu     sync.Mutex
	counts map[int64]int
	noBar  map[int64]int
}

var rounds = &roundCounter{counts: map[int64]int{}, noBar: map[int64]int{}}

// inc records one finished round and returns the new total for this task.
func (c *roundCounter) inc(taskID int64, played bool) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.counts) > 64 {
		// Never happens in practice; keeps the map from growing without bound.
		c.counts = map[int64]int{}
		c.noBar = map[int64]int{}
	}
	c.counts[taskID]++
	if played {
		c.noBar[taskID] = 0
	} else {
		c.noBar[taskID]++
	}
	return c.counts[taskID]
}

// noBarStreak returns how many consecutive rounds saw no bar.
func (c *roundCounter) noBarStreak(taskID int64) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.noBar[taskID]
}

// waitForBar polls until the minigame bar shows up.
//
// A bar that never appears means either the hook-set click missed, or the bar
// ROI / colour filters are wrong.
func (a *FishingMinigame) waitForBar(ctx *maa.Context, p fishingParam, ctrl *maa.Controller, frame *image.RGBA) bool {
	deadline := time.Now().Add(time.Duration(p.Timing.BarWaitMS) * time.Millisecond)
	for {
		if stopping(ctx) {
			return false
		}
		if time.Now().After(deadline) {
			return false
		}
		img, err := capture(ctrl, frame)
		if err == nil {
			f := analyzeBar(img, p.Bar.Roi, p.Colors, p.Bar.MinColHits, p.Bar.MinZoneWidth, p.Bar.CursorMinHits, p.Bar.ZoneGap)
			if f.Valid {
				return true
			}
		}
		settle(p.Timing.PollMS)
	}
}

// play runs the QTE until the bar disappears or the round times out.
//
// Every frame is re-decided: the wait time is only trusted for the last
// poll interval, so a fish that suddenly speeds up, slows down or freezes the
// cursor cannot make us click into empty space.
func (a *FishingMinigame) play(ctx *maa.Context, p fishingParam, ctrl *maa.Controller, frame *image.RGBA, stats *runStats) (int, bool) {
	roundDeadline := time.Now().Add(time.Duration(p.Timing.MinigameSecondsMS) * time.Millisecond)
	barDeadline := time.Now().Add(time.Duration(p.Timing.BarWaitMS) * time.Millisecond)

	vel := newVelocity(p.Timing.MaxVelSamples)
	left := float64(p.Bar.Roi[0] + p.Bar.PaddingLeft)
	right := float64(p.Bar.Roi[0] + p.Bar.Roi[2] - p.Bar.PaddingRight)

	invalid := 0
	seenValid := false
	clicks := 0
	dumped := false

	for {
		if stopping(ctx) {
			return clicks, clicks > 0
		}
		now := time.Now()
		if now.After(roundDeadline) {
			logf("  minigame timed out after %d clicks", clicks)
			return clicks, clicks > 0
		}

		capStart := time.Now()
		img, err := capture(ctrl, frame)
		capEnd := time.Now()
		if err != nil {
			settle(p.Timing.PollMS)
			continue
		}
		if p.DumpColors && !dumped {
			dumped = true
			logf("  COLOR DUMP: %s", dumpRowColors(img, p.Bar.Roi))
		}

		f := analyzeBar(img, p.Bar.Roi, p.Colors, p.Bar.MinColHits, p.Bar.MinZoneWidth, p.Bar.CursorMinHits, p.Bar.ZoneGap)
		// Remove any part of the safe/crit zones covered by the forbidden
		// green zone before making decisions.
		f.Blue = subtractZones(f.Blue, f.Green, p.Bar.MinZoneWidth)
		f.Yellow = subtractZones(f.Yellow, f.Green, p.Bar.MinZoneWidth)
		if stats != nil {
			stats.Frames++
			if f.Valid {
				stats.Valid++
			}
		}
		if !f.Valid {
			if !seenValid {
				if now.After(barDeadline) {
					logf("  bar never became readable (%.1fs)", float64(p.Timing.BarWaitMS)/1000)
					return clicks, false
				}
				settle(p.Timing.PollMS)
				continue
			}
			invalid++
			if invalid >= p.Timing.EndInvalidFrames {
				logf("  bar gone for %d frames, round over (%d clicks)", invalid, clicks)
				return clicks, clicks > 0
			}
			settle(p.Timing.PollMS)
			continue
		}
		seenValid = true
		invalid = 0

		vel.add(f.CursorX, now)
		v := vel.value() // px/ms, signed

		// The frame we just analysed is already stale: a screencap costs
		// ~100ms, so by the time we decide, the marker has moved on. Assume
		// the pixels come from the middle of the capture and extrapolate.
		ageMS := float64(time.Since(capStart).Milliseconds()) -
			float64(capEnd.Sub(capStart).Milliseconds())/2
		rawX := f.CursorX
		f.CursorX = rawX + v*ageMS

		if p.LogSamples {
			logf("  cursor=%6.1f (+%4.0fms -> %6.1f) v=%+.3fpx/ms %s",
				rawX, ageMS, f.CursorX, v, f.String())
		}

		// Free hit: if the cursor is already inside a safe/crit zone, click
		// immediately instead of waiting for a potentially worse target. A
		// margin keeps us off the very edge, where the click would land after
		// the marker has already left.
		margin := math.Abs(v)*float64(p.Timing.InputCompMS)*1.5 + 4
		if kind, inside := insideZone(f, margin); inside {
			a.tap(ctx, p, ctrl)
			clicks++
			logf("    click #%d on %s (cursor already inside zone at %.0f)", clicks, kind, f.CursorX)
			settle(p.Timing.ResetMS)
			vel.reset()
			continue
		}

		// Teleport guard: the game periodically resets the cursor to the left
		// end and repositions the yellow zone (and some fish skills shove it).
		// That shows up as an impossible speed — drop the velocity window
		// instead of planning a click on it.
		if math.Abs(v) > p.Timing.MaxVel {
			logf("  teleport? v=%+.3f exceeds max_vel, resampling", v)
			vel.reset()
			settle(p.Timing.PollMS)
			continue
		}

		// Frozen cursor (fish "stop" skill, or a brief hitch): no prediction is
		// possible, but if the marker already sits inside a zone the click is
		// free. Requires a genuinely stable window so the first frames after a
		// reset (v not yet measured) are not mistaken for a freeze.
		if math.Abs(v) < 0.02 && vel.samples() >= 4 && vel.spanMS() >= 150 {
			if kind, inside := insideZone(f, margin); inside {
				a.tap(ctx, p, ctrl)
				clicks++
				logf("    click #%d on %s (frozen cursor=%.0f)", clicks, kind, f.CursorX)
				settle(p.Timing.ResetMS)
				vel.reset()
			} else {
				settle(p.Timing.PollMS)
			}
			continue
		}

		target, wait, kind, ok := planClick(f, v, left, right,
			float64(p.Timing.WaitCapMS),
			float64(p.Timing.YellowMaxWaitMS),
			float64(p.Timing.YellowExtraMS),
			p.Timing.BlueAimRatio,
			p.Timing.YellowAimRatio)
		if !ok {
			settle(p.Timing.PollMS)
			continue
		}

		threshold := float64(p.Timing.InputCompMS + p.Timing.PollMS)
		if wait > threshold {
			// Too far out to trust the prediction; re-decide next frame.
			settle(p.Timing.PollMS)
			continue
		}

		if wait > float64(p.Timing.InputCompMS) {
			settle(int(wait) - p.Timing.InputCompMS)
		}
		a.tap(ctx, p, ctrl)
		clicks++
		logf("    click #%d on %s (target=%.0f wait=%.0fms v=%+.3f)", clicks, kind, target, wait, v)

		settle(p.Timing.ResetMS)
		vel.reset()
	}
}

// ---------------------------------------------------------------------------
// Prediction
// ---------------------------------------------------------------------------

// planClick picks the best target on the bar and how long to wait before
// clicking.
//
// Yellow is only preferred when it is reachable soon AND not much later than
// the earliest blue crossing. This avoids gambling on a far-away critical zone
// when a safe blue zone is already available. The aim point inside a zone is
// chosen from the entry edge using an aim ratio, which gives late clicks more
// margin on narrow yellow zones.
func planClick(f barFrame, v, left, right, capMS, yellowMaxWaitMS, yellowExtraMS, blueAim, yellowAim float64) (target, wait float64, kind string, ok bool) {
	if math.Abs(v) < 1e-4 {
		return 0, 0, "", false
	}

	aimX := func(z zone, aim float64) float64 {
		w := float64(z.End - z.Start)
		if v > 0 {
			return float64(z.Start) + aim*w
		}
		return float64(z.End) - aim*w
	}

	bestWait := func(zones []zone, aim float64) (float64, float64, bool) {
		bestW := math.Inf(1)
		bestT := 0.0
		for _, z := range zones {
			t := aimX(z, aim)
			w, good := travelTime(f.CursorX, t, v, left, right)
			if !good || w < 0 || w > capMS {
				continue
			}
			if w < bestW {
				bestW, bestT = w, t
			}
		}
		return bestT, bestW, !math.IsInf(bestW, 1)
	}

	tBlue, wBlue, blueOK := bestWait(f.Blue, blueAim)
	tYellow, wYellow, yellowOK := bestWait(f.Yellow, yellowAim)

	// Prefer yellow only if it is quick and not much later than blue.
	if yellowOK && wYellow <= yellowMaxWaitMS && (wYellow <= wBlue+yellowExtraMS || !blueOK) {
		return tYellow, wYellow, "yellow", true
	}
	if blueOK {
		return tBlue, wBlue, "blue", true
	}
	if yellowOK {
		return tYellow, wYellow, "yellow", true
	}
	return 0, 0, "", false
}

// travelTime returns how many milliseconds the cursor needs to reach target,
// bouncing off the bar ends when the target lies behind it.
func travelTime(cursorX, target, v, left, right float64) (float64, bool) {
	dist := target - cursorX
	if (v > 0 && dist < 0) || (v < 0 && dist > 0) {
		if v > 0 {
			dist = (right - cursorX) + (right - target)
		} else {
			dist = (cursorX - left) + (target - left)
		}
	}
	d := math.Abs(dist)
	if right > left && (target < left || target > right) {
		return 0, false
	}
	return d / math.Abs(v), true
}

// insideZone reports whether the cursor currently sits inside a safe/crit zone,
// but never if it is inside the forbidden green zone. `margin` requires the
// marker to be that far from the zone edges, so a click already on its way
// cannot land after the marker has left.
func insideZone(f barFrame, margin float64) (string, bool) {
	for _, z := range f.Green {
		if f.CursorX >= float64(z.Start) && f.CursorX < float64(z.End) {
			return "", false
		}
	}
	for _, z := range f.Yellow {
		if f.CursorX >= float64(z.Start)+margin && f.CursorX < float64(z.End)-margin {
			return "yellow-frozen", true
		}
	}
	for _, z := range f.Blue {
		if f.CursorX >= float64(z.Start)+margin && f.CursorX < float64(z.End)-margin {
			return "blue-frozen", true
		}
	}
	return "", false
}

// velocity smooths the cursor speed over the last few samples.
//
// A single frame pair is noisy: the cursor is only a few pixels wide and the
// screenshot cadence jitters. Averaging over the last N samples keeps the sign
// (direction) stable without lagging behind a fish's speed-up skill for long.
type velocity struct {
	xs  []float64
	ts  []time.Time
	max int
}

func newVelocity(max int) *velocity {
	if max <= 0 {
		max = defaultMaxVelSamples
	}
	return &velocity{max: max}
}

func (v *velocity) add(x float64, t time.Time) {
	v.xs = append(v.xs, x)
	v.ts = append(v.ts, t)
	if len(v.xs) > v.max {
		v.xs = v.xs[1:]
		v.ts = v.ts[1:]
	}
}

func (v *velocity) reset() {
	v.xs = v.xs[:0]
	v.ts = v.ts[:0]
}

// samples returns how many positions the window currently holds.
func (v *velocity) samples() int { return len(v.xs) }

// spanMS returns the time between the oldest and newest sample, in ms.
func (v *velocity) spanMS() float64 {
	if len(v.ts) < 2 {
		return 0
	}
	return v.ts[len(v.ts)-1].Sub(v.ts[0]).Seconds() * 1000
}

// value returns px/ms, signed by travel direction. 0 when unknown.
func (v *velocity) value() float64 {
	if len(v.xs) < 2 {
		return 0
	}
	dx := v.xs[len(v.xs)-1] - v.xs[0]
	dt := v.ts[len(v.ts)-1].Sub(v.ts[0]).Seconds() * 1000
	if dt <= 0 {
		return 0
	}
	return dx / dt
}

// ---------------------------------------------------------------------------
// Framework helpers
// ---------------------------------------------------------------------------

// Design resolution. Every coordinate in custom_action_param is written in
// this 720P space — the same space MXU/MaaFramework uses for pipeline
// recognition — so one set of numbers works for any window resolution.
const (
	designW = 1280
	designH = 720
)

// scaleToFrame converts the 720P design-space coordinates in p to the actual
// screencap resolution. The agent reads pixels straight from controller
// screencaps, which come back at the real capture size (e.g. 1920x1080 for a
// windowed game, or 1280x720 if the controller already scales screenshots),
// so coordinates must be scaled by the measured frame size, never assumed.
func scaleToFrame(p fishingParam, img *image.RGBA) fishingParam {
	b := img.Bounds()
	sx := float64(b.Dx()) / designW
	sy := float64(b.Dy()) / designH
	if sx == 1 && sy == 1 {
		return p
	}
	p.Bar.Roi = []int{
		int(float64(p.Bar.Roi[0]) * sx),
		int(float64(p.Bar.Roi[1]) * sy),
		int(float64(p.Bar.Roi[2]) * sx),
		int(float64(p.Bar.Roi[3]) * sy),
	}
	p.Bar.PaddingLeft = int(float64(p.Bar.PaddingLeft) * sx)
	p.Bar.PaddingRight = int(float64(p.Bar.PaddingRight) * sx)
	p.Judge.X = int(float64(p.Judge.X) * sx)
	p.Judge.Y = int(float64(p.Judge.Y) * sy)
	p.Settle.X = int(float64(p.Settle.X) * sx)
	p.Settle.Y = int(float64(p.Settle.Y) * sy)
	return p
}

// capture grabs a fresh frame into buf when sizes match, else into a new one.
func capture(ctrl *maa.Controller, buf *image.RGBA) (*image.RGBA, error) {
	if !ctrl.PostScreencap().Wait().Success() {
		return nil, fmt.Errorf("screencap failed")
	}
	return ctrl.CacheImageInto(buf)
}

func stopping(ctx *maa.Context) bool {
	t := ctx.GetTasker()
	return t != nil && t.Stopping()
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
