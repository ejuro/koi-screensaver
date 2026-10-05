# Koi Pond

A slow, quiet koi pond as your Omarchy screensaver. It opens with the Omarchy wordmark floating on the water, until each letter breaks apart and gathers itself into a koi that swims away. Koi drift under lily pads and water lilies, cast soft shadows on the pond floor and on each other, and leave faint wakes when they swim near the surface. Now and then a single raindrop falls, and a koi may come over to see whether it was food.

Everything is coloured from your current Omarchy theme, so the pond matches your desktop: ink koi on rice paper under a light theme, pale koi in dark water under a dark one.

![Koi drifting between lily pads under the Sumi Night theme](preview.png)

## Scenes

Pick one in the panel; both open with the wordmark.

- **Pond**: the koi pond above, with several koi, lily pads, raindrops and petals.
- **Koi chase**: two koi, red and ink, circling a single lily pad in still water, always the same distance apart. Now and then one flicks its tail and leaves a little swirl in the water. It's quieter, and lighter on the computer.

## Install

```sh
omarchy plugin add https://github.com/ejuro/koi-pond.git --enable
```

That's all. From then on the pond opens instead of Omarchy's screensaver whenever you've been idle long enough.

## Use

- **Click the fish** in the bar for its panel: a switch for whether the pond opens by itself when you're idle, the scene, and *Open the pond now*. Any key or mouse movement closes the pond.
- **Right-click the fish** to flip that switch without opening the panel. The fish is faded while the pond only opens by hand.
- In the panel, the arrow keys move between the rows (left and right between the scenes), Enter picks, and `o` opens the pond.
- From anywhere: `omarchy-shell koi-pond start`, or `enable` / `disable` / `toggle` / `status`, and `omarchy-shell koi-pond scene chase` (or `pond`).

To open it from **Super + Escape → System → Screensaver** as well, add this line to `~/.config/omarchy/extensions/omarchy-menu.jsonc`, inside the braces. The icon and label have to be repeated, or the menu row loses them:

```jsonc
"system.screensaver": {"icon":"󱄄","label":"Screensaver","action":"omarchy-shell koi-pond start"},
```

## How it fits in

Omarchy stays in charge of idling. Your idle timeout (`idle.screensaver` in `shell.json`), stay-awake, video playback keeping the screen on, and locking all keep working as before. The plugin only swaps what appears:

- While the pond opens by itself, Omarchy's own screensaver is switched off, using Omarchy's own setting for that. Disabling or removing the plugin switches it back on. If you had already switched Omarchy's screensaver off yourself, it stays off.
- The pond opens one second after Omarchy's screensaver would have, in a window Omarchy recognises as its screensaver. So the lock screen still follows on time, and closing the pond counts as you coming back.

## Performance

It only sends the parts of the screen that changed, at up to 18 frames per second: as many as fit while each frame lasts a whole number of your monitor's refreshes, so the koi move evenly (17.1 at 120 Hz, 18 at 144 Hz, 15 at 60 Hz). Measured on a 4K, 120 Hz screen with Ghostty, terminal included, against Omarchy's own screensaver on the same machine:

| | CPU (one core = 100%) |
|---|---|
| Omarchy's screensaver | 65% |
| Koi chase | 64% |
| Pond | 124% |

The koi chase costs the same as Omarchy's screensaver. The pond is heavier, because something moves almost everywhere on screen at once. A larger font (see below) or a lower frame rate (`KOI_POND_FPS`) makes either cheaper. It runs only while it's on screen.

## Requirements

Omarchy with its shell, and Ghostty, Alacritty or foot as your terminal. kitty is supported too but hasn't been tested yet. Ready-built programs for x86_64 and aarch64 are included, so nothing has to be compiled.

The pond is drawn with Unicode sextant characters in a terminal at font size 4.5, which is much finer than Omarchy's screensaver uses. To change that, set `KOI_POND_FONT_SIZE` in your environment.

## Building

The pond itself is a small Go program in [`src/`](src). `./build.sh` rebuilds both binaries in `bin/`. To try it in any terminal, run `bin/koi-pond-x86_64`; any key quits. Add `-scene chase` for the koi chase, and `-intro=false` to skip the wordmark.

## Update and remove

```sh
omarchy plugin update io.github.ejuro.koi-pond
omarchy plugin remove io.github.ejuro.koi-pond
```

The plugin keeps a few small notes (whether it opens when idle, the scene) in `${XDG_STATE_HOME:-~/.local/state}/koi-pond/`, and screensaver mode writes why it last closed to `~/.local/state/koi-pond.log`.

MIT License.
