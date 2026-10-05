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
	wakeT     float64
	goal      pt
	goalT     float64
	pat       pattern
	grow      float64 // 1 when whole; less while the intro gathers it up
	ease      float64 // 1 when swimming freely; less while it starts to
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
	k.wakeT = p.rng.Float64() * 0.3
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

	// Drift over to where a raindrop fell, and nose at it.
	if k.goalT > 0 {
		k.goalT -= dt
		dx, dy := k.goal.x-head.x, k.goal.y-head.y
		d := math.Hypot(dx, dy)
		if d < length*0.15 {
			p.addRing(head.x, head.y, radii[0]*p.size, 3.2*p.size, 0.7, 2.2, 0.7*p.size)
			k.goalT = 0
			k.depthGoal = 0.1
		} else if d > 0 {
			sx += dx / d * 2.5
			sy += dy / d * 2.5
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
	if k.goalT > 0 {
		goal *= 1.25
	}
	k.speed += (goal - k.speed) * math.Min(1, dt*0.9)
	effort := k.speed/base - 0.4 + 2*k.burst
	k.stroke += (clamp(effort, 0.35, 1.2) - k.stroke) * math.Min(1, dt*2)
	k.phase += dt * (2.6 + 3.4*k.stroke)

	k.j[0].x += math.Cos(k.heading) * k.speed * dt
	k.j[0].y += math.Sin(k.heading) * k.speed * dt
	// The body follows the head, but a koi's spine only bends so far.
	gap := seg * p.size
	back := k.heading + math.Pi
	for n := 1; n < joints; n++ {
		a, b := k.j[n-1], &k.j[n]
		ang := math.Atan2(b.y-a.y, b.x-a.x)
		ang = back + clamp(angleTo(back, ang), -0.22, 0.22)
		b.x, b.y = a.x+math.Cos(ang)*gap, a.y+math.Sin(ang)*gap
		back = ang
	}

	// Each koi drifts between the bottom and just under the surface; near
	// the top it pushes a faint wake.
	k.depthT -= dt
	if k.depthT <= 0 {
		k.depthGoal = p.rng.Float64()
		k.depthT = 8 + p.rng.Float64()*14
	}
	k.depth += (k.depthGoal - k.depth) * math.Min(1, dt*0.25)
	k.wakeT -= dt
	if k.wakeT <= 0 {
		k.wakeT = 0.3
		near := 1 - k.depth
		if amp := 0.55 * near * near * math.Min(1.4, k.speed/base); amp > 0.05 {
			p.addRing(k.j[1].x, k.j[1].y, radii[1]*p.size, 0.45*base, amp, 2.2, 0.6*p.size)
		}
	}
}

// drawKoi draws a koi as discs along its spine, swaying its rear half with
// its stroke. paint receives each disc in drawing order.
func (p *pond) drawKoi(k *koi, paint func(cx, cy, r float64, t int8)) {
	if k.grow <= 0 {
		return
	}
	j := k.j
	size := p.size * k.grow
	pat := k.pat
	for i := 4; i < joints; i++ {
		a, b := k.j[i-1], k.j[i]
		dx, dy := a.x-b.x, a.y-b.y
		if d := math.Hypot(dx, dy); d > 0 {
			f := float64(i-3) / (joints - 4)
			off := math.Sin(k.phase-f*2.5) * f * f * k.stroke * 1.8 * size
			j[i].x -= dy / d * off
			j[i].y += dx / d * off
		}
	}

	head := math.Atan2(j[0].y-j[2].y, j[0].x-j[2].x)
	flap := math.Sin(k.phase*0.5) * 0.25
	for _, side := range [2]float64{-1, 1} {
		a := head + math.Pi + side*(1.25+flap)
		for t := range 3 {
			off := (radii[2] + 0.6 + float64(t)) * size
			paint(j[2].x+math.Cos(a)*off, j[2].y+math.Sin(a)*off, (1.5-float64(t)*0.35)*size, pat.body)
		}
	}
	back := math.Atan2(j[7].y-j[6].y, j[7].x-j[6].x)
	for _, side := range [2]float64{-1, 1} {
		a := back + side*1.9
		paint(j[7].x+math.Cos(a)*1.6*size, j[7].y+math.Sin(a)*1.6*size, 0.8*size, pat.body)
	}
	last, before := j[joints-1], j[joints-2]
	tail := math.Atan2(last.y-before.y, last.x-before.x) + math.Sin(k.phase-2)*0.6*k.stroke
	paint(last.x+math.Cos(tail)*1.2*size, last.y+math.Sin(tail)*1.2*size, 0.75*size, pat.body)
	paint(last.x+math.Cos(tail)*0.6*size, last.y+math.Sin(tail)*0.6*size, 0.65*size, pat.body)

	for i := joints - 1; i >= 0; i-- {
		paint(j[i].x, j[i].y, radii[i]*size, pat.body)
		if i > 0 {
			r := (radii[i] + radii[i-1]) / 2 * size
			paint((j[i].x+j[i-1].x)/2, (j[i].y+j[i-1].y)/2, r, pat.body)
		}
	}
	for i := joints - 1; i >= 0; i-- {
		if t := pat.back[i]; t != tagWater {
			paint(j[i].x, j[i].y, radii[i]*size*0.62, t)
		}
	}
	if pat.sheen {
		// A thin stripe of light along the spine, head to the start of
		// the tail, narrowing as the body does.
		for i := 1; i < 9; i++ {
			paint(j[i].x, j[i].y, radii[i]*size*0.3, tagSheen)
			paint((j[i].x+j[i+1].x)/2, (j[i].y+j[i+1].y)/2, (radii[i]+radii[i+1])/2*size*0.3, tagSheen)
		}
	}
	if k.grow < 0.6 {
		return
	}
	r := radii[0] * size * 0.72
	for _, side := range [2]float64{-1, 1} {
		a := head + side*1.1
		paint(j[0].x+math.Cos(a)*r, j[0].y+math.Sin(a)*r, math.Max(0.55, 0.3*size), tagEye)
	}
}
