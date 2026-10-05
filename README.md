# Koi Screensaver

Koi as your Omarchy screensaver. It opens with the Omarchy wordmark painted on to the water in one brush stroke, until the letters break apart and their pieces stream over and build a koi, which grows out from wherever they land and swims away, either into a quiet pond or into a slow chase round a lily pad.

Everything is coloured from your current Omarchy theme, so the koi match your desktop: ink koi on rice paper under a light theme, pale koi in dark water under a dark one.

![Koi drifting between lily pads under the Sumi Night theme](preview.png)

## Scenes

Pick one in the drawer; both open with the wordmark.

- **Koi chase**: two koi, red and ink, circling a single lily pad in still water, always the same distance apart. Now and then one flicks its tail and leaves a little swirl in the water, and faint ripples spread round the pad.
- **Pond** (above): five koi drift under still lily pads and water lilies, past a few floating petals, casting soft shadows on the pond floor and on each other.

## Install

```sh
omarchy plugin add https://github.com/ejuro/koi-screensaver.git --enable
```

Enabling the plugin turns on **Use as screensaver** by default and takes over Omarchy's visual screensaver when the launcher's prerequisites are available. Turn that switch off to use Koi only for manual previews.

**Koi is a visual screensaver only; locking and authentication belong to Omarchy.** Installing Koi does not configure a lock timeout.

## Use

- **Click the fish** in the bar for its drawer:
  - **Use as screensaver**: whether the koi appear by themselves when you're idle, instead of Omarchy's screensaver.
  - **Scene**: Koi chase or Pond.
  - **Motion**: *Balanced*, as light as Omarchy's own screensaver, or *Smooth*, more frames a second for more CPU.
  - **Preview**: see them now. Any key or mouse movement closes it.
- **Right-click the fish** to flip *Use as screensaver* without opening the drawer. The fish is faded while it's off.
- In the drawer, the arrow keys move between the rows, Enter picks, Space flips the switch and `p` previews.
- From anywhere: `omarchy-shell koi-screensaver start`, or `enable` / `disable` / `toggle` / `status`, `omarchy-shell koi-screensaver scene chase` (or `pond`), and `omarchy-shell koi-screensaver motion smooth` (or `balanced`).

To open it from **Super + Escape → System → Screensaver** as well, add this line to `~/.config/omarchy/extensions/omarchy-menu.jsonc`, inside the braces. The icon and label have to be repeated, or the menu row loses them:

```jsonc
"system.screensaver": {"icon":"󱄄","label":"Screensaver","action":"omarchy-shell koi-screensaver start"},
```

## Configure

Everything in the drawer is also in Omarchy's own settings for the widget, and is kept, as Omarchy keeps every plugin's settings, inline on the plugin's entry in `~/.config/omarchy/shell.json`:

```json
{ "id": "io.github.ejuro.koi-screensaver", "onIdle": true, "scene": "chase", "motion": "balanced" }
```

The idle timeout is Omarchy's own (`idle.screensaver`). A few environment variables override the rest, mostly for trying things out: `KOI_SCREENSAVER_SCENE`, `KOI_SCREENSAVER_MOTION`, `KOI_SCREENSAVER_FPS`, `KOI_SCREENSAVER_FONT_SIZE` (see Requirements) and `KOI_SCREENSAVER_COLORS` (a `colors.toml` to use instead of the current theme's).

## How it fits in

Koi uses Omarchy's `idle.screensaver` timeout and an inhibitor-aware idle monitor. It watches Omarchy's stay-awake indicator and opens one second after the stock screensaver timeout. Omarchy recognises the screensaver window class and remains responsible for the lock timer and session lock. The launcher checks lock state before each monitor and refuses to launch when that state is unknown.

While **Use as screensaver** is on, Koi records ownership of Omarchy's `screensaver-off` flag. Normal disable/removal releases the flag only if the recorded instance and file identity still match; a flag the user created is preserved. Turning Omarchy's own screensaver back on while Koi owns the flag makes Koi step aside. The drawer and `omarchy-shell koi-screensaver status` report prerequisite problems.

Cleanup is best effort: a shell crash, forced kill, or power loss can prevent the release handler from running. A later plugin load can take over its previous ownership record, but removing it while the shell is down does not execute cleanup. See recovery below. Disabling Koi stops a pending launcher; an already-open preview may still need dismissal. Slow terminal failures and unusual lock/monitor timing still need broader testing; do not treat a successful preview as proof of every idle-to-lock transition.

## What it runs and touches

The plugin runs unsandboxed as your user. Its runtime code does not make network requests, collect telemetry, or invoke sudo/pkexec. Installation and updates use Git and may access the network; building can download the Go toolchain or dependencies when they are not already available.

- **Settings:** its own inline entry in `~/.config/omarchy/shell.json`, written through the host's scoped settings API. The plugin also reads the idle timeout from this file.
- **Plugin files:** normally `~/.config/omarchy/plugins/io.github.ejuro.koi-screensaver/`. There is no package installer or privileged install hook.
- **Ownership records:** `${XDG_STATE_HOME:-$HOME/.local/state}/koi-screensaver/`, including `owns-screensaver-off` and a bounded `retired-tokens` list.
- **Omarchy state:** `~/.local/state/omarchy/toggles/screensaver-off` is the stock switch Koi temporarily owns. Koi reads `~/.local/state/omarchy/indicators/stay-awake`. These host paths follow Omarchy rather than `XDG_STATE_HOME`.
- **Runtime locks:** `koi-screensaver.state.lock` and `koi-screensaver.launch.lock` under `$XDG_RUNTIME_DIR` (the launcher currently falls back to `/tmp` if unset). A normal Omarchy desktop provides a private runtime directory. The lock files may remain after use; they contain no settings.
- **Log:** `${XDG_STATE_HOME:-$HOME/.local/state}/koi-screensaver.log`, or the path in `KOI_SCREENSAVER_LOG`. It records exit reasons and whether input was a key or mouse event, never the keys typed. It starts afresh on a subsequent write after exceeding about 64 KiB.
- **Read-only inputs:** the current theme's `colors.toml` (or `KOI_SCREENSAVER_COLORS`), the user's branding text and Omarchy's fallback logo. The `--png` development option writes to the explicitly supplied output path.

The launcher uses Bash, coreutils, util-linux (`flock`), procps (`pgrep`/`pkill`), `awk`, `grep`, `jq`, `socat`, `xdg-terminal-exec`, `hyprctl`, and Omarchy's shell/monitor/notification commands. These are available on the tested desktop. The renderer temporarily hides the compositor cursor; cleanup restores visibility and closes windows using Omarchy's screensaver command-line class convention. This process match can also close the stock screensaver. A forced kill can skip cursor restoration.

## Performance

It only sends the parts of the screen that changed. Without an explicit FPS override, the launcher chooses the frame rate as a whole fraction of your monitor's refresh rate, so every frame lasts the same time and the koi move evenly. *Balanced* takes as many frames as fit under 18 a second (17.1 at 120 Hz, 18 at 144 Hz, 15 at 60 Hz); *Smooth* takes about 20 (20 at 60 and 120 Hz).

Measured on a 4K, 120 Hz screen with Ghostty, terminal included, against Omarchy's own screensaver on the same machine:

| | CPU (one core = 100%) |
|---|---|
| Omarchy's screensaver | 58–65%, depending on the effect it plays |
| Koi chase, Balanced | 65% |
| Pond, Balanced | 67% |
| Either scene, Smooth | 70% |

On that machine, both scenes on Balanced cost about the same as Omarchy's screensaver. These are observations, not a CPU or battery-use guarantee on other systems. A larger font (see below) or a lower frame rate (`KOI_SCREENSAVER_FPS`) makes them cheaper still. It runs only while it's on screen.

## Requirements

Requires Omarchy's Quickshell-based shell and a supported terminal. Live smoke-tested on 2026-10-05 with Omarchy **4.0.4-1**, Quickshell **0.3.1-1**, Hyprland **0.56.2-2**, Qt declarative **6.11.2-1**, and Ghostty **1.3.1-2**, on Linux x86_64 with one 3840×2160 monitor at 1.25 scale. Checks covered shell restart, service status, the drawer and a manual preview.

Alacritty, foot and kitty have launcher support but were not live-tested in this release check. Ready-built Linux x86_64 and aarch64 binaries are included; aarch64 was cross-built, not runtime-tested. These are tested versions, not an established minimum-version guarantee. Multi-monitor and adverse idle/lock timing still need desktop coverage.

The koi are drawn with Unicode sextant characters in a terminal at font size 4.5, which is much finer than Omarchy's screensaver uses. To change that, set `KOI_SCREENSAVER_FONT_SIZE` in your environment (3–30; invalid values fall back to 4.5).

## Building

The screensaver itself is a small Go program in [`src/`](src), with no dependencies beyond `golang.org/x/sys`. `./build.sh` rebuilds both binaries in `bin/`, and the build is reproducible: with Go 1.27.1 it gives the shipped binaries byte for byte, so you can check them with `./build.sh && git diff --stat bin/`. To try it in any terminal, run `bin/koi-screensaver-x86_64`; any key quits. Add `-scene chase` for the koi chase, and `-intro=false` to skip the wordmark. `go test ./...` in `src/` checks both drawing consistency and terminal color state. Run `bash tests/launcher.sh` for isolated launcher/ownership tests. Third-party license texts for Go and x/sys are included in [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).

## Update and remove

```sh
omarchy plugin update io.github.ejuro.koi-screensaver
omarchy plugin remove io.github.ejuro.koi-screensaver
```

Normal removal takes the plugin entry out of `shell.json` and asks the loaded service to release its own stock-off flag. This requires a running shell and successful cleanup; it is not guaranteed after a crash or forced kill.

If you added the optional menu override, remove the `system.screensaver` entry whose action is `omarchy-shell koi-screensaver start` from `~/.config/omarchy/extensions/omarchy-menu.jsonc`, or restore your previous entry. Keep other custom entries and valid JSONC punctuation. Plugin removal does not edit that menu file.

### Recovery after a crash

First stop using Koi (disable/remove it, or keep the shell stopped). Inspect `~/.local/state/omarchy/toggles/screensaver-off`. If the stock screensaver should be enabled and the flag is a leftover from Koi, remove it:

```sh
rm -f ~/.local/state/omarchy/toggles/screensaver-off
```

Leave it in place if you deliberately disabled the stock screensaver. Do not remove another active plugin's state. If a forced kill left the cursor hidden, restore it using the compositor's current API:

```sh
hyprctl eval 'hl.config({ cursor = { invisible = false } })' || hyprctl keyword cursor:invisible false
```

After disabling/removing Koi and recovering the intended stock setting, its ownership records and log can be deleted:

```sh
rm -rf -- "${XDG_STATE_HOME:-$HOME/.local/state}/koi-screensaver"
rm -f -- "${XDG_STATE_HOME:-$HOME/.local/state}/koi-screensaver.log"
```

If `KOI_SCREENSAVER_LOG` was set, its custom log remains at that path. Runtime lock files need no manual cleanup and normally disappear with the login session; do not unlink locks while a launcher could still hold them.

See [SECURITY.md](SECURITY.md) for the trust model, limitations and reporting guidance.

MIT License. Bundled third-party components retain their own licenses; see [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
