import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    property bool setup: true
    property bool loaded: false
    property bool leaving: false
    property var transportRow: null
    property var deviceRow: null
    property var deviceEnums: []
    property string transport: "usb"
    property string device: "none"
    property int deviceIndex: 0
    property string errorText: ""

    readonly property var transports: Daemon.transports
    readonly property var mode: {
        var list = transports
        if (!list)
            return null
        for (var i = 0; i < list.length; ++i) {
            var item = list[i]
            if (item.id === transport || item.title === transport)
                return item
        }
        return null
    }
    readonly property bool needsDevice: mode && mode.needsPeer
    readonly property bool deviceReady: device.length > 0 && device !== "none"
    readonly property bool canContinue: loaded
            && (!transportRow || !(mode && mode.requirePeer) || deviceReady)

    function findRows() {
        var transportFound = null
        var deviceFound = null
        var sections = PluginSettings.sections
        for (var s = 0; s < sections.length; ++s) {
            var rows = sections[s].rows || []
            for (var i = 0; i < rows.length; ++i) {
                if (rows[i].label === "Transport")
                    transportFound = rows[i]
                else if (rows[i].label === "Head unit")
                    deviceFound = rows[i]
            }
        }
        transportRow = transportFound
        deviceRow = deviceFound
        deviceEnums = deviceFound ? deviceFound.enums : []
        transport = transportFound && transportFound.value ? transportFound.value : "usb"
        device = deviceFound && deviceFound.value ? deviceFound.value : "none"
        var idx = 0
        for (var e = 0; e < deviceEnums.length; ++e) {
            if (deviceEnums[e] === device)
                idx = e
        }
        deviceIndex = idx
    }

    function apply(row, value) {
        if (!row || row.value === value)
            return
        errorText = PluginSettings.setRow(row.section, row.row, value)
    }

    function clearFirstRun() {
        var sections = PluginSettings.sections
        for (var s = 0; s < sections.length; ++s) {
            var rows = sections[s].rows || []
            for (var i = 0; i < rows.length; ++i) {
                if (rows[i].label === "First run") {
                    page.apply(rows[i], "0")
                    return
                }
            }
        }
    }

    function finish() {
        if (!canContinue || leaving)
            return
        leaving = true
        if (!setup) {
            pageStack.pop()
            return
        }
        clearFirstRun()
        Daemon.completeOnboarding()
        pageStack.replaceAbove(null, Qt.resolvedUrl("MainPage.qml"), {}, PageStackAction.Immediate)
    }

    Timer {
        id: deferApply
        interval: 0
        property var row
        property string value
        onTriggered: page.apply(row, value)
    }

    Connections {
        target: PluginSettings
        onSectionsChanged: page.findRows()
    }

    Connections {
        target: Qt.application
        onStateChanged: {
            if (Qt.application.state === Qt.ApplicationActive)
                PluginSettings.reload()
        }
    }

    onStatusChanged: {
        if (status === PageStatus.Active) {
            leaving = false
            PluginSettings.reload()
        }
    }

    Component.onCompleted: {
        PluginSettings.reload()
        findRows()
        loaded = true
    }

    SilicaFlickable {
        anchors.fill: parent
        anchors.bottomMargin: setupBar.height
        contentHeight: column.height + Theme.paddingLarge

        Column {
            id: column
            width: parent.width
            spacing: Theme.paddingMedium

            PageHeader { title: qsTr("Pairing") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: page.loaded && (!page.transports || page.transports.length === 0)
                color: Theme.highlightColor
                text: qsTr("Pairing modes are not available yet. You can set them later.")
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: page.loaded && !page.transportRow
                         && page.transports && page.transports.length > 0
                color: Theme.highlightColor
                text: qsTr("Transport settings are not available yet. You can set them later.")
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: page.errorText.length > 0
                color: Theme.highlightColor
                text: page.errorText
            }

            SectionHeader {
                text: qsTr("Mode")
                visible: page.transports && page.transports.length > 0
            }

            Repeater {
                model: page.transports

                delegate: ListItem {
                    id: modeItem
                    width: page.width
                    contentHeight: Math.max(Theme.itemSizeSmall,
                                             modeColumn.height + Theme.paddingMedium)

                    Column {
                        id: modeColumn
                        anchors.verticalCenter: parent.verticalCenter
                        x: Theme.horizontalPageMargin
                        width: parent.width - 2 * x
                        spacing: Theme.paddingSmall

                        Label {
                            width: parent.width
                            wrapMode: Text.Wrap
                            color: page.mode && page.mode.id === modelData.id
                                   ? Theme.highlightColor
                                   : Theme.primaryColor
                            text: modelData.title
                        }
                        Label {
                            width: parent.width
                            wrapMode: Text.Wrap
                            visible: text.length > 0
                            color: Theme.secondaryColor
                            font.pixelSize: Theme.fontSizeExtraSmall
                            text: modelData.body || ""
                        }
                    }

                    // The settings row shows the title. Writing that title again
                    // is a no-op; the id would look like a change and restart the gadget.
                    onClicked: page.apply(page.transportRow, modelData.title)
                }
            }

            SectionHeader {
                text: qsTr("Head unit")
                visible: page.needsDevice && page.deviceRow
            }

            ComboBox {
                id: deviceBox
                width: parent.width
                visible: page.needsDevice && page.deviceRow
                label: qsTr("Device")
                currentIndex: page.deviceIndex
                menu: ContextMenu {
                    Repeater {
                        model: page.deviceEnums
                        MenuItem {
                            text: modelData
                            onClicked: {
                                deferApply.row = page.deviceRow
                                deferApply.value = text
                                deferApply.restart()
                            }
                        }
                    }
                }
            }
        }

        VerticalScrollDecorator {}
    }

    Item {
        id: setupBar
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: cont.height + Theme.paddingLarge * 2
                + (hint.visible ? hint.height + Theme.paddingSmall : 0)

        Label {
            id: hint
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: cont.top
            anchors.leftMargin: Theme.horizontalPageMargin
            anchors.rightMargin: Theme.horizontalPageMargin
            anchors.bottomMargin: Theme.paddingSmall
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
            visible: text.length > 0 && page.mode && page.mode.requirePeer && !page.deviceReady
            color: Theme.secondaryColor
            font.pixelSize: Theme.fontSizeExtraSmall
            text: page.mode && page.mode.hint ? page.mode.hint : ""
        }

        Button {
            id: cont
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Theme.paddingLarge
            preferredWidth: Theme.buttonWidthLarge
            enabled: page.canContinue
            text: qsTr("Continue")
            onClicked: page.finish()
        }
    }
}
