#include <sailfishapp.h>

#include <QGuiApplication>
#include <QQuickView>
#include <QQmlContext>
#include <QStringList>
#include <QtQml>

#include "daemoncontroller.h"
#include "pluginsettings.h"

Q_DECL_EXPORT int main(int argc, char *argv[])
{
    QScopedPointer<QGuiApplication> app(SailfishApp::application(argc, argv));
    QScopedPointer<QQuickView> view(SailfishApp::createView());

    DaemonController daemon;
    PluginSettings plugins;
    view->rootContext()->setContextProperty(QStringLiteral("Daemon"), &daemon);
    view->rootContext()->setContextProperty(QStringLiteral("PluginSettings"), &plugins);

    view->setSource(SailfishApp::pathToMainQml());
    view->show();

    const QStringList args = app->arguments();
    if (args.contains(QStringLiteral("--start-stack"))
        || args.contains(QStringLiteral("-start-stack")))
        daemon.startAoa();

    return app->exec();
}
