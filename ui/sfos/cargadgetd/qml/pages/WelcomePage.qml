import QtQuick 2.6
import Sailfish.Silica 1.0

Page {
    id: page
    allowedOrientations: Orientation.All

    property bool tourBuilt: false
    readonly property bool showFinale: tourBuilt && slideshow.currentIndex === tour.count - 1
    readonly property var slides: [
        { title: qsTr("Cargadgetd"), body: qsTr("Talk to your car in a open source way."), icon: "", tint: "transparent"},
        { title: qsTr("Modular architecture"), body: qsTr("Cargadgetd is fully modular, you can swap in/out any parts you like, install custom plugins, change the shell - configure everything the way you want!"), icon: "", tint: "transparent"},
        { title: "", body: "", icon: "", tint: "transparent" }
    ]

    function advance() {
        if (slideshow.currentIndex < tour.count - 1)
            slideshow.currentIndex = slideshow.currentIndex + 1
    }

    ListModel { id: tour }

    Component.onCompleted: {
        for (var i = 0; i < page.slides.length; ++i)
            tour.append(page.slides[i])
        page.tourBuilt = true
    }

    SilicaFlickable {
        anchors.fill: parent
        anchors.bottomMargin: actions.height + dots.height + Theme.paddingLarge * 2

        SlideshowView {
            id: slideshow
            anchors.fill: parent
            opacity: page.tourBuilt ? 1 : 0
            itemWidth: width
            model: tour

            delegate: Item {
                width: slideshow.itemWidth
                height: slideshow.height

                Column {
                    anchors.centerIn: parent
                    width: parent.width - Theme.horizontalPageMargin * 2
                    spacing: Theme.paddingLarge
                    visible: index < tour.count - 1

                    Rectangle {
                        anchors.horizontalCenter: parent.horizontalCenter
                        width: Theme.itemSizeHuge
                        height: width
                        radius: width / 2
                        color: model.tint

                        Icon {
                            anchors.centerIn: parent
                            source: model.icon
                            highlighted: true
                        }
                    }

                    Label {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.Wrap
                        font.pixelSize: Theme.fontSizeExtraLarge
                        text: model.title
                    }

                    Label {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.Wrap
                        color: Theme.secondaryColor
                        text: model.body
                    }
                }

                Column {
                    anchors.centerIn: parent
                    width: parent.width - Theme.horizontalPageMargin * 2
                    spacing: Theme.paddingLarge
                    visible: index === tour.count - 1

                    Label {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        font.pixelSize: Theme.fontSizeExtraLarge
                        font.bold: true
                        text: qsTr("This is Cargadgetd")
                    }

                    Label {
                        width: parent.width
                        horizontalAlignment: Text.AlignHCenter
                        wrapMode: Text.Wrap
                        color: Theme.secondaryColor
                        text: qsTr("We will get you though the setup process, and install the addons you want!")
                    }
                }
            }
        }
    }

    Row {
        id: dots
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: actions.top
        anchors.bottomMargin: Theme.paddingMedium
        spacing: Theme.paddingSmall
        visible: page.tourBuilt

        Repeater {
            model: tour.count
            delegate: Rectangle {
                width: Theme.paddingSmall
                height: width
                radius: width / 2
                color: index === slideshow.currentIndex ? Theme.highlightColor : Theme.secondaryColor
            }
        }
    }

    Item {
        id: actions
        anchors.horizontalCenter: parent.horizontalCenter
        anchors.bottom: parent.bottom
        anchors.bottomMargin: Theme.paddingLarge
        width: page.showFinale ? setupButton.width : nextButton.width
        height: page.showFinale ? setupButton.height : nextButton.height

        Button {
            id: nextButton
            visible: !page.showFinale
            text: qsTr("Next")
            onClicked: page.advance()
        }

        Button {
            id: setupButton
            visible: page.showFinale
            text: qsTr("Set up the key")
            onClicked: pageStack.push(Qt.resolvedUrl("KeyPage.qml"))
        }
    }
}
