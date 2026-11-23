import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page
    property bool setup: true
    property bool leaving: false
    property bool addonsPresent: false
    property bool needAddons: false

    function load() {
        Daemon.loadAddons()
    }

    function goEncode() {
        if (leaving)
            return
        leaving = true
        pageStack.push(Qt.resolvedUrl("EncodePage.qml"), { "setup": true })
    }

    function finishSetup() {
        if (leaving)
            return
        if (!addonsPresent) {
            needAddons = true
            return
        }
        goEncode()
    }

    function refreshAddons() {
        addonsPresent = Daemon.hasAddons()
        if (addonsPresent)
            needAddons = false
        Daemon.refreshAddons()
    }

    onStatusChanged: {
        if (status === PageStatus.Active) {
            leaving = false
            refreshAddons()
        }
    }

    Connections {
        target: Qt.application
        onStateChanged: {
            if (Qt.application.state === Qt.ApplicationActive)
                page.refreshAddons()
        }
    }

    Component.onCompleted: load()

    SilicaFlickable {
        anchors.fill: parent
        anchors.bottomMargin: page.setup ? setupBar.height : 0
        clip: true
        contentHeight: column.height + Theme.paddingLarge

        Column {
            id: column
            width: parent.width

            PageHeader { title: qsTr("Add-ons") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: Daemon.addonsError.length > 0
                color: Theme.highlightColor
                text: Daemon.addonsError
            }

            BusyIndicator {
                anchors.horizontalCenter: parent.horizontalCenter
                running: Daemon.addonsLoading
                visible: Daemon.addonsLoading
                size: BusyIndicatorSize.Medium
            }

            Repeater {
                model: Daemon.addonRows

                delegate: Column {
                    width: page.width

                    SectionHeader {
                        text: modelData.group
                        visible: index === 0 || Daemon.addonRows[index - 1].group !== modelData.group
                    }

                    ListItem {
                        id: item
                        width: parent.width
                        contentHeight: modelData.description.length > 0
                                        ? Theme.itemSizeMedium
                                        : Theme.itemSizeSmall

                        Image {
                            id: icon
                            anchors.verticalCenter: parent.verticalCenter
                            x: Theme.horizontalPageMargin
                            width: modelData.icon.length > 0 && status === Image.Ready
                                   ? Theme.iconSizeSmall : 0
                            height: width
                            asynchronous: true
                            fillMode: Image.PreserveAspectFit
                            source: modelData.icon
                        }

                        Label {
                            id: stateLabel
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.right: parent.right
                            anchors.rightMargin: Theme.horizontalPageMargin
                            font.pixelSize: Theme.fontSizeExtraSmall
                            color: modelData.hasUpdate
                                   ? Theme.highlightColor
                                   : Theme.secondaryColor
                            text: modelData.hasUpdate
                                  ? qsTr("Update")
                                  : (modelData.installed ? qsTr("Installed") : "")
                        }

                        Column {
                            anchors.verticalCenter: parent.verticalCenter
                            anchors.left: icon.right
                            anchors.leftMargin: icon.width > 0 ? Theme.paddingMedium : 0
                            anchors.right: stateLabel.left
                            anchors.rightMargin: stateLabel.text.length > 0
                                                 ? Theme.paddingSmall : 0

                            Label {
                                width: parent.width
                                elide: Text.ElideRight
                                color: item.highlighted ? Theme.highlightColor : Theme.primaryColor
                                text: modelData.title
                            }
                            Label {
                                width: parent.width
                                visible: modelData.description.length > 0
                                elide: Text.ElideRight
                                color: item.highlighted
                                      ? Theme.secondaryHighlightColor
                                      : Theme.secondaryColor
                                font.pixelSize: Theme.fontSizeExtraSmall
                                text: modelData.description
                            }
                        }

                        onClicked: Qt.openUrlExternally(modelData.url)
                    }
                }
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
        height: visible ? barColumn.height + Theme.paddingLarge * 2 : 0

        Rectangle {
            anchors.fill: parent
            color: Theme.rgba(Theme.highlightDimmerColor, 1)
        }

        Column {
            id: barColumn
            anchors.left: parent.left
            anchors.right: parent.right
            anchors.bottom: parent.bottom
            anchors.bottomMargin: Theme.paddingLarge
            spacing: Theme.paddingSmall

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                visible: page.needAddons
                wrapMode: Text.Wrap
                horizontalAlignment: Text.AlignHCenter
                color: Theme.highlightColor
                font.pixelSize: Theme.fontSizeExtraSmall
                text: qsTr("Install a shell and an app to continue.")
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: parent.width - 2 * Theme.horizontalPageMargin
                visible: page.needAddons
                text: qsTr("I know what I am doing")
                onClicked: page.goEncode()
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                text: qsTr("Continue")
                onClicked: page.finishSetup()
            }
        }
    }
}
