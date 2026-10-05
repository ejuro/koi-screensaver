package main

import (
	"cmp"
	"math"
	"math/rand/v2"
	"slices"
)

// The pond is drawn on a grid of sextant pixels, two across and three down
// each terminal cell. World units are one pixel wide; a pixel is `aspect`
// units tall, so circles stay round.
type pond struct {
	cols, rows int
	pw, ph     int
	aspect     float64
	w, h       float64
	size       float64 // koi scale; a koi is 18*size long
	fade       float64 // 0 to 1 as the pond fades in from the background
	rng        *rand.Rand
	pal        palette
	t          float64

	koi    []*koi
	pads   []*pad
	petals []*petal
	rings  []ring
	intro  *intro // while the wordmark turns into the koi
	chase  *chase // the koi chase scene; nil for the pond

	// The boxes drawn last frame, and whether the next frame must be drawn
	// whole. Only the chase draws part of a frame.
	prevBoxes []box
	fullNext  bool
	drawn     []box // what this frame drew, nil for all of it

	tag    []int8
	shade  []float32 // depth of the highest thing shading each pixel, or +Inf
	depth  []float32 // depth of the koi drawn at each pixel
	height []float32
	base   []rgb // the still water
	out    []rgb
	fitted []rgb // the pixels each cell was last fitted to

	ringTab []float32 // scratch for drawRings
	koiTmp  []float32 // scratch for shape
}

// ringStep is how finely a ring's height is tabled, in world units.
const ringStep = 0.05

type pt struct{ x, y float64 }

type ring struct {
	x, y, r, speed float64
	amp, age, life float64
	width          float64
}

type pad struct {
	x, y, r    float64
	rot, notch float64 // which way the notch opens, and half its angle
	wobble     float64 // phase of the rim's unevenness
	flower     float64 // flower radius, 0 for none
	fx, fy     float64 // flower offset from the pad's centre, in pad radii

	// The pad's and its lily's pixels as last drawn, kept while the pad
	// stays put, and where it was then.
	px    []padPx
	pxKey [4]float64
}

// padPx is one pixel of a pad or its lily, and what is there.
type padPx struct {
	i int32
	t int8
}

type petal struct {
	x, y, vx, vy, rot, spin, r float64
}

func newPond(cols, rows int, aspect float64, pal palette, seed uint64, scene string) *pond {
	p := &pond{cols: cols, rows: rows, aspect: aspect, pal: pal, fade: 1, fullNext: true, rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	p.pw, p.ph = cols*2, rows*3
	p.w, p.h = float64(p.pw), float64(p.ph)*aspect
	n := p.pw * p.ph
	p.tag = make([]int8, n)
	p.shade = make([]float32, n)
	p.depth = make([]float32, n)
	p.height = make([]float32, n)
	p.base = make([]rgb, n)
	p.out = make([]rgb, n)
	p.fitted = make([]rgb, n)

	m := math.Min(p.w, p.h)
	p.size = math.Max(0.9, m*0.21/18)

	// Flat water in the theme's own background, as on the wallpapers, so the
	// fade in from the desktop is seamless.
	for i := range p.base {
		p.base[i] = pal.bg
	}
	black := rgb{0, 0, 0}
	if pal.dark {
		p.pal.shadow = mix(pal.bg, black, 0.45)
	} else {
		p.pal.shadow = mix(pal.bg, pal.tags[tagInk], 0.12)
	}

	if scene == sceneChase {
		p.setupChase()
	} else {
		p.placePads()
		// Four or five on a wide screen: few enough that the terminal
		// isn't redrawing every line of the screen every frame.
		count := int(clamp(math.Round(p.w*p.h/(18*p.size)/(18*p.size)/9), 3, 5))
		// Each koi keeps to its own depth, spread from just under the
		// surface to near the bottom, so one that swims under another
		// always does.
		depths := p.rng.Perm(count)
		for i := range count {
			k := p.newKoi(patterns[i%len(patterns)])
			k.depth = 0.1 + 0.8*float64(depths[i])/float64(max(1, count-1))
			p.koi = append(p.koi, k)
		}
		for range 3 {
			p.petals = append(p.petals, &petal{
				x: p.rng.Float64() * p.w, y: p.rng.Float64() * p.h,
				vx: (p.rng.Float64() - 0.5) * 1.2 * p.size, vy: (p.rng.Float64() - 0.5) * 0.8 * p.size,
				rot: p.rng.Float64() * 6.28, spin: (p.rng.Float64() - 0.5) * 0.15, r: 1.6 * p.size,
			})
		}
	}
	// Let the koi straighten out and spread over the pond before the first
	// frame.
	for range 600 { // twenty seconds
		p.step(1.0 / 30)
	}
	return p
}

// placePads scatters a few clusters of lily pads, mostly towards the edges so
// the koi keep open water in the middle.
func (p *pond) placePads() {
	// Two opposite corners and the middle of one of the other edges.
	spots := []pt{{0.1, 0.18}, {0.88, 0.8}, {0.5, 0.06}}
	if p.rng.IntN(2) == 0 {
		spots = []pt{{0.9, 0.14}, {0.14, 0.86}, {0.5, 0.94}}
	}
	if p.rng.IntN(2) == 0 {
		spots[2].y = 1 - spots[2].y
	}
	unit := 18 * p.size
	for _, s := range spots {
		cx := s.x*p.w + (p.rng.Float64()-0.5)*0.08*p.w
		cy := s.y*p.h + (p.rng.Float64()-0.5)*0.08*p.h
		n := 2 + p.rng.IntN(4)
		var placed []*pad
		for try := 0; len(placed) < n && try < 200; try++ {
			r := unit * (0.26 + 0.2*p.rng.Float64())
			if len(placed) == 0 {
				r = unit * 0.48
			}
			a := p.rng.Float64() * 2 * math.Pi
			d := 0.0
			if len(placed) > 0 {
				d = unit * (0.45 + 0.5*p.rng.Float64())
			}
			x, y := cx+math.Cos(a)*d, cy+math.Sin(a)*d
			ok := true
			for _, q := range placed {
				if math.Hypot(q.x-x, q.y-y) < 0.82*(q.r+r) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			pd := &pad{x: x, y: y, r: r, rot: p.rng.Float64() * 2 * math.Pi, notch: 0.08 + 0.04*p.rng.Float64(), wobble: p.rng.Float64() * 6.28}
			placed = append(placed, pd)
		}
		// A water lily on the largest pad, a little off its centre, and now
		// and then a second one on a smaller pad.
		if len(placed) > 0 && p.rng.Float64() < 0.75 {
			placed[0].flower = placed[0].r * 0.52
			placed[0].fx, placed[0].fy = (p.rng.Float64()-0.5)*0.3, (p.rng.Float64()-0.5)*0.3
			if len(placed) > 2 && p.rng.Float64() < 0.3 {
				placed[2].flower = placed[2].r * 0.6
			}
		}
		p.pads = append(p.pads, placed...)
	}
}

func (p *pond) addRing(x, y, r0, speed, amp, life, width float64) {
	p.rings = append(p.rings, ring{x: x, y: y, r: r0, speed: speed, amp: amp, life: life, width: width})
}

func (p *pond) onPad(x, y float64) bool {
	for _, pd := range p.pads {
		if math.Hypot(pd.x-x, pd.y-y) < pd.r {
			return true
		}
	}
	return false
}

func (p *pond) step(dt float64) {
	p.t += dt

	if p.intro != nil {
		p.stepIntro(dt)
	}
	if p.chase != nil {
		p.stepChase(dt)
	}
	for _, k := range p.koi {
		if k.ease <= 0 {
			continue
		}
		if p.chase != nil {
			p.chaseSwim(k, dt*k.ease)
		} else {
			p.swim(k, dt*k.ease)
		}
	}

	for _, pe := range p.petals {
		pe.x += pe.vx * dt
		pe.y += pe.vy * dt
		pe.rot += pe.spin * dt
		m := 4 * pe.r
		if pe.x < -m {
			pe.x = p.w + m
		} else if pe.x > p.w+m {
			pe.x = -m
		}
		if pe.y < -m {
			pe.y = p.h + m
		} else if pe.y > p.h+m {
			pe.y = -m
		}
	}

	kept := p.rings[:0]
	for _, r := range p.rings {
		r.age += dt
		if r.age > 0 {
			r.r += r.speed * dt
		}
		if r.age < r.life {
			kept = append(kept, r)
		}
	}
	p.rings = kept
}

// --- drawing -------------------------------------------------------------

// fill calls set for every pixel whose centre lies within r of (cx, cy),
// with the offset from the centre in world units.
func (p *pond) fill(cx, cy, r float64, set func(i int, dx, dy float64)) {
	if r <= 0 {
		return
	}
	x0, x1 := max(0, int(math.Floor(cx-r))), min(p.pw-1, int(math.Ceil(cx+r)))
	y0, y1 := max(0, int(math.Floor((cy-r)/p.aspect))), min(p.ph-1, int(math.Ceil((cy+r)/p.aspect)))
	r2 := r * r
	for py := y0; py <= y1; py++ {
		dy := (float64(py)+0.5)*p.aspect - cy
		if dy*dy > r2 {
			continue
		}
		row := py * p.pw
		for px := x0; px <= x1; px++ {
			dx := float64(px) + 0.5 - cx
			if dx*dx+dy*dy <= r2 {
				set(row+px, dx, dy)
			}
		}
	}
}

func (p *pond) disc(cx, cy, r float64, t int8) {
	p.fill(cx, cy, r, func(i int, _, _ float64) { p.tag[i] = t })
}

// castShadow casts a shadow from something at depth d (0 is the surface, 1 the
// bottom, the pads -1) onto everything below it.
func (p *pond) castShadow(cx, cy, r float64, d float32) {
	p.fill(cx, cy, r, func(i int, _, _ float64) { p.shade[i] = min(p.shade[i], d) })
}

func (p *pond) draw() {
	p.drawn = p.frameBoxes()
	if p.drawn == nil {
		clear(p.tag)
		for i := range p.shade {
			p.shade[i] = float32(math.Inf(1))
		}
		clear(p.height)
	} else {
		for _, b := range p.drawn {
			for y := b.y0; y < b.y1; y++ {
				lo, hi := y*p.pw+b.x0, y*p.pw+b.x1
				clear(p.tag[lo:hi])
				clear(p.height[lo:hi])
				for i := lo; i < hi; i++ {
					p.shade[i] = float32(math.Inf(1))
				}
			}
		}
	}

	p.drawRings()
	if p.chase != nil {
		p.drawSwirls()
		p.drawRipples()
	}

	// The deepest koi first, so shallower ones pass over them.
	order := slices.Clone(p.koi)
	slices.SortStableFunc(order, func(a, b *koi) int { return cmp.Compare(b.depth, a.depth) })

	// Shadows fall down and to the right, further the higher their caster
	// is above the pond floor: the pads and loose petals float on the
	// surface, the koi swim in between. They darken the floor and any koi
	// swimming beneath.
	sx, sy := 0.7*p.size, 1.2*p.size
	for _, pd := range p.pads {
		ox, oy := p.padShadow(pd)
		if p.touches(pd.x+ox, pd.y+oy, pd.r) {
			p.castShadow(pd.x+ox, pd.y+oy, pd.r*0.97, -1)
		}
	}
	for _, pe := range p.petals {
		p.petal(pe, sx*2.2, sy*2.2, func(i int) { p.shade[i] = -1 })
	}
	if p.intro != nil {
		p.introShadows(sx*2.2, sy*2.2)
	}
	// In the intro a koi shows only where the letters' motes have landed
	// on it (revealMask).
	for _, k := range order {
		if k.grow < minGrow {
			continue
		}
		off := 0.5 + 1.0*(1-k.depth)
		s := p.shape(k)
		s.masked = false
		if p.intro != nil {
			if mask := p.revealMask(slices.Index(p.koi, k), s); mask != nil {
				s.mask, s.masked = mask, true
			}
		}
		p.castKoiShadow(s, sx*off, sy*off, float32(k.depth))
	}
	for _, k := range order {
		if k.grow >= minGrow {
			p.fillKoi(k.shapeBuf, k.pat.body, float32(k.depth))
		}
	}
	for _, pe := range p.petals {
		p.petal(pe, 0, 0, func(i int) { p.tag[i] = tagPetal })
	}
	for _, pd := range p.pads {
		if p.touches(pd.x, pd.y, pd.r*padReach) {
			p.drawPadAndLily(pd)
		}
	}

	pal := &p.pal
	if p.drawn == nil {
		inParallel(len(p.tag), func(lo, hi int) {
			for i := lo; i < hi; i++ {
				p.shadePixel(i)
			}
		})
	} else {
		// A few small boxes: one core does them with less fuss than eight.
		for _, b := range p.drawn {
			for y := b.y0; y < b.y1; y++ {
				for i := y*p.pw + b.x0; i < y*p.pw+b.x1; i++ {
					p.shadePixel(i)
				}
			}
		}
	}
	if p.fade < 1 {
		f := p.fade * p.fade * (3 - 2*p.fade)
		for i, c := range p.out {
			p.out[i] = mix(pal.bg, c, f)
		}
	}
	if p.intro != nil {
		p.drawIntro()
	}
}

// minGrow is how far grown a koi has to be before it is drawn.
const minGrow = 0.4

// touches is whether a disc lies at least partly in what this frame draws.
// Outside that, last frame's pixels stand, so a still thing there needn't be
// drawn again.
func (p *pond) touches(cx, cy, r float64) bool {
	if p.drawn == nil {
		return true
	}
	x0, x1 := int(math.Floor(cx-r)), int(math.Ceil(cx+r))+1
	y0, y1 := int(math.Floor((cy-r)/p.aspect)), int(math.Ceil((cy+r)/p.aspect))+1
	for _, b := range p.drawn {
		if x0 < b.x1 && b.x0 < x1 && y0 < b.y1 && b.y0 < y1 {
			return true
		}
	}
	return false
}

// frameBoxes is what to draw this frame: nil for the whole pond, or, once
// it has settled, the boxes round whatever moves, both where it was last
// frame and where it is now.
func (p *pond) frameBoxes() []box {
	cur := p.movingBoxes()
	prev := p.prevBoxes
	p.prevBoxes = cur
	if p.fullNext || p.intro != nil || p.fade < 1 {
		p.fullNext = false
		return nil
	}
	return mergeBoxes(append(slices.Clone(prev), cur...))
}

// shadePixel gives pixel i its final colour from what was drawn there.
func (p *pond) shadePixel(i int) {
	pal := &p.pal
	h := float64(p.height[i])
	t := p.tag[i]
	var c rgb
	switch {
	case t == tagWater:
		c = p.base[i]
		if !math.IsInf(float64(p.shade[i]), 1) {
			c = mix(c, pal.shadow, 0.55)
		}
		if h > 0 {
			c = mix(c, pal.ripple, math.Min(h, 1)*0.5)
		} else if h < 0 {
			c = mix(c, pal.trough, math.Min(-h, 1)*0.5)
		}
	case t < firstSurface:
		// Shaded by whatever passes above, a pad or a shallower koi,
		// and crossed by the ripples on the surface, which lie over the
		// koi much as they do over the open water.
		c = pal.tags[t]
		if p.shade[i] < p.depth[i]-0.01 {
			c = mix(c, pal.koiShadow, 0.3)
		}
		if h > 0 {
			c = mix(c, pal.ripple, math.Min(h, 1)*0.42)
		}
	default:
		c = pal.tags[t]
	}
	p.out[i] = c
}

func (p *pond) drawRings() {
	for _, r := range p.rings {
		if r.age < 0 {
			continue
		}
		fade := 1 - r.age/r.life
		amp := r.amp * math.Pow(fade, 1.3)
		w := r.width
		outer := r.r + w
		inner := math.Max(0, r.r-3*w)
		// The ring's height depends only on the distance from its middle:
		// a crest, and a shallow trough inside it. Tabled once per ring,
		// read at each pixel.
		n := int((outer-inner)/ringStep) + 2
		if cap(p.ringTab) < n {
			p.ringTab = make([]float32, n)
		}
		tab := p.ringTab[:n]
		for k := range tab {
			u := inner + float64(k)*ringStep - r.r
			var v float64
			if u >= -w {
				c := math.Cos(u / w * math.Pi / 2)
				v = c * c
			} else {
				v = -0.4 * math.Sin(math.Pi*(-u-w)/(2*w))
			}
			tab[k] = float32(amp * v)
		}
		p.fill(r.x, r.y, outer, func(i int, dx, dy float64) {
			d := math.Sqrt(dx*dx + dy*dy)
			if d < inner {
				return
			}
			f := (d - inner) / ringStep
			k := min(int(f), n-2)
			p.height[i] += tab[k] + (tab[k+1]-tab[k])*float32(f-float64(k))
		})
	}
}

// padShadow is where a pad's shadow falls. In the pond the pads float high
// over the koi, and their shadows fall well off; the chase's pad casts a
// small, close one, as on the wallpaper.
func (p *pond) padShadow(pd *pad) (ox, oy float64) {
	if p.chase != nil {
		return 0.065 * pd.r, 0.1 * pd.r
	}
	return 0.7 * p.size * 2.2, 1.2 * p.size * 2.2
}

// drawPadAndLily draws a pad and the water lily on it, if any. The pads lie
// still, so their pixels are worked out once and copied after that.
func (p *pond) drawPadAndLily(pd *pad) {
	key := [4]float64{pd.x, pd.y, pd.r, pd.rot}
	if pd.px == nil || key != pd.pxKey {
		pd.px, pd.pxKey = pd.px[:0], key
		keep := func(i int, t int8) { pd.px = append(pd.px, padPx{int32(i), t}) }
		p.drawPad(pd, keep)
		if pd.flower > 0 {
			p.drawFlower(pd, keep)
		}
	}
	for _, q := range pd.px {
		p.tag[q.i] = q.t
	}
}

// drawPad draws a lily pad as on the Sumi koi wallpaper: a gently uneven
// rim with a thin darker edge, a straight notch cut from the very centre,
// and fine veins running out from there.
func (p *pond) drawPad(pd *pad, set func(i int, t int8)) {
	veins := clamp(math.Round(pd.r/4.5), 9, 19)
	rim := math.Max(1.2, 0.05*pd.r)
	p.fill(pd.x, pd.y, pd.r*padReach, func(i int, dx, dy float64) {
		d := math.Sqrt(dx*dx + dy*dy)
		a := math.Atan2(dy, dx)
		edge := pd.r * (1 + 0.018*math.Sin(3*a+pd.wobble) + 0.012*math.Sin(7*a+2*pd.wobble) + 0.006*math.Sin(13*a))
		if d > edge {
			return
		}
		rel := math.Remainder(a-pd.rot, 2*math.Pi)
		if math.Abs(rel) < pd.notch && d > 0.6 {
			return
		}
		t := tagPad
		if d > edge-rim {
			t = tagPadRim
		} else if d > 0.12*pd.r && d < 0.93*pd.r {
			// one vein runs down the middle of the notch, hidden by it,
			// so the rest fall evenly either side
			va := math.Remainder(rel, 2*math.Pi/veins)
			if math.Abs(va)*d < 0.55 {
				t = tagPadVein
			}
		}
		set(i, t)
	})
}

// padReach is how far past its radius a pad's uneven rim can reach.
const padReach = 1.04

// petalWidth is a petal's half-width along it, broad near its base and
// pointed at its tip (from YinYang's lotus). Worked out once into a table,
// since it is asked for at every pixel of every petal.
func petalWidth(along float64) float64 {
	f := clamp(along, 0, 1) * petalSteps
	i := min(int(f), petalSteps-1)
	return petalTable[i] + (petalTable[i+1]-petalTable[i])*(f-float64(i))
}

const petalSteps = 1024

var petalTable = func() (t [petalSteps + 1]float64) {
	for i := range t {
		along := float64(i) / petalSteps
		t[i] = 0.36 * math.Pow(math.Sin(math.Pi*math.Pow(along, 0.8)), 0.6)
	}
	return
}()

// petal calls set for every pixel of a loose petal, pointed at both ends.
func (p *pond) petal(pe *petal, ox, oy float64, set func(i int)) {
	length, width := pe.r*3, pe.r
	ca, sa := math.Cos(pe.rot), math.Sin(pe.rot)
	p.fill(pe.x+ox, pe.y+oy, length/2, func(i int, dx, dy float64) {
		along := (dx*ca+dy*sa)/length + 0.5
		across := (-dx*sa + dy*ca) / width
		if along > 0 && along < 1 && math.Abs(across) <= petalWidth(along)*2.2 {
			set(i)
		}
	})
}

func (p *pond) drawFlower(pd *pad, set func(i int, t int8)) {
	cx := pd.x + pd.fx*pd.r
	cy := pd.y + pd.fy*pd.r
	rings := []struct {
		reach, turn float64
		n           int
		t           int8
	}{
		{pd.flower * 0.975, pd.rot, 10, tagPetal},
		{pd.flower * 0.69, pd.rot + math.Pi/8, 8, tagPetalInner},
	}
	for _, ring := range rings {
		n := float64(ring.n)
		p.fill(cx, cy, ring.reach, func(i int, dx, dy float64) {
			a := math.Atan2(dy, dx) - ring.turn
			a = math.Remainder(a, 2*math.Pi/n)
			r := math.Hypot(dx, dy) / ring.reach
			along, across := r*math.Cos(a), r*math.Sin(a)
			if along > 0 && along < 1 && math.Abs(across) <= petalWidth(along)*1.15 {
				set(i, ring.t)
			}
		})
	}
	p.fill(cx, cy, math.Max(0.9, pd.flower*0.2), func(i int, _, _ float64) { set(i, tagStamen) })
}
