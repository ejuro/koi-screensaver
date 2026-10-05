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

  // This instance's name for itself when it switches Omarchy's screensaver
  // off, so a late release from an earlier instance (after a shell reload)
  // can't undo this one's claim. The launcher keeps the bookkeeping.
  readonly property string token: "s" + Date.now().toString(36) + Math.floor(Math.random() * 1e9).toString(36)
  readonly property string togglesDir: home + "/.local/state/omarchy/toggles"

  // Why the koi can't run here ("" when they can). While there is a reason,
  // Omarchy's own screensaver stays on.
  property string problem: ""
  property bool launchFailed: false
  // Whether the plugin is the one holding Omarchy's screensaver off.
  property bool holding: false
  readonly property bool launching: launchProc.running

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
    var previous = root.onIdle
    if (value && root.launchFailed) {
      root.launchFailed = false
      root.problem = ""
    }
    saveSettings({ onIdle: !!value })
    if (root.onIdle === previous) sync()
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
    if (root.onIdle) root.launchFailed = false
    if (!root.onIdle) launchProc.running = false
    sync()
  }

  // Bring Omarchy's switch in line with ours: the koi take over only when
  // they can run here, and step aside if you switch Omarchy's back on.
  function sync() {
    if (!root.loaded) return
    stateCall(root.onIdle && !root.launchFailed ? "claim" : "release")
    if (!root.onIdle) checkProc.running = true
  }

  // One call at a time, in order; only the latest wish waits its turn.
  property var queue: []
  function stateCall(action) {
    root.queue = [action]
    pump()
  }
  function pump() {
    if (stateProc.running || root.queue.length === 0) return
    stateProc.action = root.queue[0]
    root.queue = []
    stateProc.command = launcherCommand([stateProc.action, root.token])
    stateProc.running = true
  }
  function stateResult(action, out, code) {
    var lines = String(out || "").trim().split("\n")
    var line = lines[lines.length - 1]
    if (code !== 0) { retryTimer.restart(); return }
    if (action === "release") {
      if (line === "released") root.holding = false
      else retryTimer.restart()
      return
    }
    if (line === "owned") { root.holding = true; if (!root.launchFailed) root.problem = "" }
    else if (line === "user-off") { root.holding = false; if (!root.launchFailed) root.problem = "" }
    else if (line === "user-on") {
      // You switched Omarchy's screensaver back on yourself.
      root.holding = false
      if (root.onIdle) saveSettings({ onIdle: false })
    } else if (line.indexOf("problem: ") === 0) {
      root.holding = false
      root.problem = line.slice(9)
    } else if (line !== "retired") retryTimer.restart()
  }

  function launcherCommand(args) {
    return ["bash", "-lc", 'exec "$@"', "bash", root.launcher].concat(args)
  }

  // Open the koi now; from idle, not while staying awake. The launcher
  // checks the lock screen before every window, and is stopped if the koi
  // are switched off or the plugin unloads while it is still opening them.
  function start(fromIdle) {
    if (fromIdle && root.stayAwake) return "staying awake"
    if (launchProc.running) return "already starting"
    launchProc.running = true
    return root.problem ? "can't run: " + root.problem : "starting"
  }

  // Read the idle timeout and this plugin's own entry from shell.json.
  // A file that can't be read or parsed (say, halfway through an edit)
  // keeps the last good settings, and before the first good read nothing is
  // taken over.
  function readConfig(text) {
    var config
    try { config = JSON.parse(text) } catch (e) { return }
    if (!config || typeof config !== "object" || Array.isArray(config)) return
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
      sync()
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
      holdingOmarchyOff: root.holding,
      launching: root.launching,
      problem: root.problem,
      launcher: root.launcher
    })
  }

  IdleMonitor {
    id: idleMonitor
    enabled: root.loaded && root.onIdle && !root.stayAwake && !root.launchFailed
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
  }

  // Omarchy's toggles: if you switch its screensaver back on while the koi
  // hold it off, they step aside.
  FileView {
    id: togglesWatcher
    path: root.togglesDir
    watchChanges: true
    printErrors: false
    onFileChanged: if (root.onIdle) root.sync()
  }

  Process {
    id: stateProc
    property string action: ""
    stdout: StdioCollector { id: stateOutput }
    onExited: function(code) {
      root.stateResult(stateProc.action, stateOutput.text, code)
      togglesWatcher.reload()
      Qt.callLater(root.pump)
    }
  }

  Process {
    id: checkProc
    command: root.launcherCommand(["check"])
    stdout: StdioCollector {
      onStreamFinished: {
        var lines = String(text || "").trim().split("\n")
        var line = lines[lines.length - 1]
        if (!root.launchFailed) root.problem = line === "ok" || line === "" ? "" : line
      }
    }
  }

  // The launcher, while it opens the koi. It says itself if they can't run.
  Process {
    id: launchProc
    command: root.launcherCommand([])
    stdout: StdioCollector { id: launchOutput }
    onExited: function(code) {
      if (code !== 0 && code !== 143) {
        root.launchFailed = true
        root.problem = "Couldn't open the screensaver. Preview to retry."
        root.sync()
      } else if (code === 0 && String(launchOutput.text).trim() === "opened" && root.launchFailed) {
        root.launchFailed = false
        root.problem = ""
        root.sync()
      }
    }
  }

  // The switch's lock was busy: try again shortly.
  Timer {
    id: retryTimer
    interval: 3000
    onTriggered: root.sync()
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

  Component.onDestruction: {
    launchProc.running = false
    Quickshell.execDetached(root.launcherCommand(["retire", root.token]))
  }

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
