package main

import (
	"fmt"
	"image"
	"math"
	"sort"
	"strings"
)

// zone is a horizontal span of the minigame bar, in absolute screen X.
type zone struct {
	Start int
	End   int // exclusive
}

func (z zone) width() int { return z.End - z.Start }

func (z zone) center() float64 { return (float64(z.Start) + float64(z.End)) / 2 }

// barFrame is one analysed frame of the minigame bar.
type barFrame struct {
	Valid   bool
	CursorX float64
	Blue    []zone
	Yellow  []zone
	Green   []zone
}

func (f barFrame) String() string {
	if !f.Valid {
		return "invalid"
	}
	return fmt.Sprintf("cursor=%.0f blue=%s yellow=%s green=%s",
		f.CursorX, formatZones(f.Blue), formatZones(f.Yellow), formatZones(f.Green))
}

func formatZones(zs []zone) string {
	if len(zs) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(zs))
	for _, z := range zs {
		parts = append(parts, fmt.Sprintf("[%d,%d)", z.Start, z.End))
	}
	return strings.Join(parts, ",")
}

// rgbToHSV converts 8-bit RGB to H(0-360) S(0-255) V(0-255).
func rgbToHSV(r, g, b uint8) (h, s, v float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	mx := math.Max(rf, math.Max(gf, bf))
	mn := math.Min(rf, math.Min(gf, bf))
	d := mx - mn

	v = mx * 255
	if mx == 0 {
		s = 0
	} else {
		s = d / mx * 255
	}
	if d == 0 {
		h = 0
	} else {
		switch mx {
		case rf:
			h = 60 * math.Mod((gf-bf)/d, 6)
		case gf:
			h = 60 * ((bf-rf)/d + 2)
		default:
			h = 60 * ((rf-gf)/d + 4)
		}
	}
	if h < 0 {
		h += 360
	}
	return h, s, v
}

// analyzeBar scans the bar ROI column by column.
//
// A column counts as "blue" / "yellow" when at least minColHits of its pixels
// match that filter; matching columns are merged into runs (bridging gaps up to
// zoneGap, which heals the split the cursor star causes when it overlaps a
// zone), and runs narrower than minZoneWidth are dropped as noise.
//
// The cursor is different: the white marker star is much taller than any other
// white speckle inside the tight bar ROI (bar end caps, glints), so only
// columns with at least cursorMinHits white pixels participate. Its position is
// the hit-weighted centre of those columns, which stays sub-pixel stable.
func analyzeBar(img *image.RGBA, roi []int, c colors, minColHits, minZoneWidth, cursorMinHits, zoneGap int) barFrame {
	out := barFrame{}
	if img == nil || len(roi) != 4 {
		return out
	}

	b := img.Bounds()
	x0, y0, w, h := roi[0], roi[1], roi[2], roi[3]
	// Clip to the image (a mis-set ROI must not panic the agent).
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x0+w > b.Max.X {
		w = b.Max.X - x0
	}
	if y0+h > b.Max.Y {
		h = b.Max.Y - y0
	}
	if w <= 0 || h <= 0 {
		return out
	}

	cursorHits := make([]int, w)
	blueHits := make([]int, w)
	yellowHits := make([]int, w)
	greenHits := make([]int, w)
	cursorSum, cursorWeight := 0.0, 0.0

	for x := 0; x < w; x++ {
		sx := x0 + x
		for y := 0; y < h; y++ {
			sy := y0 + y
			i := (sy-b.Min.Y)*img.Stride + (sx-b.Min.X)*4
			if i+2 >= len(img.Pix) {
				continue
			}
			hh, ss, vv := rgbToHSV(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
			switch {
			case c.Cursor.match(hh, ss, vv):
				cursorHits[x]++
			case c.Yellow.match(hh, ss, vv):
				yellowHits[x]++
			case c.Green.match(hh, ss, vv):
				greenHits[x]++
			case c.Blue.match(hh, ss, vv):
				blueHits[x]++
			}
		}
		if n := cursorHits[x]; n >= cursorMinHits {
			cursorSum += float64(sx) * float64(n)
			cursorWeight += float64(n)
		}
	}

	if cursorWeight > 0 {
		out.CursorX = cursorSum / cursorWeight
	}
	out.Green = mergeZones(greenHits, x0, minColHits, minZoneWidth, zoneGap)
	out.Blue = mergeZones(blueHits, x0, minColHits, minZoneWidth, zoneGap)
	out.Yellow = mergeZones(yellowHits, x0, minColHits, minZoneWidth, zoneGap)
	out.Valid = cursorWeight > 0 && (len(out.Blue) > 0 || len(out.Yellow) > 0)
	return out
}

// subtractZones removes parts of src that overlap any sub zone. Fragments
// narrower than minWidth are dropped so tiny slivers do not become targets.
func subtractZones(src, sub []zone, minWidth int) []zone {
	if len(sub) == 0 || len(src) == 0 {
		return src
	}
	// Sub zones must be sorted by start coordinate.
	sorted := make([]zone, len(sub))
	copy(sorted, sub)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	var out []zone
	for _, z := range src {
		cur := z.Start
		for _, s := range sorted {
			if s.End <= cur {
				continue
			}
			if s.Start >= z.End {
				break
			}
			if s.Start > cur {
				if s.Start-cur >= minWidth {
					out = append(out, zone{Start: cur, End: s.Start})
				}
			}
			if s.End > cur {
				cur = s.End
			}
		}
		if z.End-cur >= minWidth {
			out = append(out, zone{Start: cur, End: z.End})
		}
	}
	return out
}

// mergeZones turns per-column hit counts into absolute-coordinate runs.
//
// Gaps of up to zoneGap non-matching columns are bridged: the moving cursor
// star occludes whatever is behind it, so a zone it crosses would otherwise be
// reported as two half-zones. Runs narrower than minZoneWidth (after merging)
// are dropped.
func mergeZones(hits []int, x0, minColHits, minZoneWidth, zoneGap int) []zone {
	var out []zone
	start := -1
	gap := 0
	flush := func(end int) {
		if start >= 0 && end-start >= minZoneWidth {
			out = append(out, zone{Start: x0 + start, End: x0 + end})
		}
		start, gap = -1, 0
	}
	for x, n := range hits {
		if n >= minColHits {
			if start < 0 {
				start = x
			}
			gap = 0
		} else if start >= 0 {
			gap++
			if gap > zoneGap {
				flush(x - gap + 1)
			}
		}
	}
	if start >= 0 {
		flush(len(hits) - gap)
	}
	return out
}

// dumpRowColors renders the bar's centre row as RGB triples, four pixels apart,
// so the colour filters can be tuned from real pixels instead of guesswork.
func dumpRowColors(img *image.RGBA, roi []int) string {
	if img == nil || len(roi) != 4 {
		return ""
	}
	b := img.Bounds()
	x0, y0, w, h := roi[0], roi[1], roi[2], roi[3]
	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x0+w > b.Max.X {
		w = b.Max.X - x0
	}
	if y0+h > b.Max.Y {
		h = b.Max.Y - y0
	}
	if w <= 0 || h <= 0 {
		return ""
	}
	y := y0 + h/2

	var sb strings.Builder
	for x := 0; x < w; x += 4 {
		sx := x0 + x
		i := (y-b.Min.Y)*img.Stride + (sx-b.Min.X)*4
		if i+2 >= len(img.Pix) {
			break
		}
		hh, ss, vv := rgbToHSV(img.Pix[i], img.Pix[i+1], img.Pix[i+2])
		fmt.Fprintf(&sb, "%d:(%d,%d,%d)H%.0fS%.0fV%.0f ", sx, img.Pix[i], img.Pix[i+1], img.Pix[i+2], hh, ss, vv)
	}
	return sb.String()
}
