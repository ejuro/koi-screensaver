package main

import (
	"cmp"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The intro: Omarchy's wordmark is painted on to the pond in one stroke, left
// to right, and rests a moment. Then each letter breaks into motes that
// stream together and settle into the shape of a koi, and once they have all
// landed the koi shows in their place and swims away.

const introHold = 3.0 // seconds the wordmark rests before it breaks up

type intro struct {
	t        float64
	hw, hh   float64 // half a wordmark pixel, across and down
	motes    []mote
	release  []float64 // when each koi starts to swim
	centre   []pt      // where each koi's letter was
	splashed []bool
	sum      []float64
	count    []int
	shown    []float64 // when each koi took over from its motes, or 0
	end      float64
	x0, x1   float64 // the wordmark's left and right, for the stroke
	y0, y1   float64
}

const (
	strokeTime = 1.3  // seconds the stroke takes to cross the wordmark
	handOver   = 0.35 // seconds the motes take to fade into the koi
)

// mote is one pixel of the wordmark, on its way to a point on a koi.
type mote struct {
	hx, hy     float64 // its place in the wordmark
	k          int     // the koi it becomes part of
	n          int     // the joint it flows to
	f          float64 // and how far on towards the next one, 0 to 1
	u          float64 // and how far across the body there, -1 to 1
	tag        int8
	start, dur float64
	curl       float64 // how far it swings out on the way, in path lengths
}

type lpx struct{ x, y int }

// logoBits reads the text Omarchy's own screensaver shows, its wordmark unless
// the user has rebranded it, as a bitmap: each character holds two pixels,
// one above the other.
func logoBits() [][]bool {
	home, _ := os.UserHomeDir()
	paths := []string{filepath.Join(home, ".config/omarchy/branding/screensaver.txt")}
	if d := os.Getenv("OMARCHY_PATH"); d != "" {
		paths = append(paths, filepath.Join(d, "logo.txt"))
	}
	paths = append(paths, filepath.Join(home, ".local/share/omarchy/logo.txt"), "/usr/share/omarchy/logo.txt")
	for _, path := range paths {
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
		w := 0
		for _, l := range lines {
			w = max(w, len([]rune(l)))
		}
		bits := make([][]bool, 2*len(lines))
		for i := range bits {
			bits[i] = make([]bool, w)
		}
		for r, l := range lines {
			for c, ch := range []rune(l) {
				switch ch {
				case ' ', '\t', '\r':
				case '▀':
					bits[2*r][c] = true
				case '▄':
					bits[2*r+1][c] = true
				default:
					bits[2*r][c], bits[2*r+1][c] = true, true
				}
			}
		}
		return bits
	}
	return nil
}

// letters splits the wordmark into its letters, left to right: the pieces
// whose pixels touch.
func letters(bits [][]bool) [][]lpx {
	h, w := len(bits), len(bits[0])
	seen := make([]bool, w*h)
	var parts [][]lpx
	for y := range h {
		for x := range w {
			if !bits[y][x] || seen[y*w+x] {
				continue
			}
			seen[y*w+x] = true
			stack := []lpx{{x, y}}
			var part []lpx
			for len(stack) > 0 {
				q := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				part = append(part, q)
				for _, d := range [4]lpx{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
					nx, ny := q.x+d.x, q.y+d.y
					if nx >= 0 && nx < w && ny >= 0 && ny < h && bits[ny][nx] && !seen[ny*w+nx] {
						seen[ny*w+nx] = true
						stack = append(stack, lpx{nx, ny})
					}
				}
			}
			parts = append(parts, part)
		}
	}

	// A dot, an accent or a crumb joins the letter it sits over, or the
	// nearest one.
	span := func(part []lpx) (lo, hi int) {
		lo, hi = w, -1
		for _, q := range part {
			lo, hi = min(lo, q.x), max(hi, q.x)
		}
		return
	}
	for len(parts) > 1 {
		slices.SortFunc(parts, func(a, b []lpx) int { return cmp.Compare(len(a), len(b)) })
		total := 0
		for _, part := range parts {
			total += len(part)
		}
		small := parts[0]
		lo, hi := span(small)
		best, bestOver, bestGap := -1, 0, w
		for i := 1; i < len(parts); i++ {
			lo2, hi2 := span(parts[i])
			over := min(hi, hi2) - max(lo, lo2) + 1
			gap := max(lo2-hi, lo-hi2)
			if over > bestOver || (bestOver == 0 && gap < bestGap) {
				best, bestOver, bestGap = i, max(over, 0), gap
			}
		}
		if 2*bestOver < hi-lo+1 && 4*len(small)*len(parts) >= total {
			break
		}
		parts[best] = append(parts[best], small...)
		parts = parts[1:]
	}
	slices.SortFunc(parts, func(a, b []lpx) int {
		la, _ := span(a)
		lb, _ := span(b)
		return cmp.Compare(la, lb)
	})
	return parts
}

// shareOut gives each koi a part of the wordmark, left to right: a letter
// each, any spare koi to the biggest letters, or neighbouring letters
// sharing a koi when there are fewer koi than letters.
func shareOut(ls [][]lpx, n int) [][]lpx {
	if n < len(ls) {
		groups := make([][]lpx, n)
		for i, l := range ls {
			j := i * n / len(ls)
			groups[j] = append(groups[j], l...)
		}
		return groups
	}
	count := make([]int, len(ls))
	for i := range count {
		count[i] = 1
	}
	for range n - len(ls) {
		best := 0
		for i := range ls {
			if len(ls[i])*count[best] > len(ls[best])*count[i] {
				best = i
			}
		}
		count[best]++
	}
	var groups [][]lpx
	for i, l := range ls {
		l = slices.Clone(l)
		slices.SortFunc(l, func(a, b lpx) int { return cmp.Or(cmp.Compare(a.x, b.x), cmp.Compare(a.y, b.y)) })
		for c := range count[i] {
			groups = append(groups, l[c*len(l)/count[i]:(c+1)*len(l)/count[i]])
		}
	}
	return groups
}

// startIntro lays the wordmark over the middle of the pond and hides each
// koi inside a letter, to be gathered up out of it. It does nothing if
// there's no wordmark to read or the pond is too small to draw it.
func (p *pond) startIntro() {
	bits := logoBits()
	if len(bits) == 0 || len(bits[0]) == 0 {
		return
	}
	ls := letters(bits)
	if len(ls) == 0 {
		return
	}
	minX, maxX, minY, maxY := math.MaxInt, -1, math.MaxInt, -1
	for _, l := range ls {
		for _, q := range l {
			minX, maxX = min(minX, q.x), max(maxX, q.x)
			minY, maxY = min(minY, q.y), max(maxY, q.y)
		}
	}
	lw, lh := maxX-minX+1, maxY-minY+1

	// A wordmark pixel is a terminal cell wide and half a cell tall: a by b
	// pond pixels, whole ones, so the letters stay crisp.
	a := int(math.Min(0.6*float64(p.pw)/float64(lw), 0.3*float64(p.ph)/(0.75*float64(lh))))
	if a < 1 {
		return
	}
	b := max(1, int(math.Round(0.75*float64(a))))
	ox := (p.pw-lw*a)/2 - minX*a
	oy := (p.ph-lh*b)/2 - minY*b
	in := &intro{hw: float64(a) / 2, hh: float64(b) * p.aspect / 2}
	in.x0, in.x1 = float64(ox+minX*a), float64(ox+(maxX+1)*a)
	in.y0, in.y1 = float64(oy+minY*b)*p.aspect, float64(oy+(maxY+1)*b)*p.aspect
	home := func(q lpx) pt {
		return pt{float64(ox+q.x*a) + in.hw, float64(oy+q.y*b)*p.aspect + in.hh}
	}

	groups := shareOut(ls, len(p.koi))
	n := len(groups)
	in.release = make([]float64, n)
	in.centre = make([]pt, n)
	in.splashed = make([]bool, n)
	in.sum = make([]float64, n)
	in.count = make([]int, n)
	in.shown = make([]float64, n)

	var cum [joints]float64
	total := 0.0
	for j, r := range radii {
		total += r
		cum[j] = total
	}
	for i, g := range groups {
		if len(g) == 0 {
			continue // a letter too small to share with every koi it was given
		}
		k := p.koi[i]
		var c pt
		for _, q := range g {
			h := home(q)
			c.x += h.x
			c.y += h.y
		}
		c.x /= float64(len(g))
		c.y /= float64(len(g))
		in.centre[i] = c

		// Lie along the letter, head up or down by turns, the middle of
		// the body over the middle of the letter.
		heading := -math.Pi / 2
		if i%2 == 1 {
			heading = math.Pi / 2
		}
		dx, dy := math.Cos(heading), math.Sin(heading)
		gap := seg * p.size
		for j := range joints {
			d := gap * float64(6-j)
			k.j[j] = pt{c.x + dx*d, c.y + dy*d}
		}
		k.heading, k.wander = heading, heading
		k.wanderT = 3 + p.rng.Float64()*3
		k.burst = 0.6
		k.grow, k.ease = 0, 0

		// Match the letter's pixels to points on the koi, front to front
		// and side to side, so each letter folds into its fish rather
		// than scrambling.
		type spot struct {
			along, across float64
			j             int
			f, u          float64
		}
		m := len(g)
		targets := make([]spot, m)
		for t := range targets {
			r := p.rng.Float64() * total
			j := 0
			for cum[j] < r {
				j++
			}
			u := p.rng.Float64()*2 - 1
			f := p.rng.Float64()
			targets[t] = spot{along: -float64(j) - f, across: u, j: j, f: f, u: u}
		}
		sources := make([]spot, m)
		for s, q := range g {
			h := home(q)
			rx, ry := h.x-c.x, h.y-c.y
			sources[s] = spot{along: rx*dx + ry*dy, across: -rx*dy + ry*dx, j: s}
		}
		byAlong := func(a, b spot) int { return cmp.Compare(b.along, a.along) }
		byAcross := func(a, b spot) int { return cmp.Compare(a.across, b.across) }
		slices.SortFunc(targets, byAlong)
		slices.SortFunc(sources, byAlong)
		bands := max(1, int(math.Sqrt(float64(m))/2))
		curl := 0.15 + 0.1*p.rng.Float64()
		if p.rng.IntN(2) == 0 {
			curl = -curl
		}
		start := introHold + 0.22*float64(i)
		in.release[i] = start + 0.3
		for band := range bands {
			lo, hi := band*m/bands, (band+1)*m/bands
			slices.SortFunc(targets[lo:hi], byAcross)
			slices.SortFunc(sources[lo:hi], byAcross)
			for s := lo; s < hi; s++ {
				h := home(g[sources[s].j])
				t := targets[s]
				mt := mote{
					hx: h.x, hy: h.y, k: i, n: t.j, f: t.f, u: t.u, tag: k.tagAt(t.j, t.u),
					start: start + 0.8*float64(band)/float64(bands) + 0.3*p.rng.Float64(),
					dur:   1.4 + 0.6*p.rng.Float64(),
					curl:  curl * (0.7 + 0.6*p.rng.Float64()),
				}
				in.end = math.Max(in.end, mt.start+mt.dur)
				in.motes = append(in.motes, mt)
				in.count[i]++
			}
		}
	}
	if c := p.chase; c != nil && len(groups) > 0 && in.count[0] > 0 {
		// The chase sets off from where the first koi's letters were, and
		// waits for it.
		c.theta = math.Atan2(in.centre[0].y-p.h/2, in.centre[0].x-p.w/2)
		c.started = false
	}
	p.intro = in
	p.rings = p.rings[:0]
}

// tagAt is the colour of a koi at joint n, u of the way across its body.
func (k *koi) tagAt(n int, u float64) int8 {
	if k.pat.sheen && n >= 1 && n < 9 && math.Abs(u) < 0.3 {
		return tagSheen
	}
	if t := k.pat.back[n]; t != tagWater && math.Abs(u) < 0.62 {
		return t
	}
	return k.pat.body
}

// bodyPoint is the point at joint n, u of the way across a koi drawn at size.
func (k *koi) bodyPoint(n int, f, u, size float64) pt {
	m := min(joints-1, n+1)
	c := pt{k.j[n].x + (k.j[m].x-k.j[n].x)*f, k.j[n].y + (k.j[m].y-k.j[n].y)*f}
	a, b := k.j[max(0, n-1)], k.j[min(joints-1, n+2)]
	dx, dy := a.x-b.x, a.y-b.y
	d := math.Hypot(dx, dy)
	if d == 0 {
		return c
	}
	r := u * (radii[n] + (radii[m]-radii[n])*f) * size * 0.85
	return pt{c.x - dy/d*r, c.y + dx/d*r}
}

func smooth(v float64) float64 {
	v = clamp(v, 0, 1)
	return v * v * (3 - 2*v)
}

func (p *pond) stepIntro(dt float64) {
	in := p.intro
	in.t += dt
	clear(in.sum)
	for i := range in.motes {
		m := &in.motes[i]
		in.sum[m.k] += clamp((in.t-m.start)/m.dur, 0, 1)
	}
	for i, k := range p.koi {
		if i >= len(in.count) || in.count[i] == 0 {
			k.grow, k.ease = 1, 1
			continue
		}
		// The koi stays hidden while its motes gather into its shape, and
		// takes over once they have all landed.
		if in.shown[i] == 0 && in.sum[i] >= 0.97*float64(in.count[i]) {
			in.shown[i] = in.t
		}
		k.grow = 0
		if in.shown[i] > 0 {
			k.grow = 1
		}
		k.ease = smooth((in.t - in.release[i]) / 2.5)
		if !in.splashed[i] && in.t >= in.release[i]-0.3 {
			// The letter lifts off the water.
			in.splashed[i] = true
			c := in.centre[i]
			p.addRing(c.x, c.y, 2*p.size, 6*p.size, 0.7, 2.6, 1.0*p.size)
		}
	}
	done := in.t >= in.end && in.t >= in.release[len(in.release)-1]+2.5
	for i, s := range in.shown {
		if in.count[i] > 0 && (s == 0 || in.t < s+handOver) {
			done = false
		}
	}
	if done {
		for _, k := range p.koi {
			k.grow, k.ease = 1, 1
		}
		p.intro = nil
	}
}

// moteAt is where a mote is now, its half size across and down, and how
// far along its way it is, 0 to 1.
func (p *pond) moteAt(m *mote) (x, y, hw, hh, pr float64) {
	in := p.intro
	pr = clamp((in.t-m.start)/m.dur, 0, 1)
	if pr == 0 {
		return m.hx, m.hy, in.hw, in.hh, 0
	}
	k := p.koi[m.k]
	t := k.bodyPoint(m.n, m.f, m.u, p.size)
	e := smooth(pr)
	dx, dy := t.x-m.hx, t.y-m.hy
	arc := math.Sin(math.Pi*e) * m.curl
	x = m.hx + dx*e - dy*arc
	y = m.hy + dy*e + dx*arc
	s := 0.5 * p.size
	hw = in.hw + (math.Max(s, 0.55)-in.hw)*e
	hh = in.hh + (math.Max(s, 0.55*p.aspect)-in.hh)*e
	return
}

// rect calls set for every pixel whose centre lies in the box hw either
// side of cx and hh above and below cy.
func (p *pond) rect(cx, cy, hw, hh float64, set func(i int)) {
	x0 := max(0, int(math.Ceil(cx-hw-0.5)))
	x1 := min(p.pw-1, int(math.Ceil(cx+hw-0.5))-1)
	y0 := max(0, int(math.Ceil((cy-hh)/p.aspect-0.5)))
	y1 := min(p.ph-1, int(math.Ceil((cy+hh)/p.aspect-0.5))-1)
	for py := y0; py <= y1; py++ {
		for px := x0; px <= x1; px++ {
			set(py*p.pw + px)
		}
	}
}

// introShadows lets the floating wordmark, and the motes flying off it,
// shade the water below.
func (p *pond) introShadows(ox, oy float64) {
	in := p.intro
	if in.t < 0.5 {
		return
	}
	for i := range in.motes {
		m := &in.motes[i]
		x, y, hw, hh, pr := p.moteAt(m)
		// Only what the brush has painted casts a shadow.
		if pr < 0.8 && (pr > 0 || in.painted(m) > 0.5) {
			p.rect(x+ox, y+oy, hw, hh, func(i int) { p.shade[i] = -1 })
		}
	}
}

// painted is how far the brush has painted in a pixel of the wordmark, 0 to
// 1: it crosses left to right in one stroke, a little later towards the
// bottom, with a slightly ragged edge.
func (in *intro) painted(m *mote) float64 {
	across := (m.hx - in.x0) / math.Max(in.x1-in.x0, 1)
	down := (m.hy - in.y0) / math.Max(in.y1-in.y0, 1)
	h := math.Sin(m.hx*12.9898+m.hy*0.7) * 43758.5453
	ragged := h - math.Floor(h)
	return smooth((in.t - 0.15 - strokeTime*across - 0.18*down - 0.12*ragged) / 0.22)
}

// drawIntro draws the wordmark and its motes over the finished pond.
func (p *pond) drawIntro() {
	in := p.intro
	pal := &p.pal
	word := pal.tags[tagInk]
	for i := range in.motes {
		m := &in.motes[i]
		x, y, hw, hh, pr := p.moteAt(m)
		alpha := in.painted(m)
		c := word
		if pr > 0 {
			c = mix(word, pal.tags[m.tag], smooth((pr-0.15)/0.6))
			alpha = 1
			// It stays, part of the koi's shape, until the koi takes over.
			if s := in.shown[m.k]; s > 0 {
				alpha = 1 - smooth((in.t-s)/handOver)
			}
		}
		if alpha <= 0 {
			continue
		}
		p.rect(x, y, hw, hh, func(i int) { p.out[i] = mix(p.out[i], c, alpha) })
	}
}
