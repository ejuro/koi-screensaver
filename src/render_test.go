package main

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"testing"
)

// A blank cell sets only the background. A black foreground after it must
// still be selected, not assumed (found in the pre-release review).
func TestBlackForegroundAfterBlank(t *testing.T) {
	white := col8{255, 255, 255}
	cur := []cell{{mask: 0, bg: white}, {mask: 1, fg: col8{}, bg: white}}
	prev := []cell{{mask: 255}, {mask: 255}}
	out := frame(nil, cur, prev, 2)
	if !bytes.Contains(out, []byte("\x1b[38;2;0;0;0m")) {
		t.Fatalf("black foreground never selected after a blank cell: %q", out)
	}
}

// screen is a tiny model of a terminal: it follows the cursor moves and the
// 24-bit colour codes frame writes, and records what each cell shows.
type screen struct {
	cols   int
	glyph  []rune
	fg, bg []col8
	curFg  col8
	curBg  col8
	at     int
}

func (s *screen) feed(t *testing.T, b []byte) {
	r := []rune(string(b))
	for i := 0; i < len(r); i++ {
		if r[i] != 0x1b {
			s.glyph[s.at], s.fg[s.at], s.bg[s.at] = r[i], s.curFg, s.curBg
			s.at++
			continue
		}
		j := i + 2 // past ESC [
		for j < len(r) && (r[j] < 0x40 || r[j] > 0x7e) {
			j++
		}
		params, final := string(r[i+2:j]), r[j]
		switch final {
		case 'H':
			var row, col int
			if _, err := fmt.Sscanf(params, "%d;%d", &row, &col); err != nil {
				t.Fatalf("bad cursor move %q", params)
			}
			s.at = (row-1)*s.cols + col - 1
		case 'm':
			var kind, r8, g8, b8 int
			if _, err := fmt.Sscanf(params, "%d;2;%d;%d;%d", &kind, &r8, &g8, &b8); err == nil {
				c := col8{uint8(r8), uint8(g8), uint8(b8)}
				if kind == 38 {
					s.curFg = c
				} else if kind == 48 {
					s.curBg = c
				}
			}
		}
		i = j
	}
}

// Whatever colours the terminal starts with, and however cells change from
// frame to frame, what it shows must end up exactly what was drawn.
func TestFramesDrawExactly(t *testing.T) {
	const cols, rows = 12, 5
	n := cols * rows
	rng := rand.New(rand.NewPCG(1, 2))
	palette := []col8{{0, 0, 0}, {255, 255, 255}, {200, 80, 60}, {20, 22, 27}}
	s := &screen{cols: cols, glyph: make([]rune, n), fg: make([]col8, n), bg: make([]col8, n),
		curFg: col8{255, 0, 255}, curBg: col8{0, 255, 0}} // something nothing draws
	prev := make([]cell, n)
	for i := range prev {
		prev[i] = cell{mask: 255}
	}
	cur := make([]cell, n)
	for f := range 200 {
		for i := range cur {
			if f == 0 || rng.IntN(3) == 0 {
				cur[i] = cell{mask: uint8(rng.IntN(64)), fg: palette[rng.IntN(4)], bg: palette[rng.IntN(4)]}
				if rng.IntN(4) == 0 {
					cur[i].mask = 0
				}
			}
		}
		s.feed(t, frame(nil, cur, prev, cols))
		for i, c := range cur {
			if s.bg[i] != c.bg || s.glyph[i] != sextant(c.mask) || (c.mask != 0 && s.fg[i] != c.fg) {
				t.Fatalf("frame %d cell %d: shows %q fg %v bg %v, want mask %d fg %v bg %v",
					f, i, s.glyph[i], s.fg[i], s.bg[i], c.mask, c.fg, c.bg)
			}
		}
	}
}
