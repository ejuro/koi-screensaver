// koi-screensaver: a slow, quiet koi pond for the terminal. Koi drift under lily pads
// and water lilies, and petals float by. Coloured from the current Omarchy
// theme.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	screensaverClass = "org.omarchy.screensaver"
	fadeSeconds      = 3.0
)

func main() {
	screensaver := flag.Bool("screensaver", false, "run as the Omarchy screensaver: hide the cursor, wake on mouse, close every screensaver window on exit")
	fps := flag.Float64("fps", 15, "frames per second; a whole fraction of the screen's refresh rate keeps the motion even")
	seed := flag.Uint64("seed", uint64(time.Now().UnixNano()), "random seed")
	intro := flag.Bool("intro", true, "open with the Omarchy wordmark turning into the koi")
	scene := flag.String("scene", scenePond, "what to show: pond (koi drifting under lily pads) or chase (two koi circling a lily pad)")
	flag.Parse()
	if *scene != scenePond && *scene != sceneChase {
		*scene = scenePond
	}

	if err := run(*screensaver, *intro, *scene, *fps, *seed); err != nil {
		fmt.Fprintln(os.Stderr, "koi-screensaver:", err)
		os.Exit(1)
	}
}

// The largest pond it will draw: about 300 MB of buffers. A bigger terminal
// needs a bigger font (KOI_SCREENSAVER_FONT_SIZE).
const (
	maxCols  = 3000
	maxRows  = 1500
	maxCells = 600_000
)

func checkGrid(cols, rows int) error {
	if cols < 8 || rows < 4 {
		return fmt.Errorf("terminal of %dx%d cells is too small", cols, rows)
	}
	if cols > maxCols || rows > maxRows || cols*rows > maxCells {
		return fmt.Errorf("terminal of %dx%d cells is too large; use a bigger font (KOI_SCREENSAVER_FONT_SIZE)", cols, rows)
	}
	return nil
}

func aspectOf(cw, ch int) float64 {
	if cw <= 0 || ch <= 0 {
		return 1.45
	}
	return (float64(ch) / 3) / (float64(cw) / 2)
}

func run(screensaver, intro bool, scene string, fps float64, seed uint64) error {
	if math.IsNaN(fps) || math.IsInf(fps, 0) {
		fps = 15
	}
	fps = clamp(fps, 1, 60)
	ctx, stopSignals := signal.NotifyContext(context.Background(), unix.SIGINT, unix.SIGTERM, unix.SIGHUP, unix.SIGQUIT)
	defer stopSignals()
	fd := int(os.Stdin.Fd())
	old, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return fmt.Errorf("not a terminal: %w", err)
	}
	raw := *old
	raw.Lflag &^= unix.ECHO | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Iflag &^= unix.IXON | unix.ICRNL
	raw.Cc[unix.VMIN], raw.Cc[unix.VTIME] = 1, 0
	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return err
	}

	defer unix.IoctlSetTermios(fd, unix.TCSETS, old)

	out := os.Stdout
	setup := "\x1b[?1049h\x1b[?25l\x1b[2J"
	teardown := "\x1b[0m\x1b[2J\x1b[?25h\x1b[?1049l"
	if screensaver {
		// Report every mouse movement, so a nudge wakes the screen.
		setup += "\x1b[?1003h\x1b[?1006h"
		teardown = "\x1b[?1003l\x1b[?1006l" + teardown
	}
	defer func() {
		out.WriteString(teardown)
		if screensaver {
			setCursorHidden(context.Background(), false)
			// Closing one screensaver closes them all, as Omarchy's does.
			exec.Command("pkill", "-f", "["+screensaverClass[:1]+"]"+screensaverClass[1:]).Run()
		}
	}()

	if screensaver {
		setCursorHidden(ctx, true)
	}
	if ctx.Err() != nil {
		return nil
	}
	out.WriteString(setup)

	quit := make(chan struct{}, 1)
	stop := func() {
		select {
		case quit <- struct{}{}:
		default:
		}
	}
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, unix.SIGWINCH)
	defer signal.Stop(sig)

	start := time.Now()
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				stop()
				return
			}
			// Ignore what arrives while the window is still settling.
			if n > 0 && time.Since(start) > 700*time.Millisecond {
				stop()
				return
			}
		}
	}()
	if screensaver {
		go func() {
			for range time.Tick(time.Second) {
				if time.Since(start) > 2*time.Second && !screensaverFocused() {
					stop()
					return
				}
			}
		}()
	}

	// Terminals open at 80x24 and resize once the compositor has placed
	// the window; wait a moment for that before laying out the pond.
	cols, rows, cw, ch := winsize(fd)
	for wait := time.Now(); cols == 80 && rows == 24 && time.Since(wait) < 2*time.Second; {
		select {
		case <-ctx.Done():
			return nil
		case <-quit:
			return nil
		case <-time.After(20 * time.Millisecond):
		}
		cols, rows, cw, ch = winsize(fd)
	}
	if ctx.Err() != nil {
		return nil
	}
	if err := checkGrid(cols, rows); err != nil {
		return err
	}
	pal := newPalette(themeColors())
	p := newPond(cols, rows, aspectOf(cw, ch), pal, seed, scene)
	p.fade = 0 // rise gently out of the background, over a few seconds
	if intro {
		p.startIntro()
	}
	cur := make([]cell, cols*rows)
	prev := make([]cell, cols*rows)
	invalidate := func() {
		for i := range prev {
			prev[i] = cell{mask: 255}
		}
	}
	invalidate()

	// Move by the time that really passed, so a frame that runs late (a big
	// pond at a small font) doesn't play the pond in slow motion. Capped,
	// so a stall doesn't make the koi jump.
	last := time.Now()
	tick := time.NewTicker(time.Duration(float64(time.Second) / fps))
	defer tick.Stop()
	var buf []byte
	for {
		select {
		case <-quit:
			return nil
		case <-ctx.Done():
			return nil
		case <-sig:
			c, r, w, h := winsize(fd)
			if c != cols || r != rows {
				if err := checkGrid(c, r); err != nil {
					return err
				}
				cols, rows = c, r
				p = newPond(cols, rows, aspectOf(w, h), pal, seed+1, scene)
				p.fade = 0
				cur = make([]cell, cols*rows)
				prev = make([]cell, cols*rows)
				out.WriteString("\x1b[2J")
			}
			invalidate()
		case now := <-tick.C:
			dt := min(now.Sub(last).Seconds(), 3/fps)
			last = now
			p.fade = math.Min(1, p.fade+dt/fadeSeconds)
			p.step(dt)
			p.draw()
			p.cells(cur)
			buf = frame(buf[:0], cur, prev, cols)
			if _, err := out.Write(buf); err != nil {
				return nil
			}
		}
	}
}

func winsize(fd int) (cols, rows, cw, ch int) {
	ws, err := unix.IoctlGetWinsize(fd, unix.TIOCGWINSZ)
	if err != nil || ws.Col == 0 || ws.Row == 0 {
		return 80, 24, 0, 0
	}
	cols, rows = int(ws.Col), int(ws.Row)
	if ws.Xpixel > 0 && ws.Ypixel > 0 {
		cw, ch = int(ws.Xpixel)/cols, int(ws.Ypixel)/rows
	}
	return
}

// hypr runs hyprctl, giving up after two seconds rather than hang.
func hypr(parent context.Context, args ...string) error {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "hyprctl", args...).Run()
}

// setCursorHidden hides or shows the mouse pointer, as Omarchy's own
// screensaver does, with the same fallback for older Hyprland.
func setCursorHidden(ctx context.Context, hidden bool) {
	if hypr(ctx, "eval", fmt.Sprintf("hl.config({ cursor = { invisible = %t } })", hidden)) != nil && ctx.Err() == nil {
		hypr(ctx, "keyword", "cursor:invisible", fmt.Sprint(hidden))
	}
}

func screensaverFocused() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	b, err := exec.CommandContext(ctx, "hyprctl", "activewindow", "-j").Output()
	if err != nil {
		return true // without Hyprland, keep going
	}
	var win struct {
		Class string `json:"class"`
	}
	if json.Unmarshal(b, &win) != nil {
		return strings.Contains(string(b), screensaverClass)
	}
	return win.Class == screensaverClass
}
