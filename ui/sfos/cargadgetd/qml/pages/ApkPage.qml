import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    property string folder: Daemon.apkStartPath()
    property var rowModel: []

    function parentFolder() {
        var cut = folder.lastIndexOf("/")
        if (cut <= 0)
            return "/"
        return folder.substring(0, cut)
    }

    function reload() {
        var out = []
        if (folder !== "/")
            out.push({ "name": "..", "path": parentFolder(), "dir": true })
        var items = Daemon.listApkDir(folder)
        for (var i = 0; i < items.length; ++i)
            out.push(items[i])
        rowModel = out
    }

    function openRow(path, dir) {
        if (Daemon.busy)
            return
        if (dir) {
            folder = path
            reload()
            return
        }
        Daemon.installApk(path)
    }

    Component.onCompleted: reload()

    SilicaListView {
        id: list
        anchors.fill: parent
        model: page.rowModel

        header: Column {
            width: list.width
            spacing: Theme.paddingSmall

            PageHeader { title: qsTr("Choose an APK") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.WrapAnywhere
                color: Theme.secondaryColor
                font.pixelSize: Theme.fontSizeExtraSmall
                text: page.folder
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: Daemon.certMessage.length > 0
                color: Daemon.certsMissing ? Theme.errorColor : Theme.highlightColor
                text: Daemon.certMessage
            }
        }

        delegate: ListItem {
            contentHeight: Theme.itemSizeSmall
            enabled: !Daemon.busy

            Label {
                anchors.verticalCenter: parent.verticalCenter
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                elide: Text.ElideRight
                color: highlighted ? Theme.highlightColor : Theme.primaryColor
                text: modelData.dir ? modelData.name + "/" : modelData.name
            }

            onClicked: page.openRow(modelData.path, modelData.dir)
        }

        VerticalScrollDecorator {}
    }
}
