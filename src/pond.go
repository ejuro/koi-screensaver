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

	nextDrop float64 // seconds until the next raindrop

	tag    []int8
	shade  []float32 // depth of the highest thing shading each pixel, or +Inf
	depth  []float32 // depth of the koi drawn at each pixel
	height []float32
	base   []rgb // the still water
	out    []rgb
}

type pt struct{ x, y float64 }

type ring struct {
	x, y, r, speed float64
	amp, age, life float64
	width          float64
}

type pad struct {
	ax, ay, x, y, r float64
	rot, notch      float64
	drift           [4]float64 // phases
	flower          float64    // flower radius, 0 for none
	fx, fy          float64    // flower offset from the pad's centre, in pad radii
	bloom           float64    // phase of its slow breathing
}

type petal struct {
	x, y, vx, vy, rot, spin, r float64
}

func newPond(cols, rows int, aspect float64, pal palette, seed uint64) *pond {
	p := &pond{cols: cols, rows: rows, aspect: aspect, pal: pal, fade: 1, rng: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
	p.pw, p.ph = cols*2, rows*3
	p.w, p.h = float64(p.pw), float64(p.ph)*aspect
	n := p.pw * p.ph
	p.tag = make([]int8, n)
	p.shade = make([]float32, n)
	p.depth = make([]float32, n)
	p.height = make([]float32, n)
	p.base = make([]rgb, n)
	p.out = make([]rgb, n)

	m := math.Min(p.w, p.h)
	p.size = math.Max(0.9, m*0.21/18)

	// Deeper, darker water towards the middle of the pond.
	for py := 0; py < p.ph; py++ {
		for px := 0; px < p.pw; px++ {
			dx := (float64(px)+0.5)/p.w*2 - 1
			dy := (float64(py)+0.5)*aspect/p.h*2 - 1
			e := clamp((1.25-math.Hypot(dx, dy))/0.95, 0, 1)
			e = e * e * (3 - 2*e)
			p.base[py*p.pw+px] = mix(pal.shallow, pal.deep, e)
		}
	}

	p.placePads()
	count := int(clamp(math.Round(p.w*p.h/(18*p.size)/(18*p.size)/6), 3, 8))
	for i := range count {
		p.koi = append(p.koi, p.newKoi(patterns[i%len(patterns)], i))
	}
	for range 3 {
		p.petals = append(p.petals, &petal{
			x: p.rng.Float64() * p.w, y: p.rng.Float64() * p.h,
			vx: (p.rng.Float64() - 0.5) * 1.2 * p.size, vy: (p.rng.Float64() - 0.5) * 0.8 * p.size,
			rot: p.rng.Float64() * 6.28, spin: (p.rng.Float64() - 0.5) * 0.15, r: 1.6 * p.size,
		})
	}
	p.nextDrop = 3 + p.rng.Float64()*8
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
				if math.Hypot(q.ax-x, q.ay-y) < 0.82*(q.r+r) {
					ok = false
					break
				}
			}
			if !ok {
				continue
			}
			pd := &pad{ax: x, ay: y, x: x, y: y, r: r, rot: p.rng.Float64() * 2 * math.Pi, notch: 0.22 + 0.1*p.rng.Float64(), bloom: p.rng.Float64() * 6.28}
			for i := range pd.drift {
				pd.drift[i] = p.rng.Float64() * 6.28
			}
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

	// Now and then a single raindrop, never a shower.
	p.nextDrop -= dt
	if p.nextDrop <= 0 {
		p.nextDrop = 5 + p.rng.Float64()*10
		p.drop(p.rng.Float64()*p.w, p.rng.Float64()*p.h)
	}

	if p.intro != nil {
		p.stepIntro(dt)
	}
	for _, k := range p.koi {
		if k.ease > 0 {
			p.swim(k, dt*k.ease)
		}
	}

	for _, pd := range p.pads {
		a := 0.06 * pd.r
		pd.x = pd.ax + a*math.Sin(p.t*0.05+pd.drift[0]) + 0.4*a*math.Sin(p.t*0.13+pd.drift[1])
		pd.y = pd.ay + a*math.Sin(p.t*0.04+pd.drift[2]) + 0.4*a*math.Sin(p.t*0.11+pd.drift[3])
		pd.rot += 0.004 * math.Sin(p.t*0.03+pd.drift[1]) * dt * 10
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

// drop lets a raindrop fall at (x, y): two rings spreading out, unless it
// lands on a pad.
func (p *pond) drop(x, y float64) {
	if p.onPad(x, y) {
		return
	}
	s := p.size
	p.addRing(x, y, 0.3*s, 5.5*s, 1, 2.8, 0.8*s)
	p.rings = append(p.rings, ring{x: x, y: y, r: 0, speed: 4.2 * s, amp: 0.55, age: -0.35, life: 2.4, width: 0.7 * s})
	p.lure(x, y)
}

// lure sends the nearest koi, sometimes, to see whether a raindrop was food.
func (p *pond) lure(x, y float64) {
	if p.rng.Float64() > 0.45 {
		return
	}
	var best *koi
	bd := 0.4 * math.Max(p.w, p.h)
	for _, k := range p.koi {
		if d := math.Hypot(k.j[0].x-x, k.j[0].y-y); d < bd && k.goalT <= 0 && k.ease >= 1 {
			best, bd = k, d
		}
	}
	if best != nil {
		best.goal, best.goalT = pt{x, y}, 6
	}
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
	clear(p.tag)
	for i := range p.shade {
		p.shade[i] = float32(math.Inf(1))
	}
	clear(p.height)

	p.drawRings()

	// The deepest koi first, so shallower ones pass over them.
	order := slices.Clone(p.koi)
	slices.SortStableFunc(order, func(a, b *koi) int { return cmp.Compare(b.depth, a.depth) })

	// Shadows fall down and to the right, further the higher their caster
	// is above the pond floor: the pads and loose petals float on the
	// surface, the koi swim in between. They darken the floor and any koi
	// swimming beneath.
	sx, sy := 0.7*p.size, 1.2*p.size
	for _, pd := range p.pads {
		p.castShadow(pd.x+sx*2.2, pd.y+sy*2.2, pd.r*0.97, -1)
	}
	for _, pe := range p.petals {
		p.petal(pe, sx*2.2, sy*2.2, func(i int) { p.shade[i] = -1 })
	}
	if p.intro != nil {
		p.introShadows(sx*2.2, sy*2.2)
	}
	for _, k := range order {
		off := 0.5 + 1.0*(1-k.depth)
		d := float32(k.depth)
		p.drawKoi(k, func(cx, cy, r float64, t int8) {
			if t != tagEye {
				p.castShadow(cx+sx*off, cy+sy*off, r, d)
			}
		})
	}
	for _, k := range order {
		d := float32(k.depth)
		p.drawKoi(k, func(cx, cy, r float64, t int8) {
			p.fill(cx, cy, r, func(i int, _, _ float64) {
				p.tag[i] = t
				p.depth[i] = d
			})
		})
	}
	for _, pe := range p.petals {
		p.petal(pe, 0, 0, func(i int) { p.tag[i] = tagPetal })
	}
	for _, pd := range p.pads {
		p.drawPad(pd)
	}
	for _, pd := range p.pads {
		if pd.flower > 0 {
			p.drawFlower(pd)
		}
	}

	pal := &p.pal
	for i, t := range p.tag {
		h := float64(p.height[i])
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
			// and crossed by the ripples on the surface.
			c = pal.tags[t]
			if p.shade[i] < p.depth[i]-0.01 {
				c = mix(c, pal.koiShadow, 0.3)
			}
			if h > 0 {
				c = mix(c, pal.ripple, math.Min(h, 1)*0.18)
			}
		default:
			c = pal.tags[t]
		}
		p.out[i] = c
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
		p.fill(r.x, r.y, outer, func(i int, dx, dy float64) {
			d := math.Sqrt(dx*dx + dy*dy)
			if d < inner {
				return
			}
			u := d - r.r
			var v float64
			if u >= -w {
				c := math.Cos(u / w * math.Pi / 2)
				v = c * c
			} else {
				v = -0.4 * math.Sin(math.Pi*(-u-w)/(2*w))
			}
			p.height[i] += float32(amp * v)
		})
	}
}

func (p *pond) drawPad(pd *pad) {
	veins := 9.0
	veined := pd.r > 12 // smaller pads show only speckle
	p.fill(pd.x, pd.y, pd.r, func(i int, dx, dy float64) {
		d := math.Sqrt(dx*dx+dy*dy) / pd.r
		a := math.Atan2(dy, dx) - pd.rot
		a = math.Remainder(a, 2*math.Pi)
		if math.Abs(a) < pd.notch*(0.4+0.6*d) && d > 0.04 {
			return
		}
		t := tagPad
		if d > 0.9 {
			t = tagPadRim
		} else if veined && d > 0.16 && d < 0.8 {
			va := math.Remainder(a-pd.notch, 2*math.Pi/veins)
			if math.Abs(va)*d*pd.r < 0.45 {
				t = tagPadVein
			}
		}
		p.tag[i] = t
	})
}

// petalWidth is a petal's half-width along it, broad near its base and
// pointed at its tip (from YinYang's lotus).
func petalWidth(along float64) float64 {
	return 0.36 * math.Pow(math.Sin(math.Pi*math.Pow(along, 0.8)), 0.6)
}

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

func (p *pond) drawFlower(pd *pad) {
	cx := pd.x + pd.fx*pd.r
	cy := pd.y + pd.fy*pd.r
	breath := 0.5 + 0.5*math.Sin(p.t*0.25+pd.bloom)
	rings := []struct {
		reach, turn float64
		n           int
		t           int8
	}{
		{pd.flower * (0.95 + 0.05*breath), pd.rot, 10, tagPetal},
		{pd.flower * (0.66 + 0.06*breath), pd.rot + math.Pi/8, 8, tagPetalInner},
	}
	for _, ring := range rings {
		n := float64(ring.n)
		p.fill(cx, cy, ring.reach, func(i int, dx, dy float64) {
			a := math.Atan2(dy, dx) - ring.turn
			a = math.Remainder(a, 2*math.Pi/n)
			r := math.Hypot(dx, dy) / ring.reach
			along, across := r*math.Cos(a), r*math.Sin(a)
			if along > 0 && along < 1 && math.Abs(across) <= petalWidth(along)*1.15 {
				p.tag[i] = ring.t
			}
		})
	}
	p.disc(cx, cy, math.Max(0.9, pd.flower*0.2), tagStamen)
}
