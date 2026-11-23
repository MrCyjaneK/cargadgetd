import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    onStatusChanged: {
        if (status === PageStatus.Active)
            Daemon.refreshLog()
    }

    Timer {
        interval: 1000
        running: page.status === PageStatus.Active
        repeat: true
        onTriggered: Daemon.refreshLog()
    }

    SilicaFlickable {
        id: flick
        anchors.fill: parent
        contentHeight: header.height + logLabel.height + Theme.paddingLarge

        PullDownMenu {
            MenuItem {
                text: qsTr("Archive log")
                onClicked: Daemon.clearLog()
            }
            MenuItem {
                text: qsTr("Refresh")
                onClicked: Daemon.refreshLog()
            }
        }

        PageHeader {
            id: header
            title: qsTr("Log")
        }

        Label {
            id: logLabel
            anchors.top: header.bottom
            x: Theme.horizontalPageMargin
            width: parent.width - 2 * Theme.horizontalPageMargin
            wrapMode: Text.WrapAnywhere
            font.pixelSize: Theme.fontSizeExtraSmall
            font.family: "monospace"
            color: Theme.primaryColor
            text: Daemon.logText

            onTextChanged: flick.contentY = Math.max(0, flick.contentHeight - flick.height)
        }

        VerticalScrollDecorator {}
    }
}
