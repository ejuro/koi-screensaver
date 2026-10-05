import QtQuick
import Quickshell
import Quickshell.Io
import Quickshell.Wayland

// Opens the koi as the screensaver.
//
// Omarchy's idle service stays in charge of everything else. It still decides
// when the machine is idle, honours stay-awake and video inhibitors, locks on
// time, and closes the screensaver when the lock screen comes up. This service
// only switches Omarchy's own screensaver off while the koi are in use, and
// opens them a second after Omarchy would have opened its one. Their window
// carries Omarchy's screensaver class, so the idle service adopts it as its
// own: dismissing it counts as activity, and the lock still follows.
Item {
  id: root

  readonly property string home: Quickshell.env("HOME")
  readonly property string pluginDir: decodeURIComponent(String(Qt.resolvedUrl(".")).replace(/^file:\/\//, "").replace(/\/$/, ""))
  readonly property string launcher: pluginDir + "/bin/koi-screensaver"
  readonly property string stateDir: (Quickshell.env("XDG_STATE_HOME") || (home + "/.local/state")) + "/koi-screensaver"

  // Omarchy's own "screensaver off" switch, and our note that we were the
  // ones who flipped it, so removing the plugin only undoes what it did.
  readonly property string omarchyOffFlag: home + "/.local/state/omarchy/toggles/screensaver-off"
  readonly property string ownsFlagMark: stateDir + "/owns-screensaver-off"

  // The settings live inline on the plugin's own entry in shell.json, as
  // Omarchy's storage rules ask, written through the shell facade the host
  // injects (scoped to this plugin's own entry). The launcher reads them from
  // there too.
  readonly property string pluginId: "io.github.ejuro.koi-screensaver"
  property var shell: null
  property var entry: ({})
  property bool loaded: false

  // Whether the koi come out by themselves when idle; off leaves them to a
  // preview by hand.
  readonly property bool onIdle: root.entry.onIdle !== false

  // What the koi do: "chase", two koi circling a lily pad, or "pond", koi
  // drifting under lily pads.
  readonly property string scene: root.entry.scene === "chase" ? "chase" : "pond"

  // How smoothly they swim: "balanced", as light as Omarchy's own
  // screensaver, or "smooth", more frames for more CPU.
  readonly property string motion: root.entry.motion === "smooth" ? "smooth" : "balanced"

  // The same timeout as Omarchy's screensaver, read from its shell.json.
  property int screensaverSeconds: 150

  // Omarchy's stay-awake switch. The idle timer pauses while it is on, as
  // Omarchy's does, and starts over when it goes off.
  readonly property string indicatorsDir: home + "/.local/state/omarchy/indicators"
  property bool stayAwake: false

  // Merge a change into the entry and write it back to shell.json.
  function saveSettings(change) {
    var next = {}
    for (var k in root.entry) next[k] = root.entry[k]
    for (var c in change) next[c] = change[c]
    root.entry = next
    if (root.shell && typeof root.shell.updateEntryInline === "function")
      root.shell.updateEntryInline(root.pluginId, next)
  }

  function setOnIdle(value) {
    saveSettings({ onIdle: !!value })
    return root.onIdle ? "on" : "off"
  }

  function toggle() { return setOnIdle(!root.onIdle) }

  function setScene(value) {
    saveSettings({ scene: value === "chase" ? "chase" : "pond" })
    return root.scene
  }

  function setMotion(value) {
    saveSettings({ motion: value === "smooth" ? "smooth" : "balanced" })
    return root.motion
  }

  // Follow the switch, whether it was flipped here or in shell.json.
  onOnIdleChanged: if (root.loaded) {
    if (root.onIdle) claimOmarchyScreensaver()
    else releaseOmarchyScreensaver()
  }

  // Switch Omarchy's own screensaver off, unless the user already had.
  function claimOmarchyScreensaver() {
    run(["bash", "-c", '[[ -f "$1" ]] || { mkdir -p "$(dirname "$1")" "$(dirname "$2")" && touch "$1" "$2"; }', "bash", root.omarchyOffFlag, root.ownsFlagMark])
  }

  // Give it back, if it was us who switched it off.
  function releaseOmarchyScreensaver() {
    run(["bash", "-c", '[[ -f "$2" ]] && rm -f "$1" "$2"; true', "bash", root.omarchyOffFlag, root.ownsFlagMark])
  }

  // Open the koi now; from idle, not while staying awake. Never over the
  // lock screen.
  function start(fromIdle) {
    if (fromIdle && root.stayAwake) return "staying awake"
    var script = '[[ $(omarchy-shell lock isLocked 2>/dev/null) == "true" ]] && exit 0\n'
      + 'exec "$1"'
    Quickshell.execDetached(["bash", "-lc", script, "bash", root.launcher])
    return "started"
  }

  function run(argv) { Quickshell.execDetached(argv) }

  // Read the idle timeout and this plugin's own entry from shell.json.
  function readConfig(text) {
    var config = {}
    try { config = JSON.parse(text || "{}") || {} } catch (e) { config = {} }
    var n = Number(config.idle ? config.idle.screensaver : NaN)
    root.screensaverSeconds = isFinite(n) && n >= 0 ? Math.floor(n) : 150

    var found = null
    var layout = config.bar && config.bar.layout ? config.bar.layout : {}
    for (var section in layout) {
      var items = Array.isArray(layout[section]) ? layout[section] : []
      for (var i = 0; i < items.length && !found; i++)
        if (items[i] && items[i].id === root.pluginId) found = items[i]
    }
    var plugins = Array.isArray(config.plugins) ? config.plugins : []
    for (var j = 0; j < plugins.length && !found; j++)
      if (plugins[j] && plugins[j].id === root.pluginId) found = plugins[j]
    var next = {}
    if (found) for (var k in found) if (k !== "id") next[k] = found[k]
    root.entry = next

    if (!root.loaded) {
      root.loaded = true
      if (root.onIdle) claimOmarchyScreensaver()
      else releaseOmarchyScreensaver()
    }
  }

  function statusJson() {
    return JSON.stringify({
      onIdle: root.onIdle,
      scene: root.scene,
      motion: root.motion,
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
    onLoaded: root.readConfig(text())
    onLoadFailed: root.readConfig("")
  }

  // Watch the directory, since the stay-awake file comes and goes.
  FileView {
    id: indicatorsWatcher
    path: root.indicatorsDir
    watchChanges: true
    printErrors: false
    onFileChanged: stayAwakeProbe.running = true
  }

  Process {
    id: stayAwakeProbe
    // Make sure the directory exists, then watch it again: a watch set up
    // before Omarchy first creates it would never fire.
    command: ["bash", "-c", 'mkdir -p "$1"; [[ -f "$1/stay-awake" ]] && echo yes || echo no', "bash", root.indicatorsDir]
    stdout: SplitParser {
      onRead: function(line) { root.stayAwake = String(line).trim() === "yes" }
    }
    onExited: indicatorsWatcher.reload()
  }

  // Unloading the plugin (removing or disabling it, or the shell exiting)
  // hands the screensaver back to Omarchy; loading it again takes it back.
  Component.onCompleted: stayAwakeProbe.running = true

  Component.onDestruction: if (root.onIdle) releaseOmarchyScreensaver()

  IpcHandler {
    target: "koi-screensaver"

    function start(): string { return root.start(false) }
    function enable(): string { return root.setOnIdle(true) }
    function disable(): string { return root.setOnIdle(false) }
    function toggle(): string { return root.toggle() }
    function status(): string { return root.statusJson() }
    function scene(name: string): string { return root.setScene(name) }
    function motion(name: string): string { return root.setMotion(name) }
  }
}
