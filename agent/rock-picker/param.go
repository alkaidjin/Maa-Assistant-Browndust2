package main

import "time"

// rockRow is one resource-bar row: the cave node to hand control to, a short
// label for user-facing messages, and the screen ROI holding that row's number.
type rockRow struct {
	Node  string `json:"node"`
	Label string `json:"label"`
	Roi   []int  `json:"roi"`
}

// leastRockParam is `custom_recognition_param` as written in
// resource/pipeline/HuntingArea.json. Keeping both the ROIs and the retry timing
// in JSON rather than in this binary means coordinates can be corrected without
// rebuilding the agent.
type leastRockParam struct {
	// Rows are read top-to-bottom, in the order the game draws the resource bar.
	Rows []rockRow `json:"rows"`
	// SettleMS waits once before the first read, giving the game time to paint
	// the resource bar after 圣石洞穴 is clicked. 0 disables the wait.
	SettleMS int `json:"settle_ms"`
	// Attempts is how many times the five rows are read (each on a fresh frame)
	// before the task is aborted.
	Attempts int `json:"attempts"`
	// IntervalMS is the pause between attempts.
	IntervalMS int `json:"interval_ms"`
}

const (
	// leastRockRecognitionName must match the pipeline's
	// recognition.param.custom_recognition value.
	leastRockRecognitionName = "LeastRockPicker"

	defaultSettleMS   = 800
	defaultAttempts   = 3
	defaultIntervalMS = 600

	// focusNodeName is a throwaway node used to carry a `focus` payload to the UI.
	focusNodeName = "_ROCK_PICKER_FOCUS_"
)

// logDisplay shows a message in the MXU run log only.
var logDisplay = []string{"log"}

// errorDisplay pushes a failure to every channel a user might be watching: the
// run log, an in-app toast, and a system notification (so an unattended run
// still reaches them). `modal` is deliberately not used — it would block the
// task queue while nobody is at the keyboard.
var errorDisplay = []string{"log", "toast", "notification"}

// settle sleeps for the given number of milliseconds; non-positive = no wait.
func settle(ms int) {
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
}
