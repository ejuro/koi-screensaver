package main

import "testing"

// Both scenes draw only the boxes round what moves; they must come out cell
// for cell the same as drawing the whole pond each frame.
func TestChaseDrawsChangesOnly(t *testing.T) { testDrawsChangesOnly(t, sceneChase) }
func TestPondDrawsChangesOnly(t *testing.T)  { testDrawsChangesOnly(t, scenePond) }

func testDrawsChangesOnly(t *testing.T, scene string) {
	pal := newPalette(themeColors())
	a := newPond(300, 90, aspectOf(9, 16), pal, 7, scene)
	b := newPond(300, 90, aspectOf(9, 16), pal, 7, scene)
	ca, cb := make([]cell, 300*90), make([]cell, 300*90)
	for f := range 600 { // half a minute, with tail flicks
		a.step(0.05)
		b.step(0.05)
		a.draw()
		a.cells(ca)
		b.fullNext = true
		b.draw()
		b.cells(cb)
		if a.drawn == nil && f > 0 && scene == sceneChase {
			t.Fatalf("frame %d: drew the whole pond", f)
		}
		for i := range ca {
			if ca[i] != cb[i] {
				t.Fatalf("frame %d: cell %d differs", f, i)
			}
		}
	}
}
