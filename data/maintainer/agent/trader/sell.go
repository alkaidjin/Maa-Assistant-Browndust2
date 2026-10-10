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

// Sell-loop ROIs in 720P design space, calibrated from 1080p gameplay video
// (2026-10-10). Page layout: left rail holds the 购买/出售 tabs plus a
// scrollable shop list (card thumbnail + game name, ~53px row pitch, rows
// span x≈122..258); the item grid is 4 columns starting at x≈262, grouped by
// category headers; the sell confirmation popup spans x255..770 / y158..418.
var (
	// shopTitleROI covers the page header title ("购买" / "出售").
	shopTitleROI = maa.Rect{120, 8, 140, 50}
	// sellTabPoint is the "出售" tab on the left rail.
	sellTabPoint = image.Point{113, 165}
	// shopListROI covers the name-text column of the left shop list
	// (thumbnails at x130..195, names at x200..320). Two lines per entry
	// ("剧情游戏卡 N" + game name).
	shopListROI = maa.Rect{203, 70, 98, 642}
	// shopEntryPointX is a safe click X inside a shop-list row (over the
	// thumbnail/name area).
	shopEntryPointX = 210
	// priceListButton is the "价目表" button on the right side of the sell
	// page. Clicking it opens a popup listing all shops' peak-price items.
	priceListButton = image.Point{957, 670}
	// priceListClose is the X button that dismisses the price-list popup.
	priceListClose = image.Point{987, 167}
	// itemListROI covers the 4-column item grid.
	itemListROI = maa.Rect{255, 70, 760, 555}
	// popupTitleROI covers the item name in the sell confirmation popup.
	// Title text sits at 720P y≈205..235 (popup top is y≈158).
	popupTitleROI = maa.Rect{260, 198, 170, 45}
	// popupContentROI covers the 拥有/可购买 count line and the price row.
	popupContentROI = maa.Rect{255, 270, 480, 150}
	// minButton / maxButton / confirmButton / closeButton (popup).
	minButton     = image.Point{451, 481}
	maxButton     = image.Point{603, 481}
	confirmButton = image.Point{874, 484}
	closeButton   = image.Point{947, 215}
	// sliderRect is the quantity slider track in the popup.
	sliderRect = maa.Rect{295, 348, 192, 14}

	ownedPattern     = regexp.MustCompile(`拥有\s*([0-9]+)`)
	availablePattern = regexp.MustCompile(`可购买\s*([0-9]+)`)
)

// shopGroup batches one shop's sell candidates so each shop is entered once.
type shopGroup struct {
	shop  string
	items []SellItem
}

// groupByShop merges same-shop items while preserving calendar order.
func groupByShop(sell []SellItem) []shopGroup {
	var groups []shopGroup
	idx := make(map[string]int)
	for _, it := range sell {
		if i, ok := idx[it.Shop]; ok {
			groups[i].items = append(groups[i].items, it)
			continue
		}
		idx[it.Shop] = len(groups)
		groups = append(groups, shopGroup{shop: it.Shop, items: []SellItem{it}})
	}
	return groups
}

// runSellLoop sells today's peak-price items. Items are grouped by shop; the
// left shop list is scanned with ALL remaining target shops matched at every
// step (parallel recognition), so whichever shop is visible gets sold
// immediately instead of hunting one shop at a time. Verification is
// calendar-driven: OCR item name in the grid + popup title match. (The pink
// "+N%" cell badge is a generic premium indicator present on most items, so
// it cannot discriminate peak items and is not checked.)
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
	time.Sleep(1500 * time.Millisecond)

	// Bounce the price-list popup: open it (forces the shop list to render),
	// then close it. This is a workaround for the left shop list sometimes
	// being blank on first entry to the sell page.
	bouncePriceList(ctx, ctrl)
	time.Sleep(1500 * time.Millisecond)

	// Pass 1 scans the list downward, pass 2 back upward; each step OCRs the
	// list once and checks every remaining target shop against the results.
	remaining := groupByShop(sell)
	sold := 0
	for phase := 0; phase < 2 && len(remaining) > 0; phase++ {
		delta := 1
		if phase == 1 {
			delta = -1
		}
		for step := 0; step < 12 && len(remaining) > 0; step++ {
			if stopping(ctx) {
				remaining = nil
				break
			}
			frame, err := capture(ctrl, nil)
			if err != nil {
				logf("  capture failed: %v", err)
				return sold
			}
			if boxes, err := runOCR(ctx, frame, shopListROI); err != nil {
				logf("  shop list OCR failed: %v", err)
			} else {
				for _, b := range boxes {
					gi := matchRemainingShop(remaining, b.Text)
					if gi < 0 {
						continue
					}
					g := remaining[gi]
					logf("shop %s: %d candidate(s)", g.shop, len(g.items))
					if err := clickShopEntry(ctx, ctrl, b, g.shop, pageSignature(frame)); err != nil {
						focus(ctx, fmt.Sprintf("跑商：进入商店「%s」失败（%v）", g.shop, err), errorDisplay)
					} else {
						for _, it := range g.items {
							if stopping(ctx) {
								break
							}
							logf("sell candidate: %s @ %s", it.Item, shopLabel(it.Shop))
							if ok := sellOneItem(ctx, ctrl, it, p); ok {
								sold++
							}
						}
					}
					remaining = append(remaining[:gi], remaining[gi+1:]...)
					break // the view changed; re-OCR before further clicks
				}
			}
			if len(remaining) == 0 || step == 11 {
				break
			}
			before := listSignature(frame)
			scrollList(ctx, delta)
			frame2, err := capture(ctrl, nil)
			if err != nil {
				logf("  capture failed: %v", err)
				return sold
			}
			if listSignature(frame2) == before {
				logf("  shop list edge reached (phase %d)", phase)
				break
			}
		}
	}
	if len(remaining) > 0 {
		names := make([]string, 0, len(remaining))
		for _, g := range remaining {
			names = append(names, g.shop)
		}
		focus(ctx, fmt.Sprintf("跑商：左侧列表未找到商店「%s」", strings.Join(names, "、")), errorDisplay)
	}
	logf("sell loop done: %d/%d sold", sold, len(sell))
	return sold
}

// matchRemainingShop returns the index of the group whose shop matches the
// OCR text, or -1. Every remaining group is checked per OCR line, so all
// target shop names are recognized together rather than one hunt at a time.
func matchRemainingShop(groups []shopGroup, text string) int {
	for i, g := range groups {
		if shopMatches(text, g.shop) {
			return i
		}
	}
	return -1
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

// shopMatches reports whether an OCR line from the shop list refers to the
// given shop. Tolerates OCR truncation of long names (e.g. 被遗忘的战争).
func shopMatches(ocrText, shop string) bool {
	text := strings.TrimSpace(ocrText)
	if text == "" {
		return false
	}
	if strings.Contains(text, shop) {
		return true
	}
	// OCR may drop trailing runes of long names; accept a prefix of the shop
	// name that is at least 3 runes long to avoid tiny-fragment false hits.
	if len([]rune(text)) >= 3 && strings.HasPrefix(shop, text) {
		return true
	}
	return false
}

// bouncePriceList opens the "价目表" popup (which forces the left shop list
// to render) and then closes it. Each action is separated by 1500ms so the
// UI has time to settle.
func bouncePriceList(ctx *maa.Context, ctrl *maa.Controller) {
	logf("  opening price list popup")
	_ = clickPoint(ctx, ctrl, priceListButton.X, priceListButton.Y)
	time.Sleep(1500 * time.Millisecond)
	logf("  closing price list popup")
	_ = clickPoint(ctx, ctrl, priceListClose.X, priceListClose.Y)
	time.Sleep(1500 * time.Millisecond)
}

// scrollList swipes the left shop list. delta > 0 reveals entries below the
// current view (finger moves up); delta < 0 scrolls back toward the top.
// The swipe stays within the list's scrollable band (y≈100..650).
func scrollList(ctx *maa.Context, delta int) {
	beginY, endY := 590, 150
	if delta < 0 {
		beginY, endY = 150, 590
	}
	_, _ = ctx.RunActionDirect(maa.ActionTypeSwipe, &maa.SwipeParam{
		Begin:    maa.NewTargetRect(maa.Rect{shopEntryPointX, beginY, 1, 1}),
		End:      []maa.Target{maa.NewTargetRect(maa.Rect{shopEntryPointX, endY, 1, 1})},
		Duration: []time.Duration{500 * time.Millisecond},
		EndHold:  []time.Duration{200 * time.Millisecond},
	}, maa.Rect{}, nil)
	time.Sleep(900 * time.Millisecond)
}

// scrollItemGrid swipes the right-side sell item grid. down=true reveals items
// below the current view (finger moves up); down=false scrolls back up.
// Coordinates verified on-device: begin (516,298) ↔ end (516,136), holding
// 0.5s at the end.
func scrollItemGrid(ctx *maa.Context, down bool) {
	beginY, endY := 298, 136
	if !down {
		beginY, endY = 136, 298
	}
	_, _ = ctx.RunActionDirect(maa.ActionTypeSwipe, &maa.SwipeParam{
		Begin:    maa.NewTargetRect(maa.Rect{516, beginY, 1, 1}),
		End:      []maa.Target{maa.NewTargetRect(maa.Rect{516, endY, 1, 1})},
		Duration: []time.Duration{500 * time.Millisecond},
		EndHold:  []time.Duration{500 * time.Millisecond},
	}, maa.Rect{}, nil)
	time.Sleep(1200 * time.Millisecond)
}

// listSignature is a coarse change detector over the shop-list region,
// used to detect that the list stopped scrolling (top/bottom reached).
func listSignature(img image.Image) string {
	var sum uint64
	for y := 60; y < 630; y += 15 {
		for x := 125; x < 290; x += 15 {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += uint64(r>>8) + uint64(g>>8) + uint64(bl>>8)
		}
	}
	return strconv.FormatUint(sum, 16)
}

// clickShopEntry clicks the found shop row and waits for its grid to load.
// sigBefore is the page signature captured while the list was scanned; the
// grid is considered loaded once the page changes (or after a fixed grace
// period when the clicked shop was already active).
func clickShopEntry(ctx *maa.Context, ctrl *maa.Controller, box *maa.OCRResult, shop string, sigBefore string) error {
	x := shopEntryPointX
	y := box.Box.Y() + box.Box.Height()/2
	logf("  shop %q entry at (%d,%d), clicking", shop, x, y)
	if err := clickPoint(ctx, ctrl, x, y); err != nil {
		return err
	}
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if stopping(ctx) {
			return fmt.Errorf("stopped")
		}
		frame, err := capture(ctrl, nil)
		if err != nil {
			return err
		}
		if sigBefore == "" || pageSignature(frame) != sigBefore {
			break
		}
		time.Sleep(300 * time.Millisecond)
	}
	time.Sleep(800 * time.Millisecond)
	return waitGridReady(ctx, ctrl)
}

// waitGridReady waits until the item grid shows OCR text (list loaded).
func waitGridReady(ctx *maa.Context, ctrl *maa.Controller) error {
	deadline := time.Now().Add(4 * time.Second)
	time.Sleep(1200 * time.Millisecond) // transition animation
	for time.Now().Before(deadline) {
		if stopping(ctx) {
			return fmt.Errorf("stopped")
		}
		frame, err := capture(ctrl, nil)
		if err != nil {
			return err
		}
		boxes, err := runOCR(ctx, frame, itemListROI)
		if err == nil && len(boxes) >= 2 {
			return nil
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("item grid did not load")
}

// ensureSellTab clicks the sell tab and waits for the title to read "出售".
func ensureSellTab(ctx *maa.Context, ctrl *maa.Controller) error {
	const maxAttempts = 3
	for i := 0; i < maxAttempts; i++ {
		frame, err := capture(ctrl, nil)
		if err != nil {
			return err
		}
		boxes, err := runOCR(ctx, frame, shopTitleROI)
		if err == nil && findTextBox(boxes, "出售") != nil {
			return nil
		}
		logf("sell tab not active, clicking (%d attempt)", i+1)
		if err := clickPoint(ctx, ctrl, sellTabPoint.X, sellTabPoint.Y); err != nil {
			return err
		}
		if err := waitGridReady(ctx, ctrl); err != nil {
			logf("  grid wait after tab switch: %v", err)
		}
	}
	return fmt.Errorf("sell tab not confirmed after %d attempts", maxAttempts)
}

// sellOneItem handles a single item: find it in the grid, open the popup,
// verify the popup title, set quantity, confirm.
//
// Item search within a shop: OCR the current view, then scroll down up to 3
// times (each swipe followed by OCR), then scroll back up up to 4 times. If
// the item is never seen, the shop has nothing to sell and we move on.
func sellOneItem(ctx *maa.Context, ctrl *maa.Controller, it SellItem, p tradeParam) bool {
	var nameBox *maa.OCRResult

	findInView := func() bool {
		frame, err := capture(ctrl, nil)
		if err != nil {
			logf("  capture failed: %v", err)
			return false
		}
		boxes, err := runOCR(ctx, frame, itemListROI)
		if err != nil {
			logf("  OCR grid failed: %v", err)
			return false
		}
		nameBox = findSellItem(boxes, it)
		return nameBox != nil
	}

	// Current view.
	if findInView() {
		logf("  found %q at box %v (current view)", it.Item, nameBox.Box)
	} else {
		// Scroll down up to 3 times, OCR after each.
		for i := 0; i < 3; i++ {
			logf("  item %q not visible, scrolling grid down (%d/3)", it.Item, i+1)
			scrollItemGrid(ctx, true)
			if findInView() {
				logf("  found %q at box %v (down swipe %d)", it.Item, nameBox.Box, i+1)
				break
			}
		}
	}
	if nameBox == nil {
		// Scroll back up up to 4 times, OCR after each.
		for i := 0; i < 4; i++ {
			logf("  item %q not visible, scrolling grid up (%d/4)", it.Item, i+1)
			scrollItemGrid(ctx, false)
			if findInView() {
				logf("  found %q at box %v (up swipe %d)", it.Item, nameBox.Box, i+1)
				break
			}
		}
	}
	if nameBox == nil {
		logf("  item %q not found in grid (3 down + 4 up swipes), skip", it.Item)
		return false
	}

	if p.DryRun {
		logf("  [dry-run] would sell %q", it.Item)
		return false
	}

	// 2. Click the item to open the sell popup.
	clickX := nameBox.Box.X() + nameBox.Box.Width()/2
	clickY := nameBox.Box.Y() + nameBox.Box.Height()/2
	if err := clickPoint(ctx, ctrl, clickX, clickY); err != nil {
		logf("  click item failed: %v", err)
		return false
	}
	time.Sleep(1500 * time.Millisecond)

	// 3. Verify popup title and owned count.
	popupFrame, err := capture(ctrl, nil)
	if err != nil {
		return false
	}
	titleBoxes, err := runOCR(ctx, popupFrame, popupTitleROI)
	if err != nil || findSellItem(titleBoxes, it) == nil {
		logf("  popup title mismatch for %q, close and skip", it.Item)
		clickPoint(ctx, ctrl, closeButton.X, closeButton.Y)
		time.Sleep(1500 * time.Millisecond)
		return false
	}

	contentBoxes, err := runOCR(ctx, popupFrame, popupContentROI)
	if err != nil {
		return false
	}
	owned := readOwnedCount(contentBoxes)
	logf("  popup confirmed: owned=%d", owned)
	sigBefore := pageSignature(popupFrame)

	// 4. Set quantity. Per-item reserve (it.Reserve > 0) overrides the global
	//    reserve_count when selling in reserve mode.
	reserve := p.ReserveCount
	if it.Reserve > 0 {
		reserve = it.Reserve
	}
	switch p.SellMode {
	case "max":
		clickPoint(ctx, ctrl, maxButton.X, maxButton.Y)
		time.Sleep(1500 * time.Millisecond)
	case "reserve":
		if owned <= reserve {
			logf("  owned %d <= reserve %d, skip", owned, reserve)
			clickPoint(ctx, ctrl, closeButton.X, closeButton.Y)
			time.Sleep(1500 * time.Millisecond)
			return false
		}
		// Reserve: drag slider to (owned-reserve)/owned fraction.
		setSliderQuantity(ctx, ctrl, owned-reserve, owned)
		time.Sleep(1500 * time.Millisecond)
	default: // min
		clickPoint(ctx, ctrl, minButton.X, minButton.Y)
		time.Sleep(1500 * time.Millisecond)
	}

	// 5. Confirm and wait for completion.
	if err := clickPoint(ctx, ctrl, confirmButton.X, confirmButton.Y); err != nil {
		return false
	}
	if waitSellComplete(ctx, ctrl, sigBefore) {
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
func setSliderQuantity(ctx *maa.Context, ctrl *maa.Controller, amount, owned int) {
	if owned <= 0 || amount <= 0 {
		return
	}
	frac := float64(amount) / float64(owned)
	if frac > 1 {
		frac = 1
	}
	handleX := sliderRect.X() + int(float64(sliderRect.Width())*frac)
	handleY := sliderRect.Y() + sliderRect.Height()/2
	_, _ = ctx.RunActionDirect(maa.ActionTypeSwipe, &maa.SwipeParam{
		Begin:    maa.NewTargetRect(maa.Rect{sliderRect.X(), handleY, 1, 1}),
		End:      []maa.Target{maa.NewTargetRect(maa.Rect{handleX, handleY, 1, 1})},
		Duration: []time.Duration{400 * time.Millisecond},
		EndHold:  []time.Duration{150 * time.Millisecond},
	}, maa.Rect{}, nil)
	time.Sleep(300 * time.Millisecond)
}

// waitSellComplete waits for the sell toast or a page-signature change (the
// popup closing and the grid refreshing), up to 8 seconds. sigBefore is the
// signature captured while the popup was still open.
func waitSellComplete(ctx *maa.Context, ctrl *maa.Controller, sigBefore string) bool {
	deadline := time.Now().Add(8 * time.Second)
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
		// Popup closed / grid refreshed → page signature changed.
		if sigBefore != "" && pageSignature(frame) != sigBefore {
			time.Sleep(600 * time.Millisecond) // let the list settle
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

// pageSignature returns a coarse hash of the item-list area to detect that
// the popup has closed and the list refreshed.
func pageSignature(img image.Image) string {
	var sum uint64
	for y := 100; y < 400; y += 20 {
		for x := 200; x < 900; x += 40 {
			r, g, bl, _ := img.At(x, y).RGBA()
			sum += uint64(r>>8) + uint64(g>>8) + uint64(bl>>8)
		}
	}
	return strconv.FormatUint(sum, 16)
}
