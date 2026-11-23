import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    readonly property bool inSession: Daemon.running && Daemon.ffsReady

    SilicaFlickable {
        anchors.fill: parent
        anchors.bottomMargin: dock.height + Theme.paddingLarge + Theme.paddingMedium
        contentHeight: column.height

        PullDownMenu {
            MenuItem {
                text: qsTr("Restart handshake")
                visible: Daemon.aoaRunning
                enabled: !Daemon.busy
                onClicked: Daemon.restartAoa()
            }
            MenuItem {
                text: qsTr("Add-ons")
                onClicked: pageStack.push(Qt.resolvedUrl("AddonsPage.qml"), { "setup": false })
            }
            MenuItem {
                text: qsTr("Pairing")
                onClicked: pageStack.push(Qt.resolvedUrl("PairPage.qml"), { "setup": false })
            }
            MenuItem {
                text: qsTr("Encoder")
                onClicked: pageStack.push(Qt.resolvedUrl("EncodePage.qml"), { "setup": false })
            }
            MenuItem {
                text: qsTr("Certificate")
                onClicked: {
                    Daemon.probeCerts()
                    pageStack.push(Qt.resolvedUrl("KeyPage.qml"))
                }
            }
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

            PageHeader { title: qsTr("CarGadgetd") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * Theme.horizontalPageMargin
                wrapMode: Text.Wrap
                color: page.inSession ? Theme.highlightColor : Theme.primaryColor
                font.pixelSize: Theme.fontSizeLarge
                text: {
                    if (page.inSession)
                        return qsTr("In session")
                    if (Daemon.ffsReady)
                        return qsTr("Accessory bound")
                    if (Daemon.aoaRunning)
                        return qsTr("Waiting for the head unit")
                    return qsTr("Handshake service stopped")
                }
            }

            Item { width: 1; height: Theme.paddingSmall }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * Theme.horizontalPageMargin
                wrapMode: Text.Wrap
                color: Theme.secondaryColor
                font.pixelSize: Theme.fontSizeSmall
                text: Daemon.statusText
            }

            SectionHeader { text: qsTr("Link") }

            DetailItem {
                label: qsTr("Handshake")
                value: Daemon.aoaRunning ? qsTr("Waiting for START") : qsTr("Stopped")
            }
            DetailItem {
                label: qsTr("Accessory")
                value: Daemon.ffsReady ? qsTr("Bound") : qsTr("Not bound")
            }
            DetailItem {
                label: qsTr("Session")
                value: Daemon.running ? qsTr("In session") : qsTr("Idle")
            }
        }

        VerticalScrollDecorator {}
    }

    Item {
        id: dock
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Theme.paddingLarge
        height: actionButton.height + (busy.running ? busy.height + Theme.paddingMedium : 0)

        BusyIndicator {
            id: busy
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: actionButton.top
            anchors.bottomMargin: Theme.paddingMedium
            running: Daemon.busy
            visible: Daemon.busy
            size: BusyIndicatorSize.Medium
        }

        Button {
            id: actionButton
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            preferredWidth: Theme.buttonWidthLarge
            enabled: !Daemon.busy
            text: Daemon.aoaRunning ? qsTr("Stop handshake") : qsTr("Start handshake")
            onClicked: {
                if (Daemon.aoaRunning)
                    Daemon.stopAoa()
                else
                    Daemon.startAoa()
            }
        }
    }
}
