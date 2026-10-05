package main

import "math"

const (
	joints = 13
	seg    = 1.5 // joint spacing; a koi is 18 units long at size 1
)

// radii is a koi's half-width at each joint, head to tail (as in YinYang).
var radii = [joints]float64{2.5, 2.9, 2.9, 2.7, 2.4, 2.05, 1.75, 1.45, 1.2, 1.0, 0.85, 0.7, 0.6}

type pattern struct {
	body  int8
	back  [joints]int8 // a patch over each joint, or water for none
	sheen bool         // a metallic koi: light catches a stripe down its back
}

const (
	w = tagWater
	K = tagInk
	R = tagRed
	G = tagGold
)

var patterns = []pattern{
	{body: K, back: [joints]int8{w, R, R, w, w, R, R, R, w, w, w, w, w}},      // kohaku
	{body: R, back: [joints]int8{w, K, K, w, K, K, w, w, K, K, w, K, w}},      // YinYang's red
	{body: tagGold, sheen: true},                                              // yamabuki ogon
	{body: K, back: [joints]int8{R, w, w, w, w, w, w, w, w, w, w, w, w}},      // tancho
	{body: tagOrange, back: [joints]int8{w, w, K, K, w, w, w, K, w, w, w, w}}, // orange with ink
	{body: R, back: [joints]int8{w, G, w, w, G, G, w, w, w, G, w, w, w}},      // red and gold
	{body: K, back: [joints]int8{w, w, R, R, R, w, w, w, R, R, w, w, w}},
	{body: tagGold, back: [joints]int8{w, R, R, w, w, w, R, w, w, w, w, w, w}},
}

type koi struct {
	j         [joints]pt
	heading   float64
	speed     float64
	cruise    float64
	burst     float64
	burstT    float64
	wander    float64
	wanderT   float64
	phase     float64
	stroke    float64
	depth     float64
	depthGoal float64
	depthT    float64
	pat       pattern
	grow      float64 // 1 when whole; less while the intro gathers it up
	ease      float64 // 1 when swimming freely; less while it starts to
	slot      float64 // the chase: its place on the circle, behind the leader
	flick     float64 // the chase: seconds into a tail flick, 0 for none
	shapeBuf  *koiShape
}

func (p *pond) newKoi(pat pattern) *koi {
	k := &koi{pat: pat, grow: 1, ease: 1}
	m := 18 * p.size
	x := m + p.rng.Float64()*math.Max(1, p.w-2*m)
	y := m + p.rng.Float64()*math.Max(1, p.h-2*m)
	k.heading = p.rng.Float64() * 2 * math.Pi
	k.wander = k.heading
	k.cruise = 0.75 + 0.35*p.rng.Float64()
	k.speed = k.cruise * p.baseSpeed()
	k.phase = p.rng.Float64() * 6.28
	k.depth = p.rng.Float64()
	k.depthGoal = k.depth
	k.burstT = 4 + p.rng.Float64()*12
	for n := range joints {
		d := seg * p.size * float64(n)
		k.j[n] = pt{x - math.Cos(k.heading)*d, y - math.Sin(k.heading)*d}
	}
	return k
}

// baseSpeed: a koi glides about a fifth of its length each second.
func (p *pond) baseSpeed() float64 { return 18 * p.size * 0.2 }

func angleTo(from, to float64) float64 { return math.Remainder(to-from, 2*math.Pi) }

func (p *pond) swim(k *koi, dt float64) {
	length := 18 * p.size
	base := p.baseSpeed()
	head := k.j[0]

	// Wander: every few seconds pick a new heading to drift towards.
	k.wanderT -= dt
	if k.wanderT <= 0 {
		k.wander = k.heading + (p.rng.Float64()-0.5)*2.4
		k.wanderT = 4 + p.rng.Float64()*7
	}
	sx, sy := math.Cos(k.wander), math.Sin(k.wander)

	// Turn back from the banks.
	margin := length * 1.1
	push := func(v, lo, hi float64) float64 {
		if v < lo+margin {
			return (lo + margin - v) / margin
		}
		if v > hi-margin {
			return -(v - (hi - margin)) / margin
		}
		return 0
	}
	sx += 3 * push(head.x, 0, p.w)
	sy += 3 * push(head.y, 0, p.h)

	// Drift back towards the open water in the middle, where the pads
	// don't hide them, once they stray past a third of the way to the edge.
	nx, ny := head.x/p.w*2-1, head.y/p.h*2-1
	if r := math.Hypot(nx, ny); r > 0.3 {
		f := 3.0 * (r - 0.3) / r
		sx -= nx * f
		sy -= ny * f
	}

	// Keep a little apart from the others.
	for _, o := range p.koi {
		if o == k {
			continue
		}
		for _, n := range [3]int{0, 4, 8} {
			dx, dy := head.x-o.j[n].x, head.y-o.j[n].y
			d := math.Hypot(dx, dy)
			if d > 0 && d < length*1.1 {
				f := 2.6 * (1 - d/(length*1.1))
				sx += dx / d * f
				sy += dy / d * f
			}
		}
	}

	want := math.Atan2(sy, sx)
	// Turning radius never tighter than about half a body length.
	maxTurn := k.speed / (0.5 * length)
	k.heading += clamp(angleTo(k.heading, want)*1.3, -maxTurn, maxTurn) * dt

	// Now and then a few strong strokes, then a long glide.
	k.burstT -= dt
	if k.burstT <= 0 {
		k.burst = 0.6 + 0.6*p.rng.Float64()
		k.burstT = 9 + p.rng.Float64()*16
	}
	k.burst *= math.Exp(-dt / 1.6)
	goal := base * (k.cruise + k.burst)
	k.speed += (goal - k.speed) * math.Min(1, dt*0.9)
	effort := k.speed/base - 0.4 + 2*k.burst
	k.stroke += (clamp(effort, 0.35, 1.2) - k.stroke) * math.Min(1, dt*2)
	k.phase += dt * (2.6 + 3.4*k.stroke)

	k.j[0].x += math.Cos(k.heading) * k.speed * dt
	k.j[0].y += math.Sin(k.heading) * k.speed * dt
	p.follow(k)

	// Each koi drifts between the bottom and just under the surface.
	k.depthT -= dt
	if k.depthT <= 0 {
		k.depthGoal = p.rng.Float64()
		k.depthT = 8 + p.rng.Float64()*14
	}
	k.depth += (k.depthGoal - k.depth) * math.Min(1, dt*0.25)
}

// follow drags the body after the head, but a koi's spine only bends so far.
func (p *pond) follow(k *koi) {
	gap := seg * p.size
	back := k.heading + math.Pi
	for n := 1; n < joints; n++ {
		a, b := k.j[n-1], &k.j[n]
		ang := math.Atan2(b.y-a.y, b.x-a.x)
		ang = back + clamp(angleTo(back, ang), -0.22, 0.22)
		b.x, b.y = a.x+math.Cos(ang)*gap, a.y+math.Sin(ang)*gap
		back = ang
	}
}
