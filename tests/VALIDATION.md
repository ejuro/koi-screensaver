# Release validation — 2026-10-05

Environment: Omarchy 4.0.4-1, Quickshell 0.3.1-1, Hyprland 0.56.2-2,
qt6-declarative 6.11.2-1, Ghostty 1.3.1-2, Go 1.27.1, Linux x86_64.
One 3840×2160 monitor at 1.25 scale. The enabled plugin directory was a
symlink to this working tree.

## Checks

- `omarchy plugin validate .`: exit 0.
- `qmllint Service.qml Panel.qml`: exit 0, no output. **This executable is
  supplied by Qt 5.15.19 on the test machine**, so it is not sufficient as
  the only QML check.
- Qt 6 check, with the shell's `qs` imports made available:

  ```sh
  lint_imports=$(mktemp -d)
  ln -s /usr/share/omarchy/shell "$lint_imports/qs"
  /usr/lib/qt6/bin/qmllint -I "$lint_imports" Service.qml Panel.qml
  rm -rf -- "$lint_imports"
  ```

  Exit 0, **33 warnings**, not warning-free. No missing `qs` imports remain.
  The warnings are three unresolved `QProcess::ExitStatus` signal metadata
  warnings from Quickshell, missing-property warnings for host objects typed
  as `QObject`, and unqualified references to outer component IDs. No lint
  categories were suppressed. The live check below is additional evidence,
  not a substitute for documenting these static-analysis limitations.
- `GOCACHE=/tmp/koi-final-go-cache GOPROXY=off ./build.sh`: exit 0; built both
  Linux architectures using the actual release build script.
- In `src/`, `go test ./...` and `go vet ./...`, with the same cache/proxy
  environment: both exit 0. Includes ANSI foreground-state regression tests.
- `bash tests/launcher.sh`: all ownership and launcher tests pass, including
  stale release, user-created flag preservation, twenty simultaneous starts,
  early/late dismissal, unknown lock state and launch cancellation.
- `bash -n bin/koi-screensaver build.sh tests/launcher.sh`: exit 0.
- ShellCheck 0.10.0 on those three scripts: exit 0, no findings.
- `git diff --check`: exit 0.

## Live smoke check

`omarchy restart shell` succeeded. After startup settled,
`omarchy-shell koi-screensaver status` reported `onIdle=true`,
`holdingOmarchyOff=true`, `launching=false`, `problem=""`, scene `pond`,
motion `smooth`, timeout 151 seconds. Existing stay-awake state remained on.

Opened the drawer through its `io.github.ejuro.koi-screensaver` IPC target
and visually inspected its switch, scene/motion choices and Preview row.
Closed it and started a manual preview through `koi-screensaver start`.
The rebuilt renderer appeared fullscreen in Ghostty with the Omarchy
wordmark, water and lily pads. The active window had the expected
`org.omarchy.screensaver` class. It subsequently dismissed on mouse input,
as recorded in the exit log; status remained healthy afterward. The recent
shell log contained no Koi QML load/runtime error; unrelated portal/MPRIS
warnings were present.

An initial scripted check accidentally put the literal screensaver class
inside the parent shell's command line, and the launcher's broad `pgrep -f`
guard treated that inspection command as an existing saver. Separating the
preview launch from that query allowed it to run. This remains a limitation
of command-line matching, not a missing binary or a QML load failure.

No live multi-monitor, adverse lock-timing, suspend/resume, other-terminal
or aarch64 execution claim is made. Crash cleanup is documented as best
effort. No publication or push was performed.

## Rebuilt binary SHA-256

```text
ba4da3590a03272ed942da837ef2b4f81bc8d533156113bd1ae426a9cb2e792f  bin/koi-screensaver-aarch64
4d5cd6b57f5bdf644e0dab3a6047ef295d3018f6ed008d073098a9633c36ea78  bin/koi-screensaver-x86_64
```
