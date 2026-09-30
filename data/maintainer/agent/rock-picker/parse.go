package main

import (
	"fmt"
	"strings"
)

// maxRockCount is the sanity ceiling for one resource-bar number. 圣石 counts in
// BrownDust2 grow into the tens of thousands; anything past one million means the
// OCR grabbed something that is not a count at all, so we refuse it instead of
// silently picking a wrong cave.
const maxRockCount = 9_999_999

// digitFixes maps OCR look-alikes onto the digit they were probably meant to be.
// Deliberately conservative: only glyphs that are common confusions for a
// segmented game font. Anything outside this table + ASCII digits is a parse
// failure (we would rather raise an error than guess a wrong cave).
var digitFixes = map[rune]rune{
	'O': '0', 'o': '0', 'D': '0',
	'I': '1', 'l': '1', '|': '1', 'i': '1',
	'Z': '2',
	'A': '4',
	'S': '5',
	'B': '8',
	'g': '9',
}

// thousandsSeparators are dropped before parsing: the game renders counts as
// "9,428" (half-width comma) but CJK fonts / OCR sometimes produce other marks.
var thousandsSeparators = map[rune]bool{
	',': true, '，': true, '、': true, '.': true, ' ': true,
	'\u00a0': true, '\u3000': true, '\t': true, '_': true, '\'': true,
}

// parseRockCount turns one OCR text into an integer count.
//
// It accepts the separators and look-alikes above and rejects everything else,
// so callers can treat `ok == false` as "this row was not read reliably".
// Returns the parsed value plus a cleaned-up string for logging.
func parseRockCount(raw string) (value int, cleaned string, ok bool) {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if thousandsSeparators[r] {
			continue
		}
		if r >= '０' && r <= '９' { // full-width digits
			b.WriteRune('0' + (r - '０'))
			continue
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			continue
		}
		if fixed, hit := digitFixes[r]; hit {
			b.WriteRune(fixed)
			continue
		}
		return 0, b.String(), false
	}

	cleaned = b.String()
	if cleaned == "" {
		return 0, "", false
	}
	for _, r := range cleaned {
		if r < '0' || r > '9' {
			return 0, cleaned, false
		}
	}

	// Manual accumulation: counts are small enough that overflow is impossible
	// once the length is bounded, and this keeps the "too long" case explicit.
	if len(cleaned) > 7 {
		return 0, cleaned, false
	}
	for _, r := range cleaned {
		value = value*10 + int(r-'0')
	}
	if value > maxRockCount {
		return 0, cleaned, false
	}
	return value, cleaned, true
}

// pickLeast returns the index of the smallest count in counts.
// Ties resolve to the earliest index (i.e. the first row), which keeps the
// behaviour deterministic and reportable.
func pickLeast(counts []int) (int, error) {
	if len(counts) == 0 {
		return 0, fmt.Errorf("no counts to compare")
	}
	best := 0
	for i, c := range counts {
		if c < counts[best] {
			best = i
		}
	}
	return best, nil
}

// formatCounts renders the per-row readings for the log / focus message, e.g.
// "火=13010 水=9177 风=12008 光=10722 暗=16724".
func formatCounts(labels []string, counts []int) string {
	parts := make([]string, 0, len(counts))
	for i, c := range counts {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		parts = append(parts, fmt.Sprintf("%s=%d", label, c))
	}
	return strings.Join(parts, " ")
}
