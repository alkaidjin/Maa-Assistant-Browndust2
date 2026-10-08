package main

import (
	"image"
	"strings"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// focus surfaces a message in MXU via a throwaway DoNothing node carrying a
// focus event. Mirrors the fishing agent's focus() helper.
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

// clickPoint clicks a single 720P-design coordinate. Coordinates are scaled to
// the real frame size using the scale derived from the current capture.
func clickPoint(ctx *maa.Context, ctrl *maa.Controller, x, y int) error {
	box := maa.Rect{x, y, 1, 1}
	_, err := ctx.RunActionDirect(maa.ActionTypeClick, &maa.ClickParam{
		Target: maa.NewTargetRect(box),
	}, box, nil)
	return err
}

// runOCR runs OCR over the given ROI (720P design space) and returns all
// recognized text boxes. The ROI is scaled to the real frame.
func runOCR(ctx *maa.Context, img image.Image, roi maa.Rect, expected ...string) ([]*maa.OCRResult, error) {
	param := &maa.OCRParam{
		ROI:       maa.NewTargetRect(roi),
		Threshold: 0.3,
	}
	if len(expected) > 0 {
		param.Expected = expected
	}
	detail, err := ctx.RunRecognitionDirect(maa.RecognitionTypeOCR, param, img)
	if err != nil {
		return nil, err
	}
	if detail == nil || detail.Results == nil {
		return nil, nil
	}
	out := make([]*maa.OCRResult, 0, len(detail.Results.All))
	for _, r := range detail.Results.All {
		if ocr, ok := r.AsOCR(); ok {
			out = append(out, ocr)
		}
	}
	return out, nil
}

// findTextBox searches OCR results for a box whose text contains all of the
// given keywords (case-insensitive substring match). Returns the first match
// or nil.
func findTextBox(results []*maa.OCRResult, keywords ...string) *maa.OCRResult {
	if len(results) == 0 || len(keywords) == 0 {
		return nil
	}
	for _, r := range results {
		text := strings.ToLower(r.Text)
		matched := true
		for _, kw := range keywords {
			if kw == "" {
				continue
			}
			if !strings.Contains(text, strings.ToLower(kw)) {
				matched = false
				break
			}
		}
		if matched {
			return r
		}
	}
	return nil
}

// matchTemplate runs TemplateMatch on the image with the given template path
// (relative to resource/image) and threshold. Returns the best hit box or
// (zero, false).
func matchTemplate(ctx *maa.Context, img image.Image, template string, roi maa.Rect, threshold float64) (maa.Rect, bool) {
	param := &maa.TemplateMatchParam{
		ROI:       maa.NewTargetRect(roi),
		Template:  []string{template},
		Threshold: []float64{threshold},
	}
	detail, err := ctx.RunRecognitionDirect(maa.RecognitionTypeTemplateMatch, param, img)
	if err != nil {
		return maa.Rect{}, false
	}
	if detail == nil || detail.Results == nil || detail.Results.Best == nil {
		return maa.Rect{}, false
	}
	tm, ok := detail.Results.Best.AsTemplateMatch()
	if !ok {
		return maa.Rect{}, false
	}
	return tm.Box, true
}

// waitStable waits until the screen within roi stops changing for `stable`
// duration, up to `timeout`. Returns nil on success.
func waitStable(ctx *maa.Context, roi maa.Rect, stable, timeout time.Duration) error {
	return ctx.WaitFreezes(stable, &roi, nil)
}

// stopping reports whether the user has pressed stop.
func stopping(ctx *maa.Context) bool {
	t := ctx.GetTasker()
	return t != nil && t.Stopping()
}
