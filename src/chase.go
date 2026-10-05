package main

import "math"

// The koi chase: two koi, red and ink, circling a lily pad in the middle of
// still water, the same distance apart all the way round, as on the Sumi
// koi wallpaper and in cliamp's YinYang. Now and then one flicks its tail
// and leaves a little swirl in the water, as in YinYang.
//
// Almost nothing moves but the two koi, so only the water around them, and
// around a swirl, is drawn again each frame.

const (
	scenePond  = "pond"
	sceneChase = "chase"
)

type chase struct {
	theta   float64 // where the chase has got to round the circle
	rp      float64 // the radius the koi swim round
	padR    float64 // the pad's radius
	speed   float64
	started bool    // the chase only sets off when the koi do
	flickT  float64 // seconds until the next tail flick
	swirls  []swirl
}

// Sizes as fractions of the shorter side of the screen, from the wallpaper.
const (
	chaseKoiSize = 0.0185 // a koi is 18 of these long
	chaseRadius  = 0.248
	chasePad     = 0.0995
)

func (p *pond) setupChase() {
	m := math.Min(p.w, p.h)
	p.size = m * chaseKoiSize
	c := &chase{rp: m * chaseRadius, theta: math.Pi}
	// A slow lap, about half a minute.
	c.speed = 18 * p.size * 0.17
	c.flickT = 6 + p.rng.Float64()*6
	p.chase = c

	// Flat water in the theme's own background, as on the wallpaper, so
	// the fade in from the desktop is seamless.
	for i := range p.base {
		p.base[i] = p.pal.bg
	}
	black := rgb{0, 0, 0}
	if p.pal.dark {
		p.pal.shadow = mix(p.pal.bg, black, 0.45)
	} else {
		p.pal.shadow = mix(p.pal.bg, p.pal.tags[tagInk], 0.12)
	}

	// One still pad in the middle, its notch up and to the right.
	r := m * chasePad
	c.padR = r
	p.pads = []*pad{{ax: p.w / 2, ay: p.h / 2, x: p.w / 2, y: p.h / 2, r: r, rot: -0.55, notch: 0.15}}

	// Red with ink, and ink with red, as in YinYang.
	p.koi = []*koi{p.newKoi(patterns[1]), p.newKoi(patterns[0])}
	for i, k := range p.koi {
		k.slot = float64(i) * math.Pi
		k.depth, k.depthGoal = 0.3, 0.3
		k.stroke = 0.45
		// Start in place on the circle, the body laid along it.
		a := c.theta + k.slot
		k.heading = a + math.Pi/2
		gap := seg * p.size
		for n := range joints {
			b := a - float64(n)*gap/c.rp
			k.j[n] = pt{p.w/2 + math.Cos(b)*c.rp, p.h/2 + math.Sin(b)*c.rp}
		}
		k.speed = c.speed
	}
	c.started = true
}

// stepChase moves the chase on: the circle turns, now and then a tail flicks.
func (p *pond) stepChase(dt float64) {
	c := p.chase
	if !c.started && p.intro != nil && len(p.intro.release) > 0 && p.intro.t >= p.intro.release[0] {
		c.started = true
	}
	if c.started {
		c.theta += c.speed / c.rp * dt
	}

	// While the wordmark is up the pad waits; it opens out as the letters
	// lift off the water.
	grow := 1.0
	if in := p.intro; in != nil {
		grow = smooth((in.t - introHold) / 2.5)
	}
	p.pads[0].r = c.padR * grow

	// Now and then one of them flicks its tail.
	c.flickT -= dt
	if c.flickT <= 0 {
		c.flickT = 7 + p.rng.Float64()*9
		k := p.koi[p.rng.IntN(len(p.koi))]
		if k.ease >= 1 && k.flick == 0 {
			k.flick = 1e-9 // under way
		}
	}
	kept := c.swirls[:0]
	for _, w := range c.swirls {
		w.age += dt
		if w.age < w.life {
			kept = append(kept, w)
		}
	}
	c.swirls = kept
}

const flickRise = 0.35 // seconds a tail flick takes to reach its strongest

// swirlFrom leaves a swirl at a koi's tail tip, curling the way it sweeps.
func (p *pond) swirlFrom(k *koi) {
	t, b := k.j[joints-1], k.j[joints-2]
	dir := 1.0
	if p.rng.IntN(2) == 0 {
		dir = -1
	}
	a := math.Atan2(t.y-b.y, t.x-b.x) + dir*math.Pi/2
	p.chase.swirls = append(p.chase.swirls, swirl{x: t.x, y: t.y, a: a, dir: dir, life: 1.4})
}

// swirl is the little curl of water a tail flick leaves behind: a spiral
// that opens and fades where the tail was (YinYang's swirl).
type swirl struct {
	x, y, a, dir float64
	age, life    float64
}

// reach is how far a swirl spreads from where it started, at its widest.
func (p *pond) swirlReach() float64 { return 4.2 * p.size }

func (p *pond) drawSwirls() {
	s := p.size
	for _, w := range p.chase.swirls {
		open := w.age / w.life
		r := (1 + 2.5*open) * s
		amp := 0.95 * math.Pow(1-open, 1.2)
		// the curl shortens from its outer end as it opens
		end := 1 - 0.6*open
		const steps = 48
		for i := 0; i <= steps; i++ {
			t := end * float64(i) / steps
			a := w.a + w.dir*t*4.5
			rr := r * (0.4 + 0.6*t)
			x, y := w.x+math.Cos(a)*rr, w.y+math.Sin(a)*rr
			width := s * (0.22 + 0.2*(1-t))
			v := float32(amp * (0.55 + 0.45*(1-t)))
			p.fill(x, y, width, func(i int, _, _ float64) {
				p.height[i] = max(p.height[i], v)
			})
		}
	}
}

// chaseSwim keeps a koi in its place on the circle: steering on to the
// circle, and swimming a little faster or slower until it is level with its
// place, then holding it.
func (p *pond) chaseSwim(k *koi, dt float64) {
	c := p.chase
	length := 18 * p.size
	cx, cy := p.w/2, p.h/2
	head := k.j[0]
	r := math.Hypot(head.x-cx, head.y-cy)
	phi := math.Atan2(head.y-cy, head.x-cx)

	// Along the circle (clockwise on screen), leaning in or out towards it.
	g := clamp((r-c.rp)/(0.5*length), -1, 1)
	tx, ty := -math.Sin(phi)-math.Cos(phi)*g, math.Cos(phi)-math.Sin(phi)*g
	want := math.Atan2(ty, tx)
	maxTurn := k.speed / (0.4 * length)
	k.heading += (k.speed/math.Max(r, 1) + clamp(angleTo(k.heading, want)*2, -maxTurn, maxTurn)) * dt

	goal := c.speed
	if c.started {
		gap := math.Remainder(c.theta+k.slot-phi, 2*math.Pi)
		goal *= clamp(1+1.2*gap, 0.55, 1.7)
	}
	k.speed += (goal - k.speed) * math.Min(1, dt*1.5)

	// An easy, steady stroke, and a few stronger beats when it flicks: the
	// stroke builds over a third of a second and settles back over about
	// one, rather than snapping.
	e := 0.0
	if k.flick > 0 {
		was := k.flick
		k.flick += dt
		e = smooth(k.flick/flickRise) * math.Exp(-math.Max(0, k.flick-flickRise)/0.8)
		if was < flickRise && k.flick >= flickRise {
			p.swirlFrom(k) // at the strongest sweep
		}
		if k.flick > 4 {
			k.flick = 0
		}
	}
	k.stroke = 0.45 + 0.8*e
	k.phase += dt * (2.6 + 3.4*k.stroke)

	k.j[0].x += math.Cos(k.heading) * k.speed * dt
	k.j[0].y += math.Sin(k.heading) * k.speed * dt
	p.follow(k)
}
