import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

// Opens the koi pond as the screensaver.
//
// Omarchy's idle service stays in charge of everything else. It still decides
// when the machine is idle, honours stay-awake and video inhibitors, locks on
// time, and closes the screensaver when the lock screen comes up. This service
// only switches Omarchy's own screensaver off while the plugin is installed,
// and opens the pond a second after Omarchy would have opened its one. The pond
// window carries Omarchy's screensaver class, so the idle service adopts it as
// its own: dismissing the pond counts as activity, and the lock still follows.
Item {
  id: root

  readonly property string home: Quickshell.env("HOME")
  readonly property string pluginDir: decodeURIComponent(String(Qt.resolvedUrl(".")).replace(/^file:\/\//, "").replace(/\/$/, ""))
  readonly property string launcher: pluginDir + "/bin/koi-pond-screensaver"
  readonly property string stateDir: (Quickshell.env("XDG_STATE_HOME") || (home + "/.local/state")) + "/koi-pond"

  // Omarchy's own "screensaver off" switch, and our note that we were the
  // ones who flipped it, so removing the plugin only undoes what it did.
  readonly property string omarchyOffFlag: home + "/.local/state/omarchy/toggles/screensaver-off"
  readonly property string ownsFlagMark: stateDir + "/owns-screensaver-off"

  // Whether the pond comes up by itself when idle; off leaves it to be opened
  // by hand. Stored as the absence of `idle-off`.
  property bool onIdle: true
  property bool loaded: false

  // The same timeout as Omarchy's screensaver, read from its shell.json.
  property int screensaverSeconds: 150

  // Omarchy's stay-awake switch. The idle timer pauses while it is on, as
  // Omarchy's does, and starts over when it goes off.
  readonly property string indicatorsDir: home + "/.local/state/omarchy/indicators"
  property bool stayAwake: false

  function setOnIdle(value) {
    root.onIdle = !!value
    run(["bash", "-c", root.onIdle ? 'rm -f "$1/idle-off"' : 'mkdir -p "$1" && touch "$1/idle-off"', "bash", root.stateDir])
    if (root.onIdle) claimOmarchyScreensaver()
    else releaseOmarchyScreensaver()
    return root.onIdle ? "on" : "off"
  }

  function toggle() { return setOnIdle(!root.onIdle) }

  // Switch Omarchy's own screensaver off, unless the user already had.
  function claimOmarchyScreensaver() {
    run(["bash", "-c", '[[ -f "$1" ]] || { mkdir -p "$(dirname "$1")" "$(dirname "$2")" && touch "$1" "$2"; }', "bash", root.omarchyOffFlag, root.ownsFlagMark])
  }

  // Give it back, if it was us who switched it off.
  function releaseOmarchyScreensaver() {
    run(["bash", "-c", '[[ -f "$2" ]] && rm -f "$1" "$2"; true', "bash", root.omarchyOffFlag, root.ownsFlagMark])
  }

  // Open the pond now; from idle, not while staying awake. Never over the
  // lock screen.
  function start(fromIdle) {
    if (fromIdle && root.stayAwake) return "staying awake"
    var script = '[[ $(omarchy-shell lock isLocked 2>/dev/null) == "true" ]] && exit 0\n'
      + 'exec "$1"'
    Quickshell.execDetached(["bash", "-lc", script, "bash", root.launcher])
    return "started"
  }

  function run(argv) { Quickshell.execDetached(argv) }

  function readIdleConfig(text) {
    try {
      var n = Number(JSON.parse(text || "{}").idle.screensaver)
      root.screensaverSeconds = isFinite(n) && n >= 0 ? Math.floor(n) : 150
    } catch (e) {
      root.screensaverSeconds = 150
    }
  }

  function statusJson() {
    return JSON.stringify({
      onIdle: root.onIdle,
      stayAwake: root.stayAwake,
      idle: idleMonitor.isIdle,
      timeout: idleMonitor.timeout,
      launcher: root.launcher
    })
  }

  IdleMonitor {
    id: idleMonitor
    enabled: root.loaded && root.onIdle && !root.stayAwake
    // One second after Omarchy's, so its idle cycle has begun and is watching
    // for a screensaver window when the pond's appears.
    timeout: root.screensaverSeconds + 1
    respectInhibitors: true
    onIsIdleChanged: if (isIdle) root.start(true)
  }

  FileView {
    path: root.home + "/.config/omarchy/shell.json"
    watchChanges: true
    printErrors: false
    onFileChanged: reload()
    onLoaded: root.readIdleConfig(text())
    onLoadFailed: root.readIdleConfig("")
  }

  // Watch the directory, since the stay-awake file comes and goes.
  FileView {
    path: root.indicatorsDir
    watchChanges: true
    printErrors: false
    onFileChanged: stayAwakeProbe.running = true
  }

  Process {
    id: stayAwakeProbe
    command: ["bash", "-c", '[[ -f "$1/stay-awake" ]] && echo yes || echo no', "bash", root.indicatorsDir]
    stdout: SplitParser {
      onRead: function(line) { root.stayAwake = String(line).trim() === "yes" }
    }
  }

  FileView {
    path: root.stateDir + "/idle-off"
    printErrors: false
    onLoaded: { root.onIdle = false; root.loaded = true; root.releaseOmarchyScreensaver() }
    onLoadFailed: { root.onIdle = true; root.loaded = true; root.claimOmarchyScreensaver() }
  }

  // Unloading the plugin (removing or disabling it, or the shell exiting)
  // hands the screensaver back to Omarchy; loading it again takes it back.
  Component.onCompleted: stayAwakeProbe.running = true

  Component.onDestruction: if (root.onIdle) releaseOmarchyScreensaver()

  IpcHandler {
    target: "koi-pond"

    function start(): string { return root.start(false) }
    function enable(): string { return root.setOnIdle(true) }
    function disable(): string { return root.setOnIdle(false) }
    function toggle(): string { return root.toggle() }
    function status(): string { return root.statusJson() }
  }
}
