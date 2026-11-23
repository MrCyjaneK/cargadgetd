import QtQuick 2.0
import Sailfish.Silica 1.0

CoverBackground {
    property int frameMs: 33
    property bool audioOn: true
    readonly property int fps: frameMs > 0
                                 ? Math.floor((1000 + Math.floor(frameMs / 2)) / frameMs)
                                 : 30

    function readTuning() {
        var ms = 33
        var audio = true
        var sections = PluginSettings.sections
        for (var s = 0; s < sections.length; ++s) {
            var rows = sections[s].rows || []
            for (var i = 0; i < rows.length; ++i) {
                if (rows[i].label === "Frame ms") {
                    var n = parseInt(rows[i].value, 10)
                    if (n > 0)
                        ms = n
                } else if (rows[i].label === "Audio enabled") {
                    audio = rows[i].value === "1"
                }
            }
        }
        frameMs = ms
        audioOn = audio
    }

    Component.onCompleted: readTuning()

    Connections {
        target: PluginSettings
        onSectionsChanged: readTuning()
    }

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
            text: qsTr("%1 fps - audio %2")
                .arg(fps)
                .arg(audioOn ? qsTr("on") : qsTr("off"))
        }
    }
}
