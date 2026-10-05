import QtQuick
import QtQuick.Layouts
import qs.Commons
import qs.Ui

// The fish in the bar and its drawer: whether the koi are your screensaver,
// which scene they swim in, and a preview. The service owns the screensaver;
// this is only a handle on it.
Panel {
  id: root
  moduleName: "io.github.ejuro.koi-screensaver"
  ipcTarget: "io.github.ejuro.koi-screensaver"

  readonly property var svc: bar && bar.shell ? bar.shell.serviceFor("io.github.ejuro.koi-screensaver") : null
  readonly property bool onIdle: svc ? svc.onIdle === true : false
  readonly property bool stayAwake: svc ? svc.stayAwake === true : false
  readonly property int idleSeconds: svc ? svc.screensaverSeconds : 150
  readonly property string scene: svc && svc.scene === "chase" ? "chase" : "pond"
  readonly property string motion: svc && svc.motion === "smooth" ? "smooth" : "balanced"

  readonly property var scenes: [
    { value: "chase", label: "Koi chase", detail: "Two koi circling a lily pad" },
    { value: "pond", label: "Pond", detail: "Five koi drifting under lily pads" }
  ]
  readonly property var motions: [
    { value: "balanced", label: "Balanced", detail: "As light as Omarchy's own screensaver" },
    { value: "smooth", label: "Smooth", detail: "Smoother, uses more CPU" }
  ]

  readonly property color foreground: bar ? bar.foreground : Color.foreground
  readonly property color dim: Qt.darker(foreground, 1.55)
  readonly property string fontFamily: bar ? bar.fontFamily : Style.font.family

  // The keyboard cursor walks these rows top to bottom.
  readonly property var rows: ["switch", "scene:chase", "scene:pond", "motion:balanced", "motion:smooth", "preview"]
  property int cursor: 0
  property bool cursorActive: false
  function at(row) { return root.cursorActive && root.rows[root.cursor] === row }
  function point(row) { root.cursorActive = true; root.cursor = root.rows.indexOf(row) }

  function durationText(s) {
    if (s < 60) return s + " s"
    return Math.floor(s / 60) + " min" + (s % 60 ? " " + (s % 60) + " s" : "")
  }

  // The header's one-line status.
  readonly property string statusText: !onIdle
    ? "Off · Omarchy's screensaver in use"
    : stayAwake
      ? "On · paused while staying awake"
      : "On · after " + durationText(idleSeconds) + " idle"

  function toggleOnIdle() { if (svc) svc.toggle() }
  function setScene(value) { if (svc) svc.setScene(value) }
  function setMotion(value) { if (svc) svc.setMotion(value) }

  // Close the drawer first, so the koi take focus from it.
  function preview() {
    root.close()
    startTimer.restart()
  }

  function activate() {
    var row = root.rows[root.cursor]
    if (row === "switch") toggleOnIdle()
    else if (row === "preview") preview()
    else if (row.indexOf("scene:") === 0) setScene(row.split(":")[1])
    else setMotion(row.split(":")[1])
  }

  implicitWidth: button.implicitWidth
  implicitHeight: button.implicitHeight

  onOpenedChanged: if (opened) {
    cursorActive = false
    cursor = 0
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
    tooltipText: root.onIdle ? "Koi Screensaver · on" : "Koi Screensaver · off"
    // Like the other icons: plain while the koi are the screensaver, faded
    // when they only come out for a preview.
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
    contentHeight: panel.fittedContentHeight(column.implicitHeight, Style.space(560))

    PanelKeyCatcher {
      id: keyCatcher
      anchors.fill: parent
      onMoveRequested: function(dx, dy) {
        if (!root.cursorActive) { root.cursorActive = true; return }
        if (dy !== 0) root.cursor = Math.max(0, Math.min(root.rows.length - 1, root.cursor + (dy > 0 ? 1 : -1)))
      }
      onActivateRequested: if (root.cursorActive) root.activate()
      onCloseRequested: root.close()
      onTabRequested: function(direction) { root.switchPanel(direction) }
      onTextKey: function(t) {
        if (t === "p" || t === "P") root.preview()
        else if (t === " ") root.toggleOnIdle()
      }

      Column {
        id: column
        width: parent.width
        spacing: Style.space(10)

        PanelHero {
          width: parent.width
          title: "Koi Screensaver"
          meta: root.statusText
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
        }

        PanelSeparator { foreground: root.foreground }

        // Whether the koi come out by themselves when you're idle.
        CursorSurface {
          id: switchRow
          width: parent.width
          hasCursor: root.at("switch")
          foreground: root.foreground
          implicitHeight: switchLayout.implicitHeight + Style.spacing.rowPaddingX

          MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onEntered: root.point("switch")
            onClicked: root.toggleOnIdle()
          }

          RowLayout {
            id: switchLayout
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.verticalCenter: parent.verticalCenter
            anchors.leftMargin: Style.space(10)
            anchors.rightMargin: Style.space(10)
            spacing: Style.space(8)

            RowText {
              Layout.fillWidth: true
              title: "Use as screensaver"
              detail: "Replaces Omarchy's own screensaver"
            }

            ToggleSwitch {
              checked: root.onIdle
              foreground: root.foreground
              Layout.alignment: Qt.AlignVCenter
              onHovered: function(on) { if (on) root.point("switch") }
              onToggled: root.toggleOnIdle()
            }
          }
        }

        PanelSeparator { foreground: root.foreground }

        Column {
          width: parent.width
          spacing: Style.space(4)

          PanelSectionHeader {
            text: "SCENE"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          Repeater {
            model: root.scenes
            ChoiceRow {
              required property var modelData
              width: column.width
              row: "scene:" + modelData.value
              chosen: root.scene === modelData.value
              label: modelData.label
              detail: modelData.detail
              onPicked: root.setScene(modelData.value)
            }
          }
        }

        PanelSeparator { foreground: root.foreground }

        Column {
          width: parent.width
          spacing: Style.space(4)

          PanelSectionHeader {
            text: "MOTION"
            foreground: root.foreground
            fontFamily: root.fontFamily
          }

          Repeater {
            model: root.motions
            ChoiceRow {
              required property var modelData
              width: column.width
              row: "motion:" + modelData.value
              chosen: root.motion === modelData.value
              label: modelData.label
              detail: modelData.detail
              onPicked: root.setMotion(modelData.value)
            }
          }
        }

        PanelSeparator { foreground: root.foreground }

        // A look at the koi now.
        CursorSurface {
          width: parent.width
          hasCursor: root.at("preview")
          foreground: root.foreground
          implicitHeight: previewLayout.implicitHeight + Style.spacing.rowPaddingX

          MouseArea {
            anchors.fill: parent
            hoverEnabled: true
            cursorShape: Qt.PointingHandCursor
            onEntered: root.point("preview")
            onClicked: root.preview()
          }

          RowLayout {
            id: previewLayout
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

            RowText {
              Layout.fillWidth: true
              title: "Preview"
              detail: "Any key or mouse movement closes it"
            }
          }
        }
      }
    }
  }

  // A row's title with a dimmer line under it.
  component RowText: ColumnLayout {
    property string title
    property string detail
    spacing: Style.space(1)

    Text {
      textFormat: Text.PlainText
      Layout.fillWidth: true
      text: parent.title
      color: root.foreground
      font.family: root.fontFamily
      font.pixelSize: Style.font.body
      elide: Text.ElideRight
    }

    Text {
      textFormat: Text.PlainText
      Layout.fillWidth: true
      text: parent.detail
      color: root.dim
      font.family: root.fontFamily
      font.pixelSize: Style.font.caption
      elide: Text.ElideRight
    }
  }

  // One of a few choices, ticked and highlighted when it's the one in use.
  component ChoiceRow: CursorSurface {
    id: choiceRow
    property string row
    property bool chosen
    property string label
    property string detail
    signal picked()

    hasCursor: root.at(row)
    current: chosen
    foreground: root.foreground
    implicitHeight: sceneLayout.implicitHeight + Style.spacing.rowPaddingX

    MouseArea {
      anchors.fill: parent
      hoverEnabled: true
      cursorShape: Qt.PointingHandCursor
      onEntered: root.point(choiceRow.row)
      onClicked: choiceRow.picked()
    }

    RowLayout {
      id: sceneLayout
      anchors.left: parent.left
      anchors.right: parent.right
      anchors.verticalCenter: parent.verticalCenter
      anchors.leftMargin: Style.space(10)
      anchors.rightMargin: Style.space(10)
      spacing: Style.space(8)

      RowText {
        Layout.fillWidth: true
        title: choiceRow.label
        detail: choiceRow.detail
      }

      Text {
        text: choiceRow.chosen ? "󰄬" : ""
        color: root.foreground
        font.family: root.fontFamily
        font.pixelSize: Style.font.heading
        Layout.alignment: Qt.AlignVCenter
      }
    }
  }
}
