import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page
    property bool _ready: false

    function apply(section, row, value) {
        PluginSettings.setRow(section, row, value)
    }

    Timer {
        id: deferApply
        interval: 0
        property var section
        property var row
        property var value
        onTriggered: page.apply(section, row, value)
    }

    Component.onCompleted: {
        PluginSettings.reload()
        _ready = true
    }

    SilicaFlickable {
        anchors.fill: parent
        contentHeight: column.height + Theme.paddingLarge

        Column {
            id: column
            width: parent.width

            PageHeader { title: qsTr("Settings") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                visible: PluginSettings.error.length > 0
                wrapMode: Text.Wrap
                color: Theme.highlightColor
                text: PluginSettings.error
            }

            Repeater {
                model: PluginSettings.sections
                delegate: Column {
                    width: page.width
                    SectionHeader { text: modelData.title }
                    Repeater {
                        model: modelData.rows
                        delegate: Column {
                            property var row: modelData
                            width: page.width

                            TextSwitch {
                                width: parent.width
                                visible: row.type === 0
                                text: row.label
                                checked: row.value === "1"
                                onClicked: page.apply(row.section, row.row, checked ? "1" : "0")
                            }

                            ComboBox {
                                id: enumBox
                                width: parent.width
                                visible: row.type === 1
                                label: row.label
                                menu: ContextMenu {
                                    Repeater {
                                        model: row.enums
                                        MenuItem {
                                            text: modelData
                                            Component.onCompleted: {
                                                if (text === row.value)
                                                    enumBox.currentIndex = index
                                            }
                                            onClicked: {
                                                deferApply.section = row.section
                                                deferApply.row = row.row
                                                deferApply.value = text
                                                deferApply.restart()
                                            }
                                        }
                                    }
                                }
                            }

                            Item {
                                id: intRow
                                width: parent.width
                                height: Theme.itemSizeSmall
                                visible: row.type === 2

                                function step(delta) {
                                    var n = parseInt(row.value, 10)
                                    if (isNaN(n))
                                        n = 0
                                    page.apply(row.section, row.row, String(n + delta))
                                }

                                IconButton {
                                    id: decButton
                                    anchors.verticalCenter: parent.verticalCenter
                                    x: Theme.horizontalPageMargin
                                    icon.source: "image://theme/icon-m-remove"
                                    onClicked: intRow.step(-1)
                                }

                                IconButton {
                                    id: incButton
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.right: parent.right
                                    anchors.rightMargin: Theme.horizontalPageMargin
                                    icon.source: "image://theme/icon-m-add"
                                    onClicked: intRow.step(1)
                                }

                                Label {
                                    id: intValue
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.right: incButton.left
                                    anchors.rightMargin: Theme.paddingMedium
                                    text: row.value
                                    color: Theme.highlightColor
                                    font.pixelSize: Theme.fontSizeLarge
                                }

                                Label {
                                    anchors.verticalCenter: parent.verticalCenter
                                    anchors.left: decButton.right
                                    anchors.right: intValue.left
                                    anchors.leftMargin: Theme.paddingMedium
                                    anchors.rightMargin: Theme.paddingSmall
                                    elide: Text.ElideRight
                                    text: row.label
                                    color: Theme.primaryColor
                                }
                            }

                            TextField {
                                width: parent.width
                                visible: row.type === 3
                                label: row.label
                                text: row.value
                                EnterKey.onClicked: {
                                    if (!page._ready)
                                        return
                                    page.apply(row.section, row.row, text)
                                    focus = false
                                }
                                onFocusChanged: if (page._ready && !focus) page.apply(row.section, row.row, text)
                            }
                        }
                    }
                }
            }

        }

        VerticalScrollDecorator {}
    }
}
