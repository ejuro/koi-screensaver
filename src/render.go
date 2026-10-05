package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"runtime"
	"strconv"
	"sync"
)

type col8 [3]uint8

func to8(c rgb) col8 {
	q := func(v float64) uint8 { return uint8(clamp(math.Round(v), 0, 255)) }
	return col8{q(c.r), q(c.g), q(c.b)}
}

// cell is a terminal cell: a sextant mask, its pixels in fg, the rest in bg.
type cell struct {
	mask   uint8
	fg, bg col8
}

// cells splits each cell's six pixels into the two colours that fit them
// best, trying every way to split them.
func (p *pond) cells(dst []cell) {
	boxes, spread := p.drawn, func(n int, f func(lo, hi int)) { f(0, n) }
	if boxes == nil {
		boxes, spread = []box{{0, 0, p.pw, p.ph}}, inParallel
	}
	// Only the cells drawn this frame; the rest of dst still holds them.
	// The whole pond is spread over the cores, a few small boxes are not.
	whole := p.drawn == nil
	for _, b := range boxes {
		r0, c0, c1 := b.y0/3, b.x0/2, b.x1/2
		spread(b.y1/3-r0, func(lo, hi int) {
			var px [6]rgb
			for row := r0 + lo; row < r0+hi; row++ {
				for c := c0; c < c1; c++ {
					same := !whole
					for i := range 6 {
						j := (row*3+i/2)*p.pw + c*2 + i%2
						px[i] = p.out[j]
						if px[i] != p.fitted[j] {
							same = false
							p.fitted[j] = px[i]
						}
					}
					// Most of a box is water that looks as it did last
					// frame; its cell needn't be fitted again.
					if !same {
						dst[row*p.cols+c] = fit(px)
					}
				}
			}
		})
	}
}

// inParallel splits 0..n into a band per core (at most 8) and runs f on
// each at once. A big pond at a small font is too much work for one core
// at full speed.
func inParallel(n int, f func(lo, hi int)) {
	parts := min(runtime.NumCPU(), 8, n)
	if parts <= 1 {
		f(0, n)
		return
	}
	var wg sync.WaitGroup
	for k := range parts {
		lo, hi := n*k/parts, n*(k+1)/parts
		wg.Go(func() { f(lo, hi) })
	}
	wg.Wait()
}

func dist2(a, b rgb) float64 {
	dr, dg, db := a.r-b.r, a.g-b.g, a.b-b.b
	return dr*dr + dg*dg + db*db
}

func fit(px [6]rgb) cell {
	var sum rgb
	flat := true
	for i := range 6 {
		sum.r, sum.g, sum.b = sum.r+px[i].r, sum.g+px[i].g, sum.b+px[i].b
		if dist2(px[i], px[0]) > 12 {
			flat = false
		}
	}
	if flat {
		c := to8(rgb{sum.r / 6, sum.g / 6, sum.b / 6})
		return cell{mask: 0, fg: c, bg: c}
	}
	best, bestErr := uint8(0), math.Inf(1)
	var bestA, bestB rgb
	// Pixel 5 always goes to the background, so each split is tried once.
	for m := uint8(1); m < 32; m++ {
		var a, b rgb
		na, nb := 0.0, 0.0
		for i := range 6 {
			if m&(1<<i) != 0 {
				a.r, a.g, a.b = a.r+px[i].r, a.g+px[i].g, a.b+px[i].b
				na++
			} else {
				b.r, b.g, b.b = b.r+px[i].r, b.g+px[i].g, b.b+px[i].b
				nb++
			}
		}
		a = rgb{a.r / na, a.g / na, a.b / na}
		b = rgb{b.r / nb, b.g / nb, b.b / nb}
		e := 0.0
		for i := range 6 {
			if m&(1<<i) != 0 {
				e += dist2(px[i], a)
			} else {
				e += dist2(px[i], b)
			}
		}
		if e < bestErr {
			best, bestErr, bestA, bestB = m, e, a, b
		}
	}
	// Two shades too close to tell apart would only show as stripes across
	// the smooth water; draw the cell flat instead.
	if dist2(bestA, bestB) < 80 {
		c := to8(rgb{sum.r / 6, sum.g / 6, sum.b / 6})
		return cell{mask: 0, fg: c, bg: c}
	}
	// Show each half in one of its own colours, not their average, so a
	// cell where red, ink and water meet never turns muddy.
	return cell{mask: best, fg: to8(medoid(px, best, true)), bg: to8(medoid(px, best, false))}
}

// medoid is the pixel of one half of a split closest to the rest of it.
func medoid(px [6]rgb, mask uint8, set bool) rgb {
	best, bestD := rgb{}, math.Inf(1)
	for i := range 6 {
		if (mask&(1<<i) != 0) != set {
			continue
		}
		d := 0.0
		for j := range 6 {
			if (mask&(1<<j) != 0) == set {
				d += dist2(px[i], px[j])
			}
		}
		if d < bestD {
			best, bestD = px[i], d
		}
	}
	return best
}

// sextant returns the block character showing the pixels in mask: bits 1
// and 2 the top pair, 4 and 8 the middle, 16 and 32 the bottom.
func sextant(mask uint8) rune {
	switch mask {
	case 0:
		return ' '
	case 63:
		return '█'
	case 21:
		return '▌'
	case 42:
		return '▐'
	}
	idx := rune(mask) - 1
	if mask > 21 {
		idx--
	}
	if mask > 42 {
		idx--
	}
	return 0x1FB00 + idx
}

// frame appends the escape codes that turn prev into cur, and updates prev.
// A nil prev entry (mask 255) forces a cell to be drawn.
func frame(buf []byte, cur, prev []cell, cols int) []byte {
	buf = append(buf, "\x1b[?2026h"...)
	var fg, bg col8
	styled := false
	at := -1
	for i, c := range cur {
		if prev[i] == c {
			continue
		}
		prev[i] = c
		if at != i {
			buf = append(buf, "\x1b["...)
			buf = strconv.AppendInt(buf, int64(i/cols+1), 10)
			buf = append(buf, ';')
			buf = strconv.AppendInt(buf, int64(i%cols+1), 10)
			buf = append(buf, 'H')
		}
		if !styled || c.bg != bg {
			buf = appendColor(buf, "\x1b[48;2;", c.bg)
			bg = c.bg
		}
		if c.mask != 0 && (!styled || c.fg != fg) {
			buf = appendColor(buf, "\x1b[38;2;", c.fg)
			fg = c.fg
		}
		styled = true
		buf = append(buf, string(sextant(c.mask))...)
		at = i + 1
		if at%cols == 0 {
			at = -1 // let the terminal's wrap never decide where we are
		}
	}
	return append(buf, "\x1b[?2026l"...)
}

func appendColor(buf []byte, lead string, c col8) []byte {
	buf = append(buf, lead...)
	buf = strconv.AppendInt(buf, int64(c[0]), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(c[1]), 10)
	buf = append(buf, ';')
	buf = strconv.AppendInt(buf, int64(c[2]), 10)
	return append(buf, 'm')
}

// writePNG draws cells the way a terminal would, cw by ch pixels each.
func writePNG(path string, cells []cell, cols, rows, cw, ch int) error {
	img := image.NewRGBA(image.Rect(0, 0, cols*cw, rows*ch))
	for r := range rows {
		for c := range cols {
			cl := cells[r*cols+c]
			for y := range ch {
				sy := min(2, y*3/ch)
				for x := range cw {
					sx := min(1, x*2/cw)
					v := cl.bg
					if cl.mask&(1<<(sy*2+sx)) != 0 {
						v = cl.fg
					}
					img.SetRGBA(c*cw+x, r*ch+y, color.RGBA{v[0], v[1], v[2], 255})
				}
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
