package main

import "math"

// Drawing only what changed. Once the pond has settled, each frame clears
// and draws again only boxes round whatever moves, both where it was last
// frame and where it is now; everywhere else last frame's pixels stand. The
// water, the pads and the lilies never move.

// box is a block of pond pixels, x0 <= x < x1 and y0 <= y < y1, whole
// terminal cells.
type box struct{ x0, y0, x1, y1 int }

// boxAround is the cells covering a world-space box, clipped to the pond.
func (p *pond) boxAround(x0, y0, x1, y1 float64) (box, bool) {
	b := box{
		x0: max(0, int(math.Floor(x0/2))*2),
		x1: min(p.pw, int(math.Ceil(x1/2))*2),
		y0: max(0, int(math.Floor(y0/p.aspect/3))*3),
		y1: min(p.ph, int(math.Ceil(y1/p.aspect/3))*3),
	}
	return b, b.x0 < b.x1 && b.y0 < b.y1
}

// movingBoxes covers everything that moves this frame: the koi with their
// fins and shadows, the rings, the swirls, and the petals with theirs.
func (p *pond) movingBoxes() []box {
	var out []box
	add := func(x0, y0, x1, y1 float64) {
		if b, ok := p.boxAround(x0, y0, x1, y1); ok {
			out = append(out, b)
		}
	}
	s := p.size
	// A koi curves round the circle, so a box per stretch of it covers much
	// less water than one round it all. How far each stretch reaches past
	// its joints: the head with its big fins 6.3 sizes; the middle, with
	// the small fins and the start of the sway, 4; the tail, swaying up to
	// 3 sizes in a flick, 5.5. The shadow falls down and to the right,
	// further the deeper the koi swims.
	stretches := [...]struct {
		from, to int
		reach    float64
	}{{0, 4, 7}, {4, 8, 4}, {8, joints - 1, 5.5}}
	for _, k := range p.koi {
		off := 0.5 + 1.0*(1-k.depth) // as drawn
		sx, sy := 0.7*off*s+1, 1.2*off*s+1
		for _, st := range stretches {
			x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
			for _, q := range k.j[st.from : st.to+1] {
				x0, y0 = math.Min(x0, q.x), math.Min(y0, q.y)
				x1, y1 = math.Max(x1, q.x), math.Max(y1, q.y)
			}
			e := st.reach * s
			add(x0-e, y0-e, x1+e+sx, y1+e+sy)
		}
	}
	for _, r := range p.rings {
		e := r.r + r.width + 1
		add(r.x-e, r.y-e, r.x+e, r.y+e)
	}
	for _, pe := range p.petals {
		// the petal, pointed at both ends, and its shadow 1.5 sizes
		// right and 2.7 down
		e := 1.6*pe.r + 3*s
		add(pe.x-e, pe.y-e, pe.x+e, pe.y+e)
	}
	if p.chase != nil {
		e := p.swirlReach()
		for _, w := range p.chase.swirls {
			add(w.x-e, w.y-e, w.x+e, w.y+e)
		}
		for _, r := range p.chase.ripples {
			if r.age >= r.delay {
				e := p.rippleReach(r)
				add(p.w/2-e, p.h/2-e, p.w/2+e, p.h/2+e)
			}
		}
	}
	return out
}

func (b box) area() int { return (b.x1 - b.x0) * (b.y1 - b.y0) }

func (b box) union(o box) box {
	return box{min(b.x0, o.x0), min(b.y0, o.y0), max(b.x1, o.x1), max(b.y1, o.y1)}
}

func (b box) overlap(o box) int {
	w, h := min(b.x1, o.x1)-max(b.x0, o.x0), min(b.y1, o.y1)-max(b.y0, o.y0)
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

// mergeBoxes joins boxes that mostly overlap, such as a stretch of koi this
// frame and last, so their pixels are only drawn once. Boxes whose joined box
// would take in much more water than the two cover stay apart.
func mergeBoxes(bs []box) []box {
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(bs) && !merged; i++ {
			for j := i + 1; j < len(bs); j++ {
				covered := bs[i].area() + bs[j].area() - bs[i].overlap(bs[j])
				if u := bs[i].union(bs[j]); u.area()*10 <= covered*11 {
					bs[i] = u
					bs = append(bs[:j], bs[j+1:]...)
					merged = true
					break
				}
			}
		}
	}
	return bs
}
