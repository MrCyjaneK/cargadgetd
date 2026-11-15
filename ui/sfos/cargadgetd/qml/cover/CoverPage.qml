import QtQuick 2.0
import Sailfish.Silica 1.0

CoverBackground {
    Column {
        anchors.centerIn: parent
        width: parent.width - 2 * Theme.paddingMedium
        spacing: Theme.paddingSmall

        Label {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            color: Theme.primaryColor
            font.pixelSize: Theme.fontSizeLarge
            text: qsTr("CarGadgetd")
        }
        Label {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            wrapMode: Text.Wrap
            color: (Daemon.running && Daemon.ffsReady) ? Theme.highlightColor : Theme.secondaryColor
            text: {
                if (Daemon.running && Daemon.ffsReady)
                    return qsTr("In session")
                if (Daemon.ffsReady)
                    return qsTr("Accessory bound")
                if (Daemon.aoaRunning)
                    return qsTr("Waiting for the car")
                return qsTr("Service stopped")
            }
        }
        Label {
            width: parent.width
            horizontalAlignment: Text.AlignHCenter
            color: Theme.secondaryColor
            font.pixelSize: Theme.fontSizeExtraSmall
            text: qsTr("%1 fps · audio %2")
                .arg(Daemon.fps)
                .arg(Daemon.audioEnabled ? qsTr("on") : qsTr("off"))
        }
    }
}
