package main

import (
	"encoding/json"
	"fmt"
	"image"

	maa "github.com/MaaXYZ/maa-framework-go/v4"
)

const (
	tradeActionName = "TradeRun"
	focusNodeName   = "TradeFocus"

	// design space: 1280x720, same as MXU pipeline recognition layer.
	designW = 1280
	designH = 720
)

// display channels for focus messages.
var (
	logDisplay   = []string{"log"}
	errorDisplay = []string{"toast", "notification", "log"}
)

// tradeParam is the JSON payload passed via `custom_action_param` on the
// Trade_Start pipeline node. All coordinates are in the 720P design space.
type tradeParam struct {
	// DryRun, when true, only prints the planned buy/sell list and does not
	// click anything that changes inventory/gold. Default false.
	DryRun bool `json:"dry_run"`

	// SellMode controls how many of each peak item are sold.
	// "min" (default) sells exactly 1; "max" sells all; "reserve" keeps
	// ReserveCount and sells the rest.
	SellMode string `json:"sell_mode"`

	// ReserveCount is used when SellMode == "reserve".
	ReserveCount int `json:"reserve_count"`

	// CalendarPath overrides the bundled price_calendar.v1.json location.
	// Empty = look next to the executable / in resource dir.
	CalendarPath string `json:"calendar_path"`

	// OnlineUpdate enables fetching the latest calendar from the update URL
	// before reading. Default false in v1 (opt-in).
	OnlineUpdate bool `json:"online_update"`

	// OnlineURL overrides the default calendar update endpoint.
	OnlineURL string `json:"online_url"`

	// EnableSell / EnableBuy let the user run only one half. Both default true.
	EnableSell bool `json:"enable_sell"`
	EnableBuy  bool `json:"enable_buy"`

	// Bargain enables the bargain step in the buy flow. Default true.
	Bargain bool `json:"bargain"`

	// RebuildFavorites controls when the favorite-star alignment runs:
	// "never" (default in v1 — user sets stars manually), "weekly", "always".
	RebuildFavorites string `json:"rebuild_favorites"`
}

func defaultTradeParam() tradeParam {
	return tradeParam{
		DryRun:           false,
		SellMode:         "min",
		ReserveCount:     0,
		EnableSell:       true,
		EnableBuy:        true,
		Bargain:          true,
		RebuildFavorites: "never",
	}
}

func parseTradeParam(raw string) (tradeParam, error) {
	p := defaultTradeParam()
	if raw == "" {
		return p, nil
	}
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return p, fmt.Errorf("parse trade param: %w", err)
	}
	switch p.SellMode {
	case "", "min", "max", "reserve":
	default:
		return p, fmt.Errorf("invalid sell_mode %q (want min|max|reserve)", p.SellMode)
	}
	switch p.RebuildFavorites {
	case "", "never", "weekly", "always":
	default:
		return p, fmt.Errorf("invalid rebuild_favorites %q (want never|weekly|always)", p.RebuildFavorites)
	}
	return p, nil
}

// capture grabs the current frame into buf (reused) and returns it.
func capture(ctrl *maa.Controller, buf *image.RGBA) (*image.RGBA, error) {
	if !ctrl.PostScreencap().Wait().Success() {
		return nil, fmt.Errorf("screencap failed")
	}
	return ctrl.CacheImageInto(buf)
}
