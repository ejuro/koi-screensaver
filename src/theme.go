package main

import (
	"bufio"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type rgb struct{ r, g, b float64 }

func mix(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

func (c rgb) lum() float64 { return (0.2126*c.r + 0.7152*c.g + 0.0722*c.b) / 255 }

func hex(s string) (rgb, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return rgb{}, false
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return rgb{}, false
	}
	return rgb{float64(v >> 16), float64(v >> 8 & 0xff), float64(v & 0xff)}, true
}

// themeColors reads the current Omarchy theme's colors.toml, falling back to
// an ink-wash night palette for anything it lacks.
func themeColors() map[string]rgb {
	c := map[string]rgb{}
	for k, v := range map[string]string{
		"background": "#16161a", "foreground": "#d8d2c4",
		"red": "#d4523b", "orange": "#c98f6a", "yellow": "#cbb48a",
		"green": "#a3ab8c", "cyan": "#95aba7", "blue": "#929fb0", "magenta": "#b394a3",
	} {
		c[k], _ = hex(v)
	}
	path := os.Getenv("KOI_SCREENSAVER_COLORS")
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".local/state/omarchy/current/theme/colors.toml")
		if _, err := os.Stat(path); err != nil {
			path = filepath.Join(home, ".config/omarchy/current/theme/colors.toml")
		}
	}
	f, err := os.Open(path)
	if err != nil {
		return c
	}
	defer f.Close()
	alias := map[string]string{"color1": "red", "color2": "green", "color3": "yellow", "color4": "blue", "color5": "magenta", "color6": "cyan"}
	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, ok := strings.Cut(sc.Text(), "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		if a, ok := alias[k]; ok {
			if seen[a] {
				continue
			}
			k = a
		}
		if col, ok := hex(strings.Trim(strings.TrimSpace(v), `"'`)); ok {
			c[k] = col
			seen[k] = true
		}
	}
	if !seen["orange"] {
		c["orange"] = mix(c["red"], c["yellow"], 0.5)
	}
	return c
}

// Everything in the pond is one of these flat colours, except the water,
// which is shaded per pixel.
const (
	tagWater   int8 = iota
	tagEye          // dark, on a light or red head
	tagEyePale      // pale, on a head as dark as ink
	// koi
	tagInk
	tagRed
	tagOrange
	tagGold
	tagSheen // the light down a gold koi's back
	// on the surface
	tagPad
	tagPadRim
	tagPadVein
	tagPetal
	tagPetalInner
	tagStamen
	tagCount
)

const firstSurface = tagPad

type palette struct {
	tags                   [tagCount]rgb
	shallow, deep          rgb // the water at the edge and in the middle
	ripple, trough, shadow rgb
	bg                     rgb // the theme's background, to fade in from
	koiShadow              rgb // what a shadow darkens a koi towards
	dark                   bool
}

func newPalette(c map[string]rgb) palette {
	var p palette
	bg, fg := c["background"], c["foreground"]
	p.dark = bg.lum() < 0.5
	p.bg = bg
	black := rgb{0, 0, 0}
	lightest, darkest := bg, fg
	if p.dark {
		lightest, darkest = fg, black
	}
	tint := mix(c["blue"], c["cyan"], 0.5)
	p.shallow = mix(bg, tint, 0.10)
	if p.dark {
		p.deep = mix(mix(bg, tint, 0.06), black, 0.35)
		p.shadow = mix(p.deep, black, 0.3)
		p.ripple = mix(bg, fg, 0.7)
	} else {
		p.deep = mix(p.shallow, mix(tint, fg, 0.3), 0.12)
		p.shadow = mix(p.deep, fg, 0.14)
		p.ripple = mix(bg, fg, 0.5)
	}
	p.trough = mix(p.deep, darkest, 0.25)
	p.koiShadow = black

	// Eyes: dark ink, or nearly the lightest colour where the head itself
	// is dark.
	if p.dark {
		p.tags[tagEye] = mix(p.deep, darkest, 0.4)
		p.tags[tagEyePale] = mix(bg, fg, 0.9)
	} else {
		p.tags[tagEye] = mix(fg, black, 0.2)
		p.tags[tagEyePale] = mix(fg, bg, 0.9)
	}
	p.tags[tagInk] = fg
	p.tags[tagRed] = c["red"]
	p.tags[tagOrange] = c["orange"]
	p.tags[tagGold] = c["yellow"]
	// Dark themes have a pale gold already, so its sheen needs more light.
	sheen := 0.4
	if p.dark {
		sheen = 0.6
	}
	p.tags[tagSheen] = mix(c["yellow"], lightest, sheen)
	green := c["green"]
	// The pad as on the Sumi wallpapers: the green a little washed into the
	// water, a thin darker edge, paler veins.
	pad := mix(green, bg, 0.17)
	p.tags[tagPad] = pad
	p.tags[tagPadRim] = mix(pad, darkest, 0.22)
	p.tags[tagPadVein] = mix(pad, lightest, 0.2)
	pink := mix(c["magenta"], c["red"], 0.35)
	p.tags[tagPetal] = mix(pink, lightest, 0.35)
	p.tags[tagPetalInner] = mix(pink, lightest, 0.65)
	p.tags[tagStamen] = c["yellow"]
	return p
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
