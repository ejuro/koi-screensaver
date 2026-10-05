# Koi Screensaver

Koi as your Omarchy screensaver. It opens with the Omarchy wordmark floating on the water, until each letter breaks apart and gathers itself into a koi that swims away, either into a quiet pond or into a slow chase round a lily pad.

Everything is coloured from your current Omarchy theme, so the koi match your desktop: ink koi on rice paper under a light theme, pale koi in dark water under a dark one.

![Koi drifting between lily pads under the Sumi Night theme](preview.png)

## Scenes

Pick one in the drawer; both open with the wordmark.

- **Pond** (above): five koi drift under still lily pads and water lilies, past a few floating petals, casting soft shadows on the pond floor and on each other.
- **Koi chase**: two koi, red and ink, circling a single lily pad in still water, always the same distance apart. Now and then one flicks its tail and leaves a little swirl in the water, and faint ripples spread round the pad. It's quieter, and lighter on the computer.

## Install

```sh
omarchy plugin add https://github.com/ejuro/koi-screensaver.git --enable
```

That's all. From then on the koi appear instead of Omarchy's screensaver whenever you've been idle long enough.

## Use

- **Click the fish** in the bar for its drawer:
  - **Use as screensaver**: whether the koi appear by themselves when you're idle, instead of Omarchy's screensaver.
  - **Scene**: Pond or Koi chase.
  - **Preview**: see them now. Any key or mouse movement closes it.
- **Right-click the fish** to flip *Use as screensaver* without opening the drawer. The fish is faded while it's off.
- In the drawer, the arrow keys move between the rows, Enter picks, Space flips the switch and `p` previews.
- From anywhere: `omarchy-shell koi-screensaver start`, or `enable` / `disable` / `toggle` / `status`, and `omarchy-shell koi-screensaver scene chase` (or `pond`).

To open it from **Super + Escape → System → Screensaver** as well, add this line to `~/.config/omarchy/extensions/omarchy-menu.jsonc`, inside the braces. The icon and label have to be repeated, or the menu row loses them:

```jsonc
"system.screensaver": {"icon":"󱄄","label":"Screensaver","action":"omarchy-shell koi-screensaver start"},
```

## How it fits in

Omarchy stays in charge of idling. Your idle timeout (`idle.screensaver` in `shell.json`), stay-awake, video playback keeping the screen on, and locking all keep working as before. The plugin only swaps what appears:

- While *Use as screensaver* is on, Omarchy's own screensaver is switched off, using Omarchy's own setting for that. Disabling or removing the plugin switches it back on. If you had already switched Omarchy's screensaver off yourself, it stays off.
- The koi appear one second after Omarchy's screensaver would have, in a window Omarchy recognises as its screensaver. So the lock screen still follows on time, and closing it counts as you coming back.

## Performance

It only sends the parts of the screen that changed, at up to 18 frames per second: as many as fit while each frame lasts a whole number of your monitor's refreshes, so the koi move evenly (17.1 at 120 Hz, 18 at 144 Hz, 15 at 60 Hz). Measured on a 4K, 120 Hz screen with Ghostty, terminal included, against Omarchy's own screensaver on the same machine:

| | CPU (one core = 100%) |
|---|---|
| Omarchy's screensaver | 58–65%, depending on the effect it plays |
| Koi chase | 65% |
| Pond | 67% |

Both scenes cost about the same as Omarchy's screensaver. A larger font (see below) or a lower frame rate (`KOI_SCREENSAVER_FPS`) makes either cheaper. It runs only while it's on screen.

## Requirements

Omarchy with its shell, and Ghostty, Alacritty or foot as your terminal. kitty is supported too but hasn't been tested yet. Ready-built programs for x86_64 and aarch64 are included, so nothing has to be compiled.

The koi are drawn with Unicode sextant characters in a terminal at font size 4.5, which is much finer than Omarchy's screensaver uses. To change that, set `KOI_SCREENSAVER_FONT_SIZE` in your environment.

## Building

The screensaver itself is a small Go program in [`src/`](src). `./build.sh` rebuilds both binaries in `bin/`. To try it in any terminal, run `bin/koi-screensaver-x86_64`; any key quits. Add `-scene chase` for the koi chase, and `-intro=false` to skip the wordmark.

## Update and remove

```sh
omarchy plugin update io.github.ejuro.koi-screensaver
omarchy plugin remove io.github.ejuro.koi-screensaver
```

The plugin keeps a few small notes (whether it opens when idle, the scene) in `${XDG_STATE_HOME:-~/.local/state}/koi-screensaver/`, and screensaver mode writes why it last closed to `~/.local/state/koi-screensaver.log`.

MIT License.
