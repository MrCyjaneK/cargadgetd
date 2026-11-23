import QtQuick 2.0
import Sailfish.Silica 1.0

Page {
    id: page

    property bool leaving: false

    function finish() {
        if (leaving)
            return
        leaving = true
        if (Daemon.hasAddons())
            pageStack.push(Qt.resolvedUrl("PairPage.qml"))
        else
            pageStack.push(Qt.resolvedUrl("AddonsPage.qml"), { "setup": true })
    }

    onStatusChanged: {
        if (status === PageStatus.Active)
            leaving = false
    }

    Connections {
        target: Daemon
        onCertInstallFinished: page.finish()
    }

    SilicaFlickable {
        anchors.fill: parent
        contentHeight: column.height + Theme.paddingLarge

        Column {
            id: column
            width: parent.width
            spacing: Theme.paddingMedium

            PageHeader { title: qsTr("Certificate") }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                color: Theme.secondaryColor
                text: qsTr("In order to communicate with your car we need TLS key embedded in Android Auto .apk. This app is **NOT** going to be installed in Waydroid/AlienDalvik, we will just search it for the key. You can provide your own .apk or you can try downloading latset version automagically from GrapheneOS app store.")
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                enabled: !Daemon.busy
                text: qsTr("Try download")
                onClicked: Daemon.downloadCerts()
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                enabled: !Daemon.busy
                text: qsTr("Provide APK")
                onClicked: pageStack.push(Qt.resolvedUrl("ApkPage.qml"))
            }

            Button {
                anchors.horizontalCenter: parent.horizontalCenter
                preferredWidth: Theme.buttonWidthLarge
                visible: !Daemon.certsMissing
                enabled: !Daemon.busy
                text: qsTr("Continue")
                onClicked: page.finish()
            }

            BusyIndicator {
                anchors.horizontalCenter: parent.horizontalCenter
                running: Daemon.busy
                size: BusyIndicatorSize.Medium
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: Daemon.certMessage.length > 0
                color: Daemon.certsMissing ? Theme.errorColor : Theme.highlightColor
                text: Daemon.certMessage
            }

            Label {
                x: Theme.horizontalPageMargin
                width: parent.width - 2 * x
                wrapMode: Text.Wrap
                visible: Daemon.certProbeText.length > 0
                color: Theme.primaryColor
                font.pixelSize: Theme.fontSizeExtraSmall
                font.family: "monospace"
                text: Daemon.certProbeText
            }
        }

        VerticalScrollDecorator {}
    }
}
