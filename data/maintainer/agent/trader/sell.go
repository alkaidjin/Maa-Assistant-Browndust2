package main

import (
	"fmt"
	"image"
	"regexp"
	"strconv"
	"strings"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// Sell-loop ROIs in 720P design space (scaled from ok-bd2 1920x1080).
var (
	// shopTitleROI covers the mode title ("购买" / "出售") top-left.
	shopTitleROI = maa.Rect{140, 12, 120, 50}
	// sellTabPoint is the "出售" tab on the left shop menu.
	sellTabPoint = image.Point{115, 167}
	// itemListROI covers the scrollable item list where OCR finds item names.
	itemListROI = maa.Rect{200, 90, 900, 560}
	// popupTitleROI covers the item name in the sell confirmation popup.
	popupTitleROI = maa.Rect{320, 210, 200, 40}
	// popupContentROI covers the owned/available text inside the popup.
	popupContentROI = maa.Rect{310, 190, 660, 340}
	// markerROITpl is the ROI to the LEFT of the item-name box where the
	// ↑120% peak marker is drawn. Width/height are offsets applied to the
	// OCR box: left border - markerW, top - markerPad.
	markerW   = 150
	markerPad = 12
	// minButton / maxButton / confirmButton coordinates in 720P space.
	minButton     = image.Point{451, 481}
	maxButton     = image.Point{602, 483}
	confirmButton = image.Point{875, 485}
	closeButton   = image.Point{947, 215}

	ownedPattern     = regexp.MustCompile(`拥有\s*([0-9]+)`)
	availablePattern = regexp.MustCompile(`可购买\s*([0-9]+)`)
)

// runSellLoop iterates today's peak-price items and sells each one after
// triple verification: OCR item name found + ↑120% marker present + popup
// confirms the item. Returns the number of items successfully sold.
func runSellLoop(ctx *maa.Context, ctrl *maa.Controller, sell []SellItem, p tradeParam) int {
	if !p.EnableSell || len(sell) == 0 {
		logf("sell loop skipped (enable_sell=%v, candidates=%d)", p.EnableSell, len(sell))
		return 0
	}

	// Ensure we are on the SELL tab.
	if err := ensureSellTab(ctx, ctrl); err != nil {
		focus(ctx, fmt.Sprintf("跑商：无法进入出售页（%v），已跳过出售环节", err), errorDisplay)
		return 0
	}

	sold := 0
	for _, it := range sell {
		if stopping(ctx) {
			break
		}
		logf("sell candidate: %s @ %s", it.Item, shopLabel(it.Shop))
		if ok := sellOneItem(ctx, ctrl, it, p); ok {
			sold++
		}
	}
	logf("sell loop done: %d/%d sold", sold, len(sell))
	return sold
}

// findSellItem searches OCR results for a box matching the item by primary
// name or any alias (case-insensitive substring). Returns the first match.
func findSellItem(results []*maa.OCRResult, it SellItem) *maa.OCRResult {
	for _, r := range results {
		if it.matchName(r.Text) {
			return r
		}
	}
	return nil
}

// ensureSellTab clicks the sell tab and waits for the title to read "出售".
func ensureSellTab(ctx *maa.Context, ctrl *maa.Controller) error {
	const maxAttempts = 3
	for i := 0; i < maxAttempts; i++ {
		frame, err := capture(ctrl, nil)
		if err != nil {
			return err
		}
		boxes, err := runOCR(ctx, frame, shopTitleROI, "出售")
		if err == nil && findTextBox(boxes, "出售") != nil {
			return nil
		}
		logf("sell tab not active, clicking (%d attempt)", i+1)
		if err := clickPoint(ctx, ctrl, sellTabPoint.X, sellTabPoint.Y); err != nil {
			return err
		}
		time.Sleep(800 * time.Millisecond)
	}
	return fmt.Errorf("sell tab not confirmed after %d attempts", maxAttempts)
}

// sellOneItem handles a single item: find it in the list, open popup, verify
// ↑120% marker, set quantity, confirm.
func sellOneItem(ctx *maa.Context, ctrl *maa.Controller, it SellItem, p tradeParam) bool {
	frame, err := capture(ctrl, nil)
	if err != nil {
		logf("  capture failed: %v", err)
		return false
	}

	// 1. Find the item name via OCR over the list area (match primary name or
	//    any alias).
	boxes, err := runOCR(ctx, frame, itemListROI)
	if err != nil {
		logf("  OCR list failed: %v", err)
		return false
	}
	nameBox := findSellItem(boxes, it)
	if nameBox == nil {
		logf("  item %q not found in list, skip", it.Item)
		return false
	}
	logf("  found %q at box %v", it.Item, nameBox.Box)

	// 2. Verify the ↑120% peak marker to the left of the name.
	markerROI := maa.Rect{
		nameBox.Box.X() - markerW,
		nameBox.Box.Y() - markerPad,
		markerW,
		nameBox.Box.Height() + markerPad*2,
	}
	if markerROI.X() < 0 {
		markerROI[0] = 0
	}
	if _, ok := matchTemplate(ctx, frame, "TRADE/Sale120Marker.png", markerROI, 0.80); !ok {
		logf("  ↑120%% marker not found for %q, skip (calendar may be stale)", it.Item)
		return false
	}
	logf("  ↑120%% marker confirmed for %q", it.Item)

	if p.DryRun {
		logf("  [dry-run] would sell %q", it.Item)
		return false
	}

	// 3. Click the item to open the popup.
	clickX := nameBox.Box.X() + nameBox.Box.Width()/2
	clickY := nameBox.Box.Y() + nameBox.Box.Height()/2
	if err := clickPoint(ctx, ctrl, clickX, clickY); err != nil {
		logf("  click item failed: %v", err)
		return false
	}
	time.Sleep(700 * time.Millisecond)

	// 4. Verify popup title and owned count.
	popupFrame, err := capture(ctrl, nil)
	if err != nil {
		return false
	}
	titleBoxes, err := runOCR(ctx, popupFrame, popupTitleROI)
	if err != nil || findSellItem(titleBoxes, it) == nil {
		logf("  popup title mismatch for %q, close and skip", it.Item)
		clickPoint(ctx, ctrl, closeButton.X, closeButton.Y)
		time.Sleep(400 * time.Millisecond)
		return false
	}

	contentBoxes, err := runOCR(ctx, popupFrame, popupContentROI)
	if err != nil {
		return false
	}
	owned := readOwnedCount(contentBoxes)
	logf("  popup confirmed: owned=%d", owned)

	// 5. Set quantity. Per-item reserve (it.Reserve > 0) overrides the global
	//    reserve_count when selling in reserve mode.
	reserve := p.ReserveCount
	if it.Reserve > 0 {
		reserve = it.Reserve
	}
	switch p.SellMode {
	case "max":
		clickPoint(ctx, ctrl, maxButton.X, maxButton.Y)
		time.Sleep(300 * time.Millisecond)
	case "reserve":
		if owned <= reserve {
			logf("  owned %d <= reserve %d, skip", owned, reserve)
			clickPoint(ctx, ctrl, closeButton.X, closeButton.Y)
			time.Sleep(400 * time.Millisecond)
			return false
		}
		// Reserve: drag slider to (owned-reserve)/owned fraction.
		setSliderQuantity(ctx, ctrl, owned-reserve, owned)
	default: // min
		clickPoint(ctx, ctrl, minButton.X, minButton.Y)
		time.Sleep(300 * time.Millisecond)
	}

	// 6. Confirm and wait for completion.
	if err := clickPoint(ctx, ctrl, confirmButton.X, confirmButton.Y); err != nil {
		return false
	}
	if waitSellComplete(ctx, ctrl) {
		logf("  sold %q (owned was %d)", it.Item, owned)
		return true
	}
	logf("  sell of %q not confirmed", it.Item)
	return false
}

// readOwnedCount extracts the owned quantity from popup OCR results.
func readOwnedCount(boxes []*maa.OCRResult) int {
	for _, b := range boxes {
		if m := ownedPattern.FindStringSubmatch(b.Text); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				return n
			}
		}
	}
	return 0
}

// setSliderQuantity drags the sale slider to sell `amount` out of `owned`.
// Slider ROI: [368,431,240,24] in 720P. The handle starts at the left (min).
func setSliderQuantity(ctx *maa.Context, ctrl *maa.Controller, amount, owned int) {
	if owned <= 0 || amount <= 0 {
		return
	}
	slider := maa.Rect{368, 431, 240, 24}
	frac := float64(amount) / float64(owned)
	if frac > 1 {
		frac = 1
	}
	handleX := slider.X() + int(float64(slider.Width())*frac)
	handleY := slider.Y() + slider.Height()/2
	// Swipe from left end to the target position.
	_, _ = ctx.RunActionDirect(maa.ActionTypeSwipe, &maa.SwipeParam{
		Begin: maa.NewTargetRect(maa.Rect{slider.X(), handleY, 1, 1}),
		End:   []maa.Target{maa.NewTargetRect(maa.Rect{handleX, handleY, 1, 1})},
	}, maa.Rect{}, nil)
	time.Sleep(300 * time.Millisecond)
}

// waitSellComplete waits for the sell toast or a page signature change, up to
// 8 seconds. Returns true if the sale appears to have completed.
func waitSellComplete(ctx *maa.Context, ctrl *maa.Controller) bool {
	deadline := time.Now().Add(8 * time.Second)
	var sig string
	for time.Now().Before(deadline) {
		if stopping(ctx) {
			return false
		}
		frame, err := capture(ctrl, nil)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		// Toast appears center-screen; scan a middle band.
		boxes, err := runOCR(ctx, frame, maa.Rect{400, 300, 480, 120})
		if err == nil {
			for _, b := range boxes {
				if strings.Contains(b.Text, "完成") || strings.Contains(b.Text, "差价") {
					return true
				}
			}
		}
		// Page signature: if the list changed, the popup closed.
		cur := pageSignature(frame)
		if sig == "" {
			sig = cur
		} else if cur != sig {
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// pageSignature returns a coarse hash of the item-list area to detect that
// the popup has closed and the list refreshed.
func pageSignature(img image.Image) string {
	b := img.Bounds()
	var sum uint64
	for y := b.Min.Y + 100; y < b.Min.Y+400; y += 20 {
		for x := b.Min.X + 200; x < b.Min.X+900; x += 40 {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += uint64(r>>8) + uint64(g>>8) + uint64(bl>>8)
		}
	}
	return strconv.FormatUint(sum, 16)
}
