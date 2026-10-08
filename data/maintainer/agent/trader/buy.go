package main

import (
	"fmt"
	"image"
	"strings"
	"time"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

// Buy-flow ROIs / points in 720P design space.
var (
	buyTabPoint   = image.Point{115, 120}
	bargainPoint  = image.Point{127, 600}
	bargainConfirm = image.Point{698, 435}
	// buyAllROI covers the "购买全部收藏" button area (bottom-center of shop).
	buyAllROI = maa.Rect{500, 560, 300, 80}
	// buyConfirmPoint is the confirm button in the buy-all popup.
	buyConfirmPoint = image.Point{640, 460}
)

// runBuyFlow handles bargain + buy-all-favorites. Buying is favorites-driven:
// the user stars the items they want to stock up on, and the agent clicks
// "购买全部收藏". Favorite alignment is only done when RebuildFavorites !=
// "never" (not yet implemented in v1).
func runBuyFlow(ctx *maa.Context, ctrl *maa.Controller, p tradeParam) bool {
	if !p.EnableBuy {
		logf("buy flow skipped (enable_buy=false)")
		return false
	}

	if err := ensureBuyTab(ctx, ctrl); err != nil {
		focus(ctx, fmt.Sprintf("跑商：无法进入购买页（%v），已跳过采购环节", err), errorDisplay)
		return false
	}

	if p.Bargain {
		if err := runBargain(ctx, ctrl); err != nil {
			logf("bargain step failed, continuing to buy anyway: %v", err)
		}
	}

	if p.RebuildFavorites != "never" {
		logf("favorite alignment requested (%q) — not yet implemented in v1, skipping", p.RebuildFavorites)
	}

	if p.DryRun {
		logf("[dry-run] would click 购买全部收藏 (favorites set by user)")
		return true
	}

	return buyAllFavorites(ctx, ctrl)
}

// ensureBuyTab clicks the buy tab and verifies the title reads "购买".
func ensureBuyTab(ctx *maa.Context, ctrl *maa.Controller) error {
	const maxAttempts = 3
	for i := 0; i < maxAttempts; i++ {
		frame, err := capture(ctrl, nil)
		if err != nil {
			return err
		}
		boxes, err := runOCR(ctx, frame, shopTitleROI, "购买")
		if err == nil && findTextBox(boxes, "购买") != nil {
			return nil
		}
		logf("buy tab not active, clicking (%d attempt)", i+1)
		if err := clickPoint(ctx, ctrl, buyTabPoint.X, buyTabPoint.Y); err != nil {
			return err
		}
		time.Sleep(800 * time.Millisecond)
	}
	return fmt.Errorf("buy tab not confirmed after %d attempts", maxAttempts)
}

// runBargain executes the bargain skill: click bargain entry, confirm the
// discount popup, wait until back in the shop.
func runBargain(ctx *maa.Context, ctrl *maa.Controller) error {
	logf("running bargain")
	if err := clickPoint(ctx, ctrl, bargainPoint.X, bargainPoint.Y); err != nil {
		return err
	}
	time.Sleep(700 * time.Millisecond)

	// Confirm the discount popup.
	if err := clickPoint(ctx, ctrl, bargainConfirm.X, bargainConfirm.Y); err != nil {
		return err
	}
	time.Sleep(1200 * time.Millisecond)

	// Verify we're back at the buy shop (title = 购买).
	frame, err := capture(ctrl, nil)
	if err != nil {
		return err
	}
	boxes, err := runOCR(ctx, frame, shopTitleROI, "购买")
	if err != nil || findTextBox(boxes, "购买") == nil {
		return fmt.Errorf("bargain did not return to buy shop")
	}
	logf("bargain done")
	return nil
}

// buyAllFavorites clicks "购买全部收藏", confirms the popup, and waits for
// completion.
func buyAllFavorites(ctx *maa.Context, ctrl *maa.Controller) bool {
	frame, err := capture(ctrl, nil)
	if err != nil {
		return false
	}

	boxes, err := runOCR(ctx, frame, buyAllROI, "购买全部收藏")
	if err != nil {
		logf("OCR buy-all failed: %v", err)
		return false
	}
	btn := findTextBox(boxes, "购买全部收藏")
	if btn == nil {
		logf("购买全部收藏 button not found, skip buy")
		return false
	}
	logf("buy-all button at %v", btn.Box)

	cx := btn.Box.X() + btn.Box.Width()/2
	cy := btn.Box.Y() + btn.Box.Height()/2
	if err := clickPoint(ctx, ctrl, cx, cy); err != nil {
		return false
	}
	time.Sleep(800 * time.Millisecond)

	// Confirm the purchase popup.
	if err := clickPoint(ctx, ctrl, buyConfirmPoint.X, buyConfirmPoint.Y); err != nil {
		return false
	}

	// Wait for completion: toast or page change.
	deadline := time.Now().Add(8 * time.Second)
	var sig string
	for time.Now().Before(deadline) {
		if stopping(ctx) {
			return false
		}
		f, err := capture(ctrl, nil)
		if err != nil {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		toast, _ := runOCR(ctx, f, maa.Rect{400, 300, 480, 120})
		for _, b := range toast {
			if strings.Contains(b.Text, "完成") || strings.Contains(b.Text, "购买") {
				logf("buy-all completed")
				return true
			}
		}
		cur := pageSignature(f)
		if sig == "" {
			sig = cur
		} else if cur != sig {
			logf("buy-all page changed (popup closed)")
			return true
		}
		time.Sleep(300 * time.Millisecond)
	}
	logf("buy-all not confirmed within timeout")
	return false
}
