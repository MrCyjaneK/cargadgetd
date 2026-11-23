import QtQuick 2.0
import Sailfish.Silica 1.0
import "pages"

ApplicationWindow {
    id: appWindow
    initialPage: Daemon.onboardingDone
                 ? Qt.resolvedUrl("pages/MainPage.qml")
                 : (Daemon.certsMissing
                    ? Qt.resolvedUrl("pages/WelcomePage.qml")
                    : (Daemon.hasAddons()
                       ? Qt.resolvedUrl("pages/PairPage.qml")
                       : Qt.resolvedUrl("pages/AddonsPage.qml")))
    cover: Qt.resolvedUrl("cover/CoverPage.qml")
    allowedOrientations: defaultAllowedOrientations
}
