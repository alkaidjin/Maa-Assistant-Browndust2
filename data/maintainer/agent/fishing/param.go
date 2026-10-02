package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// fishingActionName must match the pipeline's action.param.custom_action value.
const fishingActionName = "FishingMinigame"

// focusNodeName is a throwaway node used to carry a `focus` payload to the UI.
const focusNodeName = "_FISHING_FOCUS_"

// logDisplay shows a message in the MXU run log only.
var logDisplay = []string{"log"}

// errorDisplay pushes a failure to every channel a user might be watching: the
// run log, an in-app toast, and a system notification. `modal` is deliberately
// not used — it would block the task queue while nobody is at the keyboard.
var errorDisplay = []string{"log", "toast", "notification"}

// ---------------------------------------------------------------------------
// Parameters — everything lives in `custom_action_param` so coordinates and
// thresholds can be corrected without rebuilding the executable.
// ---------------------------------------------------------------------------

// hsvRange is a colour filter. H is degrees (0-360), S and V are 0-255.
type hsvRange struct {
	H [2]float64 `json:"h"`
	S [2]float64 `json:"s"`
	V [2]float64 `json:"v"`
}

func (r hsvRange) match(h, s, v float64) bool {
	return h >= r.H[0] && h <= r.H[1] && s >= r.S[0] && s <= r.S[1] && v >= r.V[0] && v <= r.V[1]
}

func (r hsvRange) String() string {
	return fmt.Sprintf("H[%.0f,%.0f] S[%.0f,%.0f] V[%.0f,%.0f]",
		r.H[0], r.H[1], r.S[0], r.S[1], r.V[0], r.V[1])
}

// colors groups the colour filters used on the minigame bar.
type colors struct {
	Cursor hsvRange `json:"cursor"`
	Blue   hsvRange `json:"blue"`
	Yellow hsvRange `json:"yellow"`
	Green  hsvRange `json:"green"` // forbidden zone: clicking inside penalises
}

// judge is the click (or key) that scores inside the minigame.
//
// Key > 0 switches to the keyboard (32 = space); otherwise the button at (X, Y)
// is clicked. This is unrelated to casting — casting and hook-setting are done
// by the pipeline nodes AutoFish_Space / UpFish.
type judgeParam struct {
	X   int `json:"x"`
	Y   int `json:"y"`
	Key int `json:"key"`
}

// bar locates the minigame progress bar.
type bar struct {
	Roi          []int `json:"roi"`
	MinColHits   int   `json:"min_col_hits"`
	MinZoneWidth int   `json:"min_zone_width"`
	// CursorMinHits is how many white pixels a column needs before it counts
	// as the cursor. The marker star is much taller than any other white
	// speckle in the tight bar ROI, so this isolates it from end-cap glints.
	CursorMinHits int `json:"cursor_min_hits"`
	// ZoneGap bridges gaps this wide when merging zone columns, healing the
	// split the cursor star causes when it occludes part of a zone.
	ZoneGap      int `json:"zone_gap"`
	PaddingLeft  int `json:"padding_left"`
	PaddingRight int `json:"padding_right"`
}

// timing holds every knob of the control loop.
type timing struct {
	PollMS            int `json:"poll_ms"`
	InputCompMS       int `json:"input_comp_ms"`
	WaitCapMS         int `json:"wait_cap_ms"`
	BarWaitMS         int `json:"bar_wait_ms"`
	EndInvalidFrames  int `json:"end_invalid_frames"`
	ResetMS           int `json:"reset_ms"`
	MinigameSecondsMS int `json:"minigame_ms"`
	MaxVelSamples     int `json:"max_vel_samples"`
	// MaxVel is the largest plausible cursor speed in px/ms (720P space).
	// Anything faster is a teleport (the game periodically resets the cursor
	// and repositions the yellow zone), not motion — the velocity window is
	// dropped instead of trusted. Measured real speed: ±0.22 px/ms.
	MaxVel float64 `json:"max_vel"`
	// YellowMaxWaitMS is the longest we are willing to wait for a yellow zone.
	// If the next yellow crossing is farther out than this, we take blue now.
	YellowMaxWaitMS int `json:"yellow_max_wait_ms"`
	// YellowExtraMS: only prefer yellow if it arrives no more than this many
	// milliseconds later than the earliest blue crossing. This avoids gambling
	// on a far-away crit zone when a safe zone is already reachable.
	YellowExtraMS int `json:"yellow_extra_ms"`
	// Aim ratios (0..1) select a point inside a zone measured from the edge
	// the cursor will enter. Smaller = earlier in the zone, which gives more
	// tolerance for late clicks on narrow yellow zones.
	BlueAimRatio   float64 `json:"blue_aim_ratio"`
	YellowAimRatio float64 `json:"yellow_aim_ratio"`
}

// settle clicks repeatedly after a round ends to dismiss the result popup.
// The popup fade-in can ignore the first taps, so we keep clicking for a
// configurable window instead of relying on a single long delay.
type settleParam struct {
	Enabled    bool `json:"enabled"`
	X          int  `json:"x"`
	Y          int  `json:"y"`
	DelayMS    int  `json:"delay_ms"`
	Clicks     int  `json:"clicks"`
	IntervalMS int  `json:"interval_ms"`
}

// nextParam names the pipeline nodes this action jumps to when it finishes.
//
// Each invocation plays exactly ONE fish. Where the pipeline goes afterwards is
// decided here by counting rounds per task id, which avoids the `max_hit`
// residue problem (hit counts survive across task runs).
//
//	node   — this action's own node, whose `next` is rewritten
//	cast   — go here while rounds are left (back to AutoFish_Space)
//	finish — go here once the target count is reached
type nextParam struct {
	Node   string `json:"node"`
	Cast   string `json:"cast"`
	Finish string `json:"finish"`
}

// fishingParam is `custom_action_param` as written in
// resource/pipeline/AutoFishing.json.
type fishingParam struct {
	// Mode: "auto" plays the minigame, "dry_run" plays but never clicks,
	// "calibrate" only samples and prints what it sees (never clicks).
	Mode string `json:"mode"`
	// MaxCount is how many fish to play before redirecting to next.finish.
	// One invocation = one fish, so this is the round cap for the whole task.
	MaxCount int `json:"max_count"`
	// MaxSeconds is DEPRECATED and NOT IMPLEMENTED.
	//
	// It is parsed and normalised below but is never read at run time, so
	// putting `max_seconds` in a pipeline does nothing at all. The limits that
	// actually apply are MaxCount, Timing.MinigameMS, Timing.BarWaitMS and
	// MaxNoBar. Do not reintroduce this field into any JSON or README.
	MaxSeconds float64 `json:"max_seconds"`
	// MaxNoBar aborts the task after this many consecutive rounds in which the
	// bar never appeared — otherwise a broken UpFish would loop forever.
	MaxNoBar int         `json:"max_no_bar"`
	Judge    judgeParam  `json:"judge"`
	Bar      bar         `json:"bar"`
	Colors   colors      `json:"colors"`
	Timing   timing      `json:"timing"`
	Settle   settleParam `json:"settle"`
	Next     nextParam   `json:"next"`
	// LogSamples prints one line per analysed frame. Very chatty; meant for
	// calibration only.
	LogSamples bool `json:"log_samples"`
	// DumpColors prints, once, the raw RGB of the bar's centre row so the
	// colour filters above can be tuned from real pixels.
	DumpColors bool `json:"dump_colors"`
}

// ---------------------------------------------------------------------------
// Defaults
// ---------------------------------------------------------------------------

// All coordinates (bar.roi / judge / settle) are written in the 720P design
// space (1280x720) — the same space MXU/MaaFramework uses for pipeline
// recognition — so one set of numbers works for any window resolution. The
// agent scales them to the real screencap size at runtime (see scaleToFrame).
const (
	defaultMaxCount = 30

	defaultJudgeX = 640
	defaultJudgeY = 360

	// The REAL minigame bar is the lower rounded bar (blue hatched safe zone +
	// narrow yellow crit zone + a white star marker sliding along it).
	//
	// Measured on a 1920x1080 recording, converted to the 720P design space:
	//   water (bright blue, MUST be excluded) .......... y 585-617
	//   bar interior ................................... y 618-644
	//   cursor travel .................................. x 530-825
	// An ROI that starts above y=618 picks up the water, whose bright blue
	// makes EVERY column look "blue" — the safe zone then collapses to the
	// whole ROI and every click becomes a miss.
	defaultBarX = 500
	defaultBarY = 618
	defaultBarW = 350
	defaultBarH = 30

	defaultPaddingLeft  = 30
	defaultPaddingRight = 25

	defaultMinColHits   = 2
	defaultMinZoneWidth = 8
	// The marker star spans the full ROI height (30px). Static white
	// decorations inside the bar are only ~26px tall, so 28 isolates the
	// marker. Verified frame by frame on the recording: at 20 the marker is
	// clean, at 26 two fixed false columns (x=723/742) leak in and drag the
	// weighted centre — that alone halved the measured speed (0.09 vs 0.21).
	defaultCursorHits = 28
	defaultZoneGap    = 12

	// InputCompMS now only covers click delivery + game processing: the age of
	// the analysed frame is extrapolated separately (see play()).
	defaultPollMS            = 8
	defaultInputCompMS       = 100
	defaultWaitCapMS         = 5000
	defaultBarWaitMS         = 5000
	defaultEndInvalidFrames  = 8
	defaultResetMS           = 200
	defaultMinigameSecondsMS = 20000
	defaultMaxVelSamples     = 4
	defaultMaxVel            = 0.45

	defaultYellowMaxWaitMS = 600
	defaultYellowExtraMS   = 300
	defaultBlueAimRatio    = 0.5
	defaultYellowAimRatio  = 0.15

	defaultMaxNoBar = 3

	defaultNextNode   = "Fishing_Minigame"
	defaultNextCast   = "AutoFish_Space"
	defaultNextFinish = "AutoFish_Finish"

	defaultSettleX          = 640
	defaultSettleY          = 647
	defaultSettleDelayMS    = 1200
	defaultSettleClicks     = 8
	defaultSettleIntervalMS = 350
)

func defaultColors() colors {
	return colors{
		// The cursor is a near-white marker on a coloured bar.
		Cursor: hsvRange{H: [2]float64{0, 360}, S: [2]float64{0, 60}, V: [2]float64{200, 255}},
		// Blue = safe zone (pulls the fish in). V >= 190 keeps the dark
		// blue-grey bar background (V ~130-180) out; only the bright hatched
		// block counts.
		Blue: hsvRange{H: [2]float64{190, 220}, S: [2]float64{60, 255}, V: [2]float64{190, 255}},
		// Yellow = critical zone (worth more). Only preferred when it is
		// reachable quickly and not much later than the safe blue zone.
		Yellow: hsvRange{H: [2]float64{35, 70}, S: [2]float64{90, 255}, V: [2]float64{140, 255}},
		// Green = forbidden zone that overlaps the bar. Any part of blue or
		// yellow covered by green is removed from the candidate targets.
		// Measured on high-level map recordings: H 140-170, S 105-120,
		// V 130-165. Keep enough headroom for hue/saturation shifts.
		Green: hsvRange{H: [2]float64{130, 180}, S: [2]float64{80, 255}, V: [2]float64{90, 255}},
	}
}

func defaultParam() fishingParam {
	return fishingParam{
		Mode:       "auto",
		MaxCount:   defaultMaxCount,
		MaxSeconds: 3600,
		MaxNoBar:   defaultMaxNoBar,
		Judge:      judgeParam{X: defaultJudgeX, Y: defaultJudgeY},
		Bar: bar{
			Roi:           []int{defaultBarX, defaultBarY, defaultBarW, defaultBarH},
			MinColHits:    defaultMinColHits,
			MinZoneWidth:  defaultMinZoneWidth,
			CursorMinHits: defaultCursorHits,
			ZoneGap:       defaultZoneGap,
			PaddingLeft:   defaultPaddingLeft,
			PaddingRight:  defaultPaddingRight,
		},
		Colors: defaultColors(),
		Timing: timing{
			PollMS:            defaultPollMS,
			InputCompMS:       defaultInputCompMS,
			WaitCapMS:         defaultWaitCapMS,
			BarWaitMS:         defaultBarWaitMS,
			EndInvalidFrames:  defaultEndInvalidFrames,
			ResetMS:           defaultResetMS,
			MinigameSecondsMS: defaultMinigameSecondsMS,
			MaxVelSamples:     defaultMaxVelSamples,
			MaxVel:            defaultMaxVel,
			YellowMaxWaitMS:   defaultYellowMaxWaitMS,
			YellowExtraMS:     defaultYellowExtraMS,
			BlueAimRatio:      defaultBlueAimRatio,
			YellowAimRatio:    defaultYellowAimRatio,
		},
		Next: nextParam{Node: defaultNextNode, Cast: defaultNextCast, Finish: defaultNextFinish},
		Settle: settleParam{
			Enabled:    true,
			X:          defaultSettleX,
			Y:          defaultSettleY,
			DelayMS:    defaultSettleDelayMS,
			Clicks:     defaultSettleClicks,
			IntervalMS: defaultSettleIntervalMS,
		},
	}
}

// parseFishingParam merges raw JSON over the defaults.
func parseFishingParam(raw string) (fishingParam, error) {
	p := defaultParam()
	if strings.TrimSpace(raw) == "" {
		return p, nil
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, err
	}
	return p, p.validate()
}

func (p *fishingParam) validate() error {
	switch strings.ToLower(strings.TrimSpace(p.Mode)) {
	case "", "auto":
		p.Mode = "auto"
	case "dry_run", "dryrun":
		p.Mode = "dry_run"
	case "calibrate", "calib":
		p.Mode = "calibrate"
	default:
		return fmt.Errorf("mode %q 无效（可选 auto / dry_run / calibrate）", p.Mode)
	}
	if len(p.Bar.Roi) != 4 {
		return fmt.Errorf("bar.roi 需要 4 个整数 [x,y,w,h]")
	}
	for i, v := range p.Bar.Roi {
		if v < 0 {
			return fmt.Errorf("bar.roi[%d] 不能为负", i)
		}
	}
	if p.Bar.Roi[2] <= 0 || p.Bar.Roi[3] <= 0 {
		return fmt.Errorf("bar.roi 的宽高必须为正")
	}
	if p.Judge.Key == 0 && (p.Judge.X <= 0 || p.Judge.Y <= 0) {
		return fmt.Errorf("judge.x / judge.y 必须为正（或改用 judge.key）")
	}
	if strings.TrimSpace(p.Next.Node) == "" {
		p.Next.Node = defaultNextNode
	}
	if strings.TrimSpace(p.Next.Cast) == "" {
		p.Next.Cast = defaultNextCast
	}
	if strings.TrimSpace(p.Next.Finish) == "" {
		p.Next.Finish = defaultNextFinish
	}
	if p.Timing.PollMS <= 0 {
		p.Timing.PollMS = defaultPollMS
	}
	if p.Timing.EndInvalidFrames <= 0 {
		p.Timing.EndInvalidFrames = defaultEndInvalidFrames
	}
	if p.Timing.WaitCapMS <= 0 {
		p.Timing.WaitCapMS = defaultWaitCapMS
	}
	if p.Timing.BarWaitMS <= 0 {
		p.Timing.BarWaitMS = defaultBarWaitMS
	}
	if p.Timing.MinigameSecondsMS <= 0 {
		p.Timing.MinigameSecondsMS = defaultMinigameSecondsMS
	}
	if p.Timing.MaxVelSamples <= 0 {
		p.Timing.MaxVelSamples = defaultMaxVelSamples
	}
	if p.Bar.MinColHits <= 0 {
		p.Bar.MinColHits = defaultMinColHits
	}
	if p.Bar.MinZoneWidth <= 0 {
		p.Bar.MinZoneWidth = defaultMinZoneWidth
	}
	if p.Bar.CursorMinHits <= 0 {
		p.Bar.CursorMinHits = defaultCursorHits
	}
	if p.Bar.ZoneGap <= 0 {
		p.Bar.ZoneGap = defaultZoneGap
	}
	if p.Bar.PaddingLeft < 0 {
		p.Bar.PaddingLeft = defaultPaddingLeft
	}
	if p.Bar.PaddingRight < 0 {
		p.Bar.PaddingRight = defaultPaddingRight
	}
	if p.Timing.MaxVel <= 0 {
		p.Timing.MaxVel = defaultMaxVel
	}
	if p.Timing.YellowMaxWaitMS <= 0 {
		p.Timing.YellowMaxWaitMS = defaultYellowMaxWaitMS
	}
	if p.Timing.YellowExtraMS < 0 {
		p.Timing.YellowExtraMS = defaultYellowExtraMS
	}
	if p.Timing.BlueAimRatio <= 0 || p.Timing.BlueAimRatio > 1 {
		p.Timing.BlueAimRatio = defaultBlueAimRatio
	}
	if p.Timing.YellowAimRatio <= 0 || p.Timing.YellowAimRatio > 1 {
		p.Timing.YellowAimRatio = defaultYellowAimRatio
	}
	if p.Settle.Clicks <= 0 {
		p.Settle.Clicks = defaultSettleClicks
	}
	if p.Settle.IntervalMS <= 0 {
		p.Settle.IntervalMS = defaultSettleIntervalMS
	}
	if p.MaxNoBar <= 0 {
		p.MaxNoBar = defaultMaxNoBar
	}
	if p.MaxCount <= 0 {
		p.MaxCount = defaultMaxCount
	}
	if p.MaxSeconds <= 0 {
		p.MaxSeconds = 3600
	}
	return nil
}

// clicks reports whether this run is allowed to touch the game.
func (p *fishingParam) clicks() bool { return p.Mode == "auto" }

// describe renders the effective settings, for the run log.
func (p *fishingParam) describe() string {
	var b strings.Builder
	fmt.Fprintf(&b, "mode=%s count=%d no_bar_limit=%d", p.Mode, p.MaxCount, p.MaxNoBar)
	if p.Judge.Key > 0 {
		fmt.Fprintf(&b, " judge=key(%d)", p.Judge.Key)
	} else {
		fmt.Fprintf(&b, " judge=(%d,%d)", p.Judge.X, p.Judge.Y)
	}
	fmt.Fprintf(&b, " bar=%v", p.Bar.Roi)
	fmt.Fprintf(&b, " cursor=%s", p.Colors.Cursor)
	fmt.Fprintf(&b, " blue=%s", p.Colors.Blue)
	fmt.Fprintf(&b, " yellow=%s", p.Colors.Yellow)
	fmt.Fprintf(&b, " green=%s", p.Colors.Green)
	fmt.Fprintf(&b, " next=%s->{%s|%s}", p.Next.Node, p.Next.Cast, p.Next.Finish)
	fmt.Fprintf(&b, " poll=%dms comp=%dms yellow_wait=%dms extra=%dms",
		p.Timing.PollMS, p.Timing.InputCompMS, p.Timing.YellowMaxWaitMS, p.Timing.YellowExtraMS)
	return b.String()
}

// settle sleeps for the given number of milliseconds; non-positive = no wait.
func settle(ms int) {
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}
