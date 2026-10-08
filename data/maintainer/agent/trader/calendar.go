package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// trimBOM strips a leading UTF-8 BOM (EF BB BF) from b.
func trimBOM(b []byte) []byte {
	return bytes.TrimPrefix(b, []byte{0xEF, 0xBB, 0xBF})
}

// PriceCalendar describes which items reach peak price on each day of the
// month. The calendar is generated from the deterministic market-price
// algorithm and item/pack database published by browndust2-db.souseha.com.
//
// Schema (v1):
//
//	{
//	  "schema_version": 1,
//	  "updated_at": "2026-10-03T12:00:00+08:00",
//	  "timezone": "Asia/Shanghai",
//	  "source_note": "...",
//	  "source_url": "https://browndust2-db.souseha.com/cn/market-data",
//	  "shops": ["血骑士","苍蓝魔女", ...],
//	  "days": {
//	    "3": [
//	      {"item":"獸肉","shop":"血骑士","aliases":["Beast Meat"],"reserve":0}
//	    ]
//	  }
//	}
//
// `shops` is the whitelist of valid shop names (CN). `days` is keyed by
// calendar day 1..31; each entry is an item at peak price today (a sell
// candidate). `aliases` gives OCR-friendly name variants; `reserve` is the
// per-item stock to keep when selling (overrides the global reserve_count
// when > 0).
type PriceCalendar struct {
	SchemaVersion int                `json:"schema_version"`
	UpdatedAt     string             `json:"updated_at"`
	Timezone      string             `json:"timezone"`
	SourceNote    string             `json:"source_note"`
	SourceURL     string             `json:"source_url"`
	Shops         []string           `json:"shops"`
	Days          map[string][]SellItem `json:"days"`
}

// SellItem is one peak-price item for the day.
type SellItem struct {
	Item    string   `json:"item"`
	Shop    string   `json:"shop"`
	Aliases []string `json:"aliases"`
	Reserve int      `json:"reserve"`
}

const (
	calendarSchemaVersion = 1
	// defaultOnlineURL points at the authoritative upstream calendar. The
	// bundled file is generated independently from souseha's raw DB (see
	// build script), so this URL is only used when the user explicitly opts
	// into online update to pick up upstream corrections.
	defaultOnlineURL = "https://raw.githubusercontent.com/GodRaymond233/ok-bd2/main/assets/map_trade/price_calendar.v1.json"
)

// shopLabel returns the display name for a shop. Since the calendar's `shop`
// field already stores the Chinese shop name, no mapping is needed.
func shopLabel(code string) string { return code }

// gameDay returns the in-game calendar day for the given local time.
//
// Shops reset at KST 00:00 = Beijing (UTC+8) 23:00 of the previous day.
// So at Beijing time 23:00 the "next" game day has already started.
func gameDay(now time.Time) int {
	t := now.In(time.FixedZone("UTC+8", 8*3600))
	if t.Hour() >= 23 {
		t = t.AddDate(0, 0, 1)
	}
	return t.Day()
}

// loadCalendar reads the bundled calendar (or a user override), optionally
// refreshing it from the online source first. Returns the calendar and the
// day number (1..31) for "today" by game rules.
func loadCalendar(p tradeParam) (*PriceCalendar, int, error) {
	if p.OnlineUpdate {
		if err := refreshOnlineCalendar(p); err != nil {
			logf("online calendar refresh failed, falling back to bundled: %v", err)
		}
	}

	path := p.CalendarPath
	if path == "" {
		path = findBundledCalendar()
	}
	if path == "" {
		return nil, 0, fmt.Errorf("price calendar not found (set calendar_path or bundle price_calendar.v1.json)")
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read calendar %s: %w", path, err)
	}

	var cal PriceCalendar
	if err := json.Unmarshal(trimBOM(raw), &cal); err != nil {
		return nil, 0, fmt.Errorf("parse calendar: %w", err)
	}

	if err := cal.validate(); err != nil {
		return nil, 0, err
	}

	return &cal, gameDay(time.Now()), nil
}

// validate enforces schema_version, timezone, and that every day key is 1..31
// with a shop present in the calendar's own `shops` whitelist. Missing days
// are tolerated (treated as empty).
func (c *PriceCalendar) validate() error {
	if c.SchemaVersion != calendarSchemaVersion {
		return fmt.Errorf("calendar schema_version=%d, want %d", c.SchemaVersion, calendarSchemaVersion)
	}
	if c.Timezone != "Asia/Shanghai" {
		return fmt.Errorf("calendar timezone=%q, want Asia/Shanghai", c.Timezone)
	}
	shopSet := make(map[string]bool, len(c.Shops))
	for _, s := range c.Shops {
		shopSet[s] = true
	}
	if len(shopSet) == 0 {
		return fmt.Errorf("calendar has no shops whitelist")
	}
	for d, items := range c.Days {
		day, err := strconv.Atoi(d)
		if err != nil || day < 1 || day > 31 {
			return fmt.Errorf("calendar days key %q is not 1..31", d)
		}
		for _, it := range items {
			if !shopSet[it.Shop] {
				return fmt.Errorf("day %d item %q: shop %q not in shops whitelist", day, it.Item, it.Shop)
			}
			if strings.TrimSpace(it.Item) == "" {
				return fmt.Errorf("day %d has an item with empty name", day)
			}
		}
	}
	return nil
}

// sellList returns today's peak-price (sell) candidates.
func (c *PriceCalendar) sellList(day int) []SellItem {
	if c == nil {
		return nil
	}
	return c.Days[strconv.Itoa(day)]
}

// findBundledCalendar locates price_calendar.v1.json relative to the agent.
// Layout: <exe>/../resource/price_calendar.v1.json (shipped next to the
// resource dir) or <exe>/price_calendar.v1.json.
func findBundledCalendar() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "..", "resource", "price_calendar.v1.json"),
		filepath.Join(exeDir, "resource", "price_calendar.v1.json"),
		filepath.Join(exeDir, "price_calendar.v1.json"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}

// refreshOnlineCalendar downloads the latest calendar and writes it next to
// the bundled file. Uses a 24h mtime cache: if the cached file is fresher
// than 24h the network call is skipped.
func refreshOnlineCalendar(p tradeParam) error {
	url := p.OnlineURL
	if url == "" {
		url = defaultOnlineURL
	}

	cached := findBundledCalendar()
	if cached != "" {
		if info, err := os.Stat(cached); err == nil {
			if time.Since(info.ModTime()) < 24*time.Hour {
				logf("calendar cache fresh (%s), skip online update", info.ModTime().Format(time.RFC3339))
				return nil
			}
		}
	}

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("calendar HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20)) // 2 MiB cap
	if err != nil {
		return err
	}
	// Quick sanity: must be JSON with schema_version.
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(trimBOM(body), &probe); err != nil {
		return fmt.Errorf("online calendar not valid JSON: %w", err)
	}
	if probe.SchemaVersion != calendarSchemaVersion {
		return fmt.Errorf("online calendar schema_version=%d, want %d", probe.SchemaVersion, calendarSchemaVersion)
	}

	out := cached
	if out == "" {
		exe, _ := os.Executable()
		out = filepath.Join(filepath.Dir(exe), "..", "resource", "price_calendar.v1.json")
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(out, body, 0o644); err != nil {
		return err
	}
	logf("online calendar updated: %s (%d bytes)", out, len(body))
	return nil
}

// describePlan builds a single-line summary of today's sell list for logging.
func describePlan(cal *PriceCalendar, day int, sell []SellItem) string {
	var b strings.Builder
	fmt.Fprintf(&b, "game day %d: %d peak item(s) to sell", day, len(sell))
	if len(sell) > 0 {
		b.WriteString(" | ")
		for i, it := range sell {
			if i > 0 {
				b.WriteString(", ")
			}
			fmt.Fprintf(&b, "%s@%s", it.Item, shopLabel(it.Shop))
		}
	}
	return b.String()
}

// matchName reports whether the OCR text matches the item (by primary name or
// any alias, case-insensitive substring).
func (it SellItem) matchName(ocrText string) bool {
	text := strings.ToLower(strings.TrimSpace(ocrText))
	if text == "" {
		return false
	}
	if strings.Contains(text, strings.ToLower(it.Item)) {
		return true
	}
	for _, alias := range it.Aliases {
		if alias != "" && strings.Contains(text, strings.ToLower(alias)) {
			return true
		}
	}
	return false
}
