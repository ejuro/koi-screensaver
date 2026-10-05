// koi-screensaver: a slow, quiet koi pond for the terminal. Koi drift under lily pads
// and water lilies, and petals float by. Coloured from the current Omarchy
// theme.
package main

import (
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
	pngOut := flag.String("png", "", "render a preview to this PNG instead of running")
	seconds := flag.Float64("seconds", 20, "preview: seconds to simulate first")
	size := flag.String("size", "279x72", "preview: terminal size in cells")
	cellPx := flag.String("cell", "11x24", "preview: cell size in pixels")
	intro := flag.Bool("intro", true, "open with the Omarchy wordmark turning into the koi")
	scene := flag.String("scene", scenePond, "what to show: pond (koi drifting under lily pads) or chase (two koi circling a lily pad)")
	flag.Parse()
	if *scene != scenePond && *scene != sceneChase {
		*scene = scenePond
	}

	if *pngOut != "" {
		var cols, rows, cw, ch int
		fmt.Sscanf(*size, "%dx%d", &cols, &rows)
		fmt.Sscanf(*cellPx, "%dx%d", &cw, &ch)
		p := newPond(cols, rows, aspectOf(cw, ch), newPalette(themeColors()), *seed, *scene)
		p.fade = 0
		if *intro {
			p.startIntro()
		}
		for range int(*seconds * 30) {
			p.fade = math.Min(1, p.fade+1.0/30/fadeSeconds)
			p.step(1.0 / 30)
		}
		p.draw()
		cells := make([]cell, cols*rows)
		p.cells(cells)
		if err := writePNG(*pngOut, cells, cols, rows, cw, ch); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(*screensaver, *intro, *scene, *fps, *seed); err != nil {
		fmt.Fprintln(os.Stderr, "koi-screensaver:", err)
		os.Exit(1)
	}
}

func aspectOf(cw, ch int) float64 {
	if cw <= 0 || ch <= 0 {
		return 1.45
	}
	return (float64(ch) / 3) / (float64(cw) / 2)
}

var screensaverMode bool

func run(screensaver, intro bool, scene string, fps float64, seed uint64) error {
	fps = clamp(fps, 1, 60)
	screensaverMode = screensaver
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

	out := os.Stdout
	setup := "\x1b[?1049h\x1b[?25l\x1b[2J"
	teardown := "\x1b[0m\x1b[2J\x1b[?25h\x1b[?1049l"
	if screensaver {
		// Report every mouse movement, so a nudge wakes the screen.
		setup += "\x1b[?1003h\x1b[?1006h"
		teardown = "\x1b[?1003l\x1b[?1006l" + teardown
		hypr("eval", "hl.config({ cursor = { invisible = true } })")
	}
	out.WriteString(setup)
	defer func() {
		out.WriteString(teardown)
		unix.IoctlSetTermios(fd, unix.TCSETS, old)
		if screensaver {
			hypr("eval", "hl.config({ cursor = { invisible = false } })")
			// Closing one screensaver closes them all, as Omarchy's does.
			exec.Command("pkill", "-f", "["+screensaverClass[:1]+"]"+screensaverClass[1:]).Run()
		}
	}()

	// Why it closed, for the log (KOI_SCREENSAVER_LOG, or ~/.local/state/koi-screensaver.log
	// as the screensaver).
	quit := make(chan string, 1)
	stop := func(why string) {
		select {
		case quit <- why:
		default:
		}
	}
	sig := make(chan os.Signal, 4)
	signal.Notify(sig, unix.SIGINT, unix.SIGTERM, unix.SIGHUP, unix.SIGQUIT, unix.SIGWINCH)

	start := time.Now()
	go func() {
		buf := make([]byte, 256)
		for {
			n, err := os.Stdin.Read(buf)
			if err != nil {
				stop("stdin closed")
				return
			}
			// Ignore what arrives while the window is still settling.
			if n > 0 && time.Since(start) > 700*time.Millisecond {
				stop(fmt.Sprintf("input %q", buf[:n]))
				return
			}
		}
	}()
	if screensaver {
		go func() {
			for range time.Tick(time.Second) {
				if time.Since(start) > 2*time.Second && !screensaverFocused() {
					stop("lost focus")
					return
				}
			}
		}()
	}

	// Terminals open at 80x24 and resize once the compositor has placed
	// the window; wait a moment for that before laying out the pond.
	cols, rows, cw, ch := winsize(fd)
	for wait := time.Now(); cols == 80 && rows == 24 && time.Since(wait) < 2*time.Second; {
		time.Sleep(20 * time.Millisecond)
		cols, rows, cw, ch = winsize(fd)
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
		case why := <-quit:
			logf("quit: %s", why)
			return nil
		case s := <-sig:
			if s != unix.SIGWINCH {
				logf("quit: %v", s)
				return nil
			}
			c, r, w, h := winsize(fd)
			if c != cols || r != rows {
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
			out.Write(buf)
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

func hypr(args ...string) {
	exec.Command("hyprctl", args...).Run()
}

func screensaverFocused() bool {
	b, err := exec.Command("hyprctl", "activewindow", "-j").Output()
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

func logf(format string, args ...any) {
	path := os.Getenv("KOI_SCREENSAVER_LOG")
	if path == "" && screensaverMode {
		home, _ := os.UserHomeDir()
		path = home + "/.local/state/koi-screensaver.log"
	}
	if path != "" {
		if f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
			fmt.Fprintf(f, time.Now().Format("15:04:05 ")+format+"\n", args...)
			f.Close()
		}
	}
}
