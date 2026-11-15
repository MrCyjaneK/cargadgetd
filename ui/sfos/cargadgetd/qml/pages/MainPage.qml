import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    SilicaFlickable {
        anchors.fill: parent
        contentHeight: column.height

        PullDownMenu {
            MenuItem {
                text: qsTr("Settings")
                onClicked: pageStack.push(Qt.resolvedUrl("SettingsPage.qml"))
            }
            MenuItem {
                text: qsTr("Log")
                onClicked: pageStack.push(Qt.resolvedUrl("LogPage.qml"))
            }
        }

        Column {
            id: column
            width: parent.width
            spacing: Theme.paddingMedium

            PageHeader { title: qsTr("CarGadgetd") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * Theme.horizontalPageMargin
                wrapMode: Text.Wrap
                color: Theme.highlightColor
                font.pixelSize: Theme.fontSizeLarge
                text: {
                    if (Daemon.running && Daemon.ffsReady)
                        return qsTr("In session")
                    if (Daemon.ffsReady)
                        return qsTr("Accessory bound")
                    if (Daemon.aoaRunning)
                        return qsTr("Waiting for the head unit")
                    return qsTr("Handshake service stopped")
                }
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * Theme.horizontalPageMargin
                wrapMode: Text.Wrap
                color: Theme.secondaryColor
                font.pixelSize: Theme.fontSizeSmall
                text: Daemon.statusText
            }

            DetailItem {
                label: qsTr("Handshake")
                value: {
                    if (!Daemon.aoaRunning)
                        return qsTr("Stopped")
                    if (Daemon.running && Daemon.ffsReady)
                        return qsTr("In session")
                    if (Daemon.ffsReady)
                        return qsTr("Accessory bound")
                    return qsTr("Waiting for START")
                }
            }
            DetailItem {
                label: qsTr("FunctionFS")
                value: Daemon.ffsReady ? qsTr("Ready") : qsTr("Not yet")
            }
            DetailItem {
                label: qsTr("MD daemon")
                value: Daemon.running ? qsTr("In session") : qsTr("Not started")
            }
            DetailItem {
                label: qsTr("Video")
                value: {
                    var sz = Daemon.videoSize === "auto" ? qsTr("auto") : Daemon.videoSize
                    return qsTr("%1 · %2 fps").arg(sz).arg(Daemon.fps)
                }
            }
            DetailItem {
                label: qsTr("Audio")
                value: Daemon.audioEnabled ? qsTr("On") : qsTr("Off")
            }
            DetailItem {
                label: qsTr("GAL certs")
                value: Daemon.certsMissing ? qsTr("Missing") : qsTr("Loaded")
            }

            Item { width: 1; height: Theme.paddingLarge }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                enabled: !Daemon.busy
                visible: !Daemon.aoaRunning
                text: qsTr("Start handshake")
                onClicked: Daemon.startAoa()
            }
            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                enabled: !Daemon.busy
                visible: Daemon.aoaRunning
                text: qsTr("Restart handshake")
                onClicked: Daemon.restartAoa()
            }
            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                enabled: !Daemon.busy
                visible: Daemon.aoaRunning
                text: qsTr("Stop handshake")
                onClicked: Daemon.stopAoa()
            }
            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                text: qsTr("View log")
                onClicked: pageStack.push(Qt.resolvedUrl("LogPage.qml"))
            }
            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                text: qsTr("Certificate details")
                onClicked: Daemon.probeCerts()
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * Theme.horizontalPageMargin
                wrapMode: Text.Wrap
                color: Daemon.certsMissing ? Theme.errorColor : Theme.primaryColor
                font.pixelSize: Theme.fontSizeExtraSmall
                font.family: "monospace"
                visible: Daemon.certProbeText.length > 0
                text: Daemon.certProbeText
            }
        }

        VerticalScrollDecorator {}
    }
}
