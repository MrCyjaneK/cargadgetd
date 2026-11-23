import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    property bool setup: true
    property bool leaving: false
    property string runError: ""
    property string selectError: ""
    property bool saved: false

    readonly property var encoders: Daemon.encoders
    readonly property var sizes: [
        { "w": 800, "h": 480 },
        { "w": 1280, "h": 720 },
        { "w": 1920, "h": 1080 },
        { "w": 2560, "h": 1440 },
        { "w": 3840, "h": 2160 }
    ]
    readonly property string encoderId: {
        var list = encoders
        if (!list || encoderBox.currentIndex < 0 || encoderBox.currentIndex >= list.length)
            return ""
        return list[encoderBox.currentIndex].id
    }
    readonly property bool canUse: !Daemon.benchRunning
                                   && !saved
                                   && Daemon.benchCanSelect
                                   && Daemon.benchEncoder === encoderId
    readonly property string failures: {
        var lines = []
        var rows = Daemon.benchRows
        for (var i = 0; i < rows.length; ++i) {
            var row = rows[i]
            if (!row.error || row.error.length === 0)
                continue
            var mode = row.mode === "max" ? qsTr("max") : row.mode
            lines.push(row.w + "×" + row.h + " " + mode + " - " + row.error)
        }
        return lines.join("\n")
    }

    function findCell(rows, w, h, mode) {
        for (var i = 0; i < rows.length; ++i) {
            var row = rows[i]
            if (row.w == w && row.h == h && row.mode == mode)
                return row
        }
        return null
    }

    function shownCell(rows, live, w, h, mode) {
        var done = findCell(rows, w, h, mode)
        if (done)
            return done
        if (live && live.w == w && live.h == h && live.mode == mode)
            return live
        return null
    }

    function scoreText(row) {
        if (!row || !(row.fps > 0))
            return "-"
        var n = Math.round(row.fps * 10) / 10
        if (Math.abs(n - Math.round(n)) < 0.05)
            return String(Math.round(n))
        return String(n)
    }

    function scoreColor(row) {
        if (!row || row.mode === "max")
            return Theme.secondaryColor
        if (row.error && row.error.length > 0)
            return Theme.errorColor
        if (!(row.fps > 0))
            return Theme.secondaryColor
        var target = row.mode === "60" ? 60 : 30
        if (Math.round(row.fps) >= target)
            return "#69f0ae"
        return Theme.errorColor
    }

    function run() {
        saved = false
        selectError = ""
        runError = Daemon.startBench(encoderId)
    }

    function applyEncoder() {
        var sections = PluginSettings.sections
        for (var s = 0; s < sections.length; ++s) {
            var rows = sections[s].rows || []
            for (var i = 0; i < rows.length; ++i) {
                if (rows[i].label === "Video encoder") {
                    selectError = PluginSettings.setRow(rows[i].section, rows[i].row, encoderId)
                    saved = selectError.length === 0
                    return
                }
            }
        }
        saved = false
        selectError = qsTr("Video encoder setting is missing")
    }

    function goPair() {
        if (leaving || Daemon.benchRunning)
            return
        leaving = true
        pageStack.push(Qt.resolvedUrl("PairPage.qml"))
    }

    Timer {
        interval: 100
        repeat: true
        running: Daemon.benchRunning
        onTriggered: Daemon.refreshBench()
    }

    Component.onCompleted: {
        encoderBox.currentIndex = 0
        Daemon.watchBench()
    }
    Component.onDestruction: Daemon.clearBench()

    onStatusChanged: {
        if (status === PageStatus.Active) {
            leaving = false
            PluginSettings.reload()
            Daemon.refreshBench()
        }
    }

    SilicaFlickable {
        anchors.fill: parent
        anchors.bottomMargin: setupBar.height
        contentHeight: column.height + Theme.paddingLarge

        Column {
            id: column
            width: parent.width
            spacing: Theme.paddingSmall

            PageHeader { title: qsTr("Encoder") }

            ComboBox {
                id: encoderBox
                width: parent.width
                label: qsTr("Encoder")
                enabled: !Daemon.benchRunning
                menu: ContextMenu {
                    Repeater {
                        model: page.encoders
                        MenuItem { text: modelData.title }
                    }
                }
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                enabled: !Daemon.benchRunning
                text: qsTr("Run")
                onClicked: page.run()
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
                visible: text.length > 0
                color: Theme.highlightColor
                text: {
                    var live = Daemon.benchLive
                    if (live && live.w > 0) {
                        var mode = live.mode === "max" ? qsTr("max") : (live.mode + " fps")
                        var secs = Math.round(live.elapsed * 10) / 10
                        return live.w + "x" + live.h + " | " + mode + " | "
                                + page.scoreText(live) + " fps"
                    }
                    return Daemon.benchStatus
                }
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
                visible: page.runError.length > 0
                color: Theme.errorColor
                font.pixelSize: Theme.fontSizeExtraSmall
                text: page.runError
            }

            Item {
                id: table
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                height: tableCol.height

                readonly property real sizeCol: width * 0.40
                readonly property real scoreCol: (width - sizeCol) / 3

                Column {
                    id: tableCol
                    width: parent.width

                    Row {
                        width: parent.width
                        height: Theme.itemSizeExtraSmall

                        Item { width: table.sizeCol; height: 1 }
                        Label {
                            width: table.scoreCol
                            height: parent.height
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            color: Theme.secondaryColor
                            font.pixelSize: Theme.fontSizeExtraSmall
                            text: "30"
                        }
                        Label {
                            width: table.scoreCol
                            height: parent.height
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            color: Theme.secondaryColor
                            font.pixelSize: Theme.fontSizeExtraSmall
                            text: "60"
                        }
                        Label {
                            width: table.scoreCol
                            height: parent.height
                            horizontalAlignment: Text.AlignHCenter
                            verticalAlignment: Text.AlignVCenter
                            color: Theme.secondaryColor
                            font.pixelSize: Theme.fontSizeExtraSmall
                            text: qsTr("max")
                        }
                    }

                    Repeater {
                        model: page.sizes

                        Row {
                            id: sizeRow
                            width: tableCol.width
                            height: Theme.itemSizeSmall
                            property int sizeW: modelData.w
                            property int sizeH: modelData.h

                            Label {
                                width: table.sizeCol
                                height: parent.height
                                verticalAlignment: Text.AlignVCenter
                                font.pixelSize: Theme.fontSizeSmall
                                text: sizeRow.sizeW + "×" + sizeRow.sizeH
                            }
                            Label {
                                width: table.scoreCol
                                height: parent.height
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                                font.pixelSize: Theme.fontSizeMedium
                                color: page.scoreColor(page.shownCell(Daemon.benchRows, Daemon.benchLive, sizeRow.sizeW, sizeRow.sizeH, "30"))
                                text: page.scoreText(page.shownCell(Daemon.benchRows, Daemon.benchLive, sizeRow.sizeW, sizeRow.sizeH, "30"))
                            }
                            Label {
                                width: table.scoreCol
                                height: parent.height
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                                font.pixelSize: Theme.fontSizeMedium
                                color: page.scoreColor(page.shownCell(Daemon.benchRows, Daemon.benchLive, sizeRow.sizeW, sizeRow.sizeH, "60"))
                                text: page.scoreText(page.shownCell(Daemon.benchRows, Daemon.benchLive, sizeRow.sizeW, sizeRow.sizeH, "60"))
                            }
                            Label {
                                width: table.scoreCol
                                height: parent.height
                                horizontalAlignment: Text.AlignHCenter
                                verticalAlignment: Text.AlignVCenter
                                font.pixelSize: Theme.fontSizeMedium
                                color: page.scoreColor(page.shownCell(Daemon.benchRows, Daemon.benchLive, sizeRow.sizeW, sizeRow.sizeH, "max"))
                                text: page.scoreText(page.shownCell(Daemon.benchRows, Daemon.benchLive, sizeRow.sizeW, sizeRow.sizeH, "max"))
                            }
                        }
                    }
                }
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: page.failures.length > 0
                color: Theme.errorColor
                font.pixelSize: Theme.fontSizeExtraSmall
                text: page.failures
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                visible: page.canUse
                text: qsTr("Use this encoder")
                onClicked: page.applyEncoder()
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
                visible: page.selectError.length > 0
                color: Theme.errorColor
                font.pixelSize: Theme.fontSizeExtraSmall
                text: page.selectError
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                horizontalAlignment: Text.AlignHCenter
                visible: page.saved
                color: Theme.secondaryColor
                font.pixelSize: Theme.fontSizeExtraSmall
                text: qsTr("Saved")
            }
        }

        VerticalScrollDecorator {}
    }

    Item {
        id: setupBar
        z: 2
        visible: page.setup
        anchors.left: parent.left
        anchors.right: parent.right
        anchors.bottom: parent.bottom
        height: visible ? cont.height + Theme.paddingLarge * 2 : 0

        Button {
            id: cont
            anchors.horizontalCenter: parent.horizontalCenter
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Theme.paddingLarge
            preferredWidth: Theme.buttonWidthLarge
            enabled: !Daemon.benchRunning
            text: qsTr("Continue")
            onClicked: page.goPair()
        }
    }
}
