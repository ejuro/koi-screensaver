import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Ui

// The fish in the bar and its drop-down: a switch for whether the pond opens
// by itself when idle, and a row to open it now. The service owns the pond;
// this is only a handle on it.
Panel {
  id: root
  moduleName: "io.github.ejuro.koi-pond"
  ipcTarget: "io.github.ejuro.koi-pond"

  readonly property var svc: bar && bar.shell ? bar.shell.serviceFor("io.github.ejuro.koi-pond") : null
  readonly property bool onIdle: svc ? svc.onIdle === true : false
  readonly property bool stayAwake: svc ? svc.stayAwake === true : false
  readonly property int idleSeconds: svc ? svc.screensaverSeconds : 150

  readonly property color foreground: bar ? bar.foreground : Color.foreground
  readonly property color dim: Qt.darker(foreground, 1.55)
  readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

  // Keyboard cursor: the switch in the header, or the open-now row.
  property string focusSection: "header"
  property bool cursorActive: false

  function durationText(s) {
    if (s < 60) return s + " s"
    return Math.floor(s / 60) + " min" + (s % 60 ? " " + (s % 60) + " s" : "")
  }

  // What the switch is doing right now, in a sentence.
  readonly property string statusText: !onIdle
    ? "Off: Omarchy's own screensaver is back. You can still open the pond below."
    : stayAwake
      ? "On, but stay-awake is on, so nothing opens while you're idle."
      : "On: after " + durationText(idleSeconds) + " idle the koi pond opens instead of Omarchy's screensaver."

  function toggleOnIdle() { if (svc) svc.toggle() }

  // Close the panel first, so the pond takes focus from it.
  function openPond() {
    root.close()
    startTimer.restart()
  }

  function activateCursor() {
    if (focusSection === "header") toggleOnIdle()
    else openPond()
  }

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  onOpenedChanged: if (opened) {
    cursorActive = false
    focusSection = "header"
    Qt.callLater(function() { keyCatcher.forceActiveFocus() })
  }

  Timer {
    id: startTimer
    interval: 200
    onTriggered: if (root.svc) root.svc.start(false)
  }

  BarIconButton {
    id: button
    anchors.fill: parent
    bar: root.bar
    text: "󰈺"
    tooltipText: root.onIdle ? "Koi Pond · replacing Omarchy's screensaver" : "Koi Pond · Omarchy's screensaver in use"
    // Like the other icons: plain while the pond is the screensaver, faded
    // when it only opens by hand.
    dimmed: !root.onIdle
    onPressed: function(buttonCode) {
      if (buttonCode === Qt.RightButton) root.toggleOnIdle()
      else root.toggle()
    }
  }

  KeyboardPanel {
    id: panel
    anchorItem: button
    owner: root
    bar: root.bar
    open: root.opened
    focusTarget: keyCatcher
    contentWidth: panel.fittedContentWidth(Style.space(360))
    contentHeight: panel.fittedContentHeight(column.implicitHeight, Style.space(300))

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      onMoveRequested: function(dx, dy) {
        if (!root.cursorActive) { root.cursorActive = true; return }
        if (dy > 0) root.focusSection = "open"
        else if (dy < 0) root.focusSection = "header"
      }
      onActivateRequested: if (root.cursorActive) root.activateCursor()
      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }
      onTextKey: function(t) {
        if (t === "o" || t === "O") root.openPond()
        else if (t === " ") root.toggleOnIdle()
      }

      Column {
        id: column
        width: parent.width
        spacing: Style.space(12)

        Item {
          id: header
          width: parent.width
          implicitHeight: hero.implicitHeight
          // The hero's trailingControl resolves `root` to PanelHero, so it
          // reaches panel state through `header`.
          readonly property bool ringVisible: root.cursorActive && root.focusSection === "header"
          function focusHero() { root.cursorActive = true; root.focusSection = "header" }

          PanelHero {
            id: hero
            width: parent.width
            title: "Koi Pond"
            meta: "Replaces your screensaver"
            foreground: root.foreground
            fontFamily: root.fontFamily
            iconOpacity: root.onIdle ? 1.0 : 0.5
            iconComponent: Component {
              Text {
                text: "󰈺"
                color: root.foreground
                font.family: root.fontFamily
                font.pixelSize: Style.font.display
              }
            }

            trailingControl: Component {
              ToggleSwitch {
                id: idleSwitch
                checked: root.onIdle
                hasCursor: header.ringVisible
                foreground: hero.foreground
                onHovered: function(on) { if (on) header.focusHero() }
                onToggled: root.toggleOnIdle()

                PanelToolTip {
                  visible: idleSwitch.containsMouse
                  text: root.onIdle ? "Give the screensaver back to Omarchy" : "Use the koi pond as your screensaver"
                  fontFamily: hero.fontFamily
                }
              }
            }
          }
        }

        Text {
          textFormat: Text.PlainText
          width: parent.width
          text: root.statusText
          color: root.dim
          font.family: root.fontFamily
          font.pixelSize: Style.font.bodySmall
          wrapMode: Text.WordWrap
        }

        PanelSeparator { foreground: root.foreground }

        CursorSurface {
          id: openRow
          width: parent.width
          hasCursor: root.cursorActive && root.focusSection === "open"
          foreground: root.foreground
          implicitHeight: openLayout.implicitHeight + Style.spacing.rowPaddingX

          MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onEntered: { root.cursorActive = true; root.focusSection = "open" }
            onClicked: root.openPond()
          }

          RowLayout {
            id: openLayout
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Style.space(10)
            anchors.rightMargin: Style.space(10)
            spacing: Style.space(8)

            Text {
              text: "󰐊"
              color: root.foreground
              font.family: root.fontFamily
              font.pixelSize: Style.font.heading
              Layout.alignment: Qt.AlignVCenter
            }

            ColumnLayout {
              Layout.fillWidth: true
              spacing: Style.space(1)

              Text {
                textFormat: Text.PlainText
                Layout.fillWidth: true
                text: "Open the pond now"
                color: root.foreground
                font.family: root.fontFamily
                font.pixelSize: Style.font.body
                elide: Text.ElideRight
              }

              Text {
                textFormat: Text.PlainText
                Layout.fillWidth: true
                text: "Any key or mouse movement closes it"
                color: root.dim
                font.family: root.fontFamily
                font.pixelSize: Style.font.caption
                elide: Text.ElideRight
              }
            }
          }
        }
      }
    }
  }
}
