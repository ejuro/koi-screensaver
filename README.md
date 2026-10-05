# Koi Screensaver

A theme-coloured koi pond for Omarchy. The Omarchy wordmark breaks into pieces that become swimming koi. Choose a quiet **Pond** or two koi circling a lily pad in **Koi chase**.

![Koi drifting between lily pads](preview.png)

## Install

```sh
omarchy plugin add https://github.com/ejuro/koi-screensaver.git --enable
```

Enabling Koi replaces the stock visual screensaver by default. **Locking and authentication remain Omarchy's responsibility.**

Requires Omarchy's Quickshell shell and Ghostty, Alacritty, foot or kitty. Tested with Omarchy 4.0.4 and Ghostty 1.3.1 on x86_64; other terminals and the included aarch64 build have not been runtime-tested. See [validation details](tests/VALIDATION.md) for exact versions and test coverage.

## Use

Click the fish in the bar to enable or disable idle activation, choose a scene, select **Balanced** or **Smooth** motion, or preview. Right-click toggles idle activation. Any key or mouse movement dismisses the preview.

Koi follows Omarchy's idle timeout and stay-awake setting. Settings live in its entry in `~/.config/omarchy/shell.json`.

```sh
omarchy-shell koi-screensaver start    # Preview
omarchy-shell koi-screensaver status   # Settings and launch problems
```

For the optional System → Screensaver menu shortcut, add this entry inside `~/.config/omarchy/extensions/omarchy-menu.jsonc`:

```jsonc
"system.screensaver": {"icon":"󱄄","label":"Screensaver","action":"omarchy-shell koi-screensaver start"},
```

## Update and remove

```sh
omarchy plugin update io.github.ejuro.koi-screensaver
omarchy plugin remove io.github.ejuro.koi-screensaver
```

Normal removal restores the stock screensaver if Koi disabled it. Remove the optional menu entry yourself. Ownership records and the exit log remain under `${XDG_STATE_HOME:-$HOME/.local/state}` as `koi-screensaver/` and `koi-screensaver.log`; you can delete them after removal.

A crash or forced kill can skip restoration. After disabling/removing Koi, if the stock screensaver should be enabled, remove a leftover `~/.local/state/omarchy/toggles/screensaver-off`. Keep it if you deliberately disabled the stock saver. If the cursor remains hidden:

```sh
hyprctl eval 'hl.config({ cursor = { invisible = false } })' || hyprctl keyword cursor:invisible false
```

## Build

Go 1.27.1; dependency: `golang.org/x/sys`. Runtime helpers are supplied by Omarchy: `hyprctl`, `jq`, `socat`, `xdg-terminal-exec`, `flock` and standard shell utilities.

```sh
./build.sh                 # Rebuild both architectures
(cd src && go test ./...)
bash tests/launcher.sh
```

[MIT License](LICENSE) · [Third-party notices](THIRD_PARTY_NOTICES) · [Security](SECURITY.md)
