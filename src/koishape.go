package main

import "math"

// A koi drawn smooth, as on the Sumi koi wallpaper: the same spine, widths,
// fins and patches as the discs it was once drawn with, but the body runs
// joint to joint as one tapering outline, the fins and tail grow out of it
// with soft joins, and the patches run together into soft blobs.
//
// Each part is worked out as a distance, below zero inside it, on a grid of
// pixels just big enough for the koi, and only near the part itself.

type koiShape struct {
	x0, y0, w, h int       // the pixels it covers
	sil, patch   []float32 // the whole silhouette, and the patches
	sheen        []float32 // a gold koi's stripe; nil for none
	patchTag     int8
	eyes         [2]pt
	eyeR         float64 // 0 while the koi is too small for eyes
}

const far = float32(1e9)

// smin joins two distances with a soft fillet k wide.
func smin(a, b, k float32) float32 {
	h := 0.5 + 0.5*(b-a)/k
	h = min(max(h, 0), 1)
	return b*(1-h) + a*h - k*h*(1-h)
}

type disc struct{ x, y, r float64 }

// shape works out koi k as drawn this frame, swaying with its stroke.
func (p *pond) shape(k *koi) *koiShape {
	sz := p.size * k.grow
	j := k.j
	for i := 4; i < joints; i++ {
		a, b := k.j[i-1], k.j[i]
		dx, dy := a.x-b.x, a.y-b.y
		if d := math.Hypot(dx, dy); d > 0 {
			f := float64(i-3) / (joints - 4)
			off := math.Sin(k.phase-f*2.5) * f * f * k.stroke * 1.8 * sz
			j[i].x -= dy / d * off
			j[i].y += dx / d * off
		}
	}

	// The parts: paddle fins behind the head (three discs each), small fins
	// further back, and the round tail.
	var paddles [2][3]disc
	head := math.Atan2(j[0].y-j[2].y, j[0].x-j[2].x)
	flap := math.Sin(k.phase*0.5) * 0.25
	for s, side := range [2]float64{-1, 1} {
		a := head + math.Pi + side*(1.25+flap)
		for t := range 3 {
			off := (radii[2] + 0.6 + float64(t)) * sz
			paddles[s][t] = disc{j[2].x + math.Cos(a)*off, j[2].y + math.Sin(a)*off, (1.5 - float64(t)*0.35) * sz}
		}
	}
	var small [2]disc
	back := math.Atan2(j[7].y-j[6].y, j[7].x-j[6].x)
	for s, side := range [2]float64{-1, 1} {
		a := back + side*1.9
		small[s] = disc{j[7].x + math.Cos(a)*1.6*sz, j[7].y + math.Sin(a)*1.6*sz, 0.8 * sz}
	}
	last, before := j[joints-1], j[joints-2]
	ta := math.Atan2(last.y-before.y, last.x-before.x) + math.Sin(k.phase-2)*0.6*k.stroke
	tail := [2]disc{
		{last.x + math.Cos(ta)*1.2*sz, last.y + math.Sin(ta)*1.2*sz, 0.75 * sz},
		{last.x + math.Cos(ta)*0.6*sz, last.y + math.Sin(ta)*0.6*sz, 0.65 * sz},
	}

	// Just the pixels the parts can reach.
	x0, y0, x1, y1 := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	reach := func(c disc) {
		x0, y0 = math.Min(x0, c.x-c.r), math.Min(y0, c.y-c.r)
		x1, y1 = math.Max(x1, c.x+c.r), math.Max(y1, c.y+c.r)
	}
	for i, q := range j {
		reach(disc{q.x, q.y, radii[i] * sz})
	}
	for s := range 2 {
		for t := range 3 {
			reach(paddles[s][t])
		}
		reach(small[s])
		reach(tail[s])
	}
	s := k.shapeBuf
	if s == nil {
		s = &koiShape{}
		k.shapeBuf = s
	}
	s.x0 = max(0, int(math.Floor(x0-2)))
	s.y0 = max(0, int(math.Floor((y0-2)/p.aspect)))
	s.w = min(p.pw, int(math.Ceil(x1+2))) - s.x0
	s.h = min(p.ph, int(math.Ceil((y1+2)/p.aspect))) - s.y0
	if s.w <= 0 || s.h <= 0 {
		s.w, s.h = 0, 0
		return s
	}
	n := s.w * s.h
	clean := func(b []float32) []float32 {
		if cap(b) < n {
			b = make([]float32, n)
		}
		b = b[:n]
		for i := range b {
			b[i] = far
		}
		return b
	}
	s.sil = clean(s.sil)
	fins := clean(p.koiTmp)
	p.koiTmp = fins

	// The body, nose to tail, straight into the silhouette.
	p.shapeDisc(s, s.sil, disc{j[0].x, j[0].y, radii[0] * sz}, 0)
	for i := 0; i < joints-1; i++ {
		p.shapeSegment(s, s.sil, j[i], j[i+1], radii[i]*sz, radii[i+1]*sz)
	}
	// The fins and tail, each run together from its discs.
	for side := range 2 {
		for t := range 3 {
			p.shapeDisc(s, fins, paddles[side][t], 0.5*sz)
		}
		p.shapeDisc(s, fins, small[side], 0.3*sz)
		p.shapeDisc(s, fins, tail[side], 0.3*sz)
	}
	// ... and grown out of the body with a soft join, where they meet it.
	join := float32(0.35 * sz)
	for i, f := range fins {
		if b := s.sil[i]; f < b+join {
			s.sil[i] = smin(b, f, join)
		}
	}

	// Patches over chosen joints, running together where they meet.
	s.patchTag = tagWater
	s.patch = clean(s.patch)
	for i := joints - 1; i >= 0; i-- {
		if t := k.pat.back[i]; t != tagWater {
			s.patchTag = t
			p.shapeDisc(s, s.patch, disc{j[i].x, j[i].y, radii[i] * sz * 0.62}, 1.0*sz)
		}
	}
	s.sheen = s.sheen[:0]
	if k.pat.sheen {
		// A thin stripe of light along the spine, head to the start of
		// the tail, narrowing as the body does.
		s.sheen = clean(s.sheen)
		for i := 1; i < 9; i++ {
			p.shapeSegment(s, s.sheen, j[i], j[i+1], radii[i]*sz*0.3, radii[i+1]*sz*0.3)
		}
	}
	s.eyeR = 0
	if k.grow >= 0.6 {
		r := radii[0] * sz * 0.72
		s.eyeR = math.Max(0.55, 0.3*sz)
		for e, side := range [2]float64{-1, 1} {
			a := head + side*1.1
			s.eyes[e] = pt{j[0].x + math.Cos(a)*r, j[0].y + math.Sin(a)*r}
		}
	}
	return s
}

// shapeDisc runs a disc into buf, with a soft join k wide (0 for none).
func (p *pond) shapeDisc(s *koiShape, buf []float32, c disc, k float64) {
	e := c.r + k + 1
	px0 := max(s.x0, int(math.Floor(c.x-e)))
	px1 := min(s.x0+s.w-1, int(math.Ceil(c.x+e)))
	py0 := max(s.y0, int(math.Floor((c.y-e)/p.aspect)))
	py1 := min(s.y0+s.h-1, int(math.Ceil((c.y+e)/p.aspect)))
	kk := float32(k)
	for py := py0; py <= py1; py++ {
		dy := (float64(py)+0.5)*p.aspect - c.y
		row := (py-s.y0)*s.w - s.x0
		for px := px0; px <= px1; px++ {
			dx := float64(px) + 0.5 - c.x
			d := float32(math.Sqrt(dx*dx+dy*dy) - c.r)
			i := row + px
			if k > 0 {
				if d < buf[i]+kk {
					buf[i] = smin(buf[i], d, kk)
				}
			} else if d < buf[i] {
				buf[i] = d
			}
		}
	}
}

// shapeSegment runs a stretch of spine into buf, ra wide at a tapering to
// rb at b.
func (p *pond) shapeSegment(s *koiShape, buf []float32, a, b pt, ra, rb float64) {
	vx, vy := b.x-a.x, b.y-a.y
	l2 := vx*vx + vy*vy
	inv := 0.0
	if l2 > 0 {
		inv = 1 / l2
	}
	r := math.Max(ra, rb) + 1
	px0 := max(s.x0, int(math.Floor(math.Min(a.x, b.x)-r)))
	px1 := min(s.x0+s.w-1, int(math.Ceil(math.Max(a.x, b.x)+r)))
	py0 := max(s.y0, int(math.Floor((math.Min(a.y, b.y)-r)/p.aspect)))
	py1 := min(s.y0+s.h-1, int(math.Ceil((math.Max(a.y, b.y)+r)/p.aspect)))
	for py := py0; py <= py1; py++ {
		ay := (float64(py)+0.5)*p.aspect - a.y
		row := (py-s.y0)*s.w - s.x0
		for px := px0; px <= px1; px++ {
			ax := float64(px) + 0.5 - a.x
			t := (ax*vx + ay*vy) * inv
			if t < 0 {
				t = 0
			} else if t > 1 {
				t = 1
			}
			ex, ey := ax-t*vx, ay-t*vy
			d := float32(math.Sqrt(ex*ex+ey*ey) - (ra + t*(rb-ra)))
			if i := row + px; d < buf[i] {
				buf[i] = d
			}
		}
	}
}

// castKoiShadow lays a koi's shadow, its silhouette moved by (ox, oy) world
// units, on everything below depth d.
func (p *pond) castKoiShadow(s *koiShape, ox, oy float64, d float32) {
	sx, sy := int(math.Round(ox)), int(math.Round(oy/p.aspect))
	for y := 0; y < s.h; y++ {
		ty := s.y0 + y + sy
		if ty < 0 || ty >= p.ph {
			continue
		}
		for x := 0; x < s.w; x++ {
			if s.sil[y*s.w+x] >= 0 {
				continue
			}
			tx := s.x0 + x + sx
			if tx < 0 || tx >= p.pw {
				continue
			}
			i := ty*p.pw + tx
			p.shade[i] = min(p.shade[i], d)
		}
	}
}

// fillKoi paints a koi's pixels: body, patches, sheen and eyes.
func (p *pond) fillKoi(s *koiShape, body int8, d float32) {
	for y := 0; y < s.h; y++ {
		row := (s.y0+y)*p.pw + s.x0
		for x := 0; x < s.w; x++ {
			l := y*s.w + x
			if s.sil[l] >= 0 {
				continue
			}
			t := body
			if s.patch[l] < 0 {
				t = s.patchTag
			}
			if len(s.sheen) > 0 && s.sheen[l] < 0 {
				t = tagSheen
			}
			p.tag[row+x] = t
			p.depth[row+x] = d
		}
	}
	if s.eyeR > 0 {
		for _, e := range s.eyes {
			p.fill(e.x, e.y, s.eyeR, func(i int, _, _ float64) { p.tag[i] = tagEye })
		}
	}
}
