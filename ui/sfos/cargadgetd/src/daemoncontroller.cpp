#include "daemoncontroller.h"

#include <QByteArray>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QMetaObject>
#include <QVariantMap>
#include <QtConcurrent>

#include <dlfcn.h>
#include <stdlib.h>

namespace {
const char *kLib = "/usr/lib/cargadgetd/libcargadgetdcontrol.so";

struct Api {
    void *handle = nullptr;
    QString error;

    int (*mdRunning)() = nullptr;
    int (*ffsReady)() = nullptr;
    int (*aoaRunning)() = nullptr;
    char *(*aoaStart)() = nullptr;
    char *(*aoaStop)() = nullptr;
    char *(*aoaRestart)() = nullptr;
    char *(*logText)() = nullptr;
    void (*logClear)() = nullptr;
    int (*certsReady)() = nullptr;
    char *(*certsProbe)() = nullptr;
    char *(*certsDownload)() = nullptr;
    char *(*certsInstall)(const char *) = nullptr;
    int (*onboardingDone)() = nullptr;
    int (*addonsPresent)() = nullptr;
    char *(*addonsFetch)() = nullptr;
    char *(*addonsRefresh)() = nullptr;
    int (*addonsCount)() = nullptr;
    char *(*addonsGroup)(int) = nullptr;
    char *(*addonsTitle)(int) = nullptr;
    char *(*addonsDescription)(int) = nullptr;
    char *(*addonsIcon)(int) = nullptr;
    int (*addonsInstalled)(int) = nullptr;
    int (*addonsUpdate)(int) = nullptr;
    char *(*addonsUrl)(int) = nullptr;
    char *(*apkStart)() = nullptr;
    int (*apkList)(const char *) = nullptr;
    char *(*apkName)(int) = nullptr;
    char *(*apkPath)(int) = nullptr;
    int (*apkDir)(int) = nullptr;
    char *(*benchStart)(const char *) = nullptr;
    int (*benchRunning)() = nullptr;
    char *(*benchStatus)() = nullptr;
    char *(*benchResult)() = nullptr;
    void (*benchWatch)() = nullptr;
    void (*benchClear)() = nullptr;
    int (*encoderCount)() = nullptr;
    char *(*encoderId)(int) = nullptr;
    char *(*encoderTitle)(int) = nullptr;
    int (*transportCount)() = nullptr;
    char *(*transportId)(int) = nullptr;
    char *(*transportTitle)(int) = nullptr;
    char *(*transportBody)(int) = nullptr;
    char *(*transportHint)(int) = nullptr;
    int (*transportNeedsPeer)(int) = nullptr;
    int (*transportRequirePeer)(int) = nullptr;
};

Api g;

template <typename T>
void bind(T &fn, const char *name)
{
    fn = reinterpret_cast<T>(dlsym(g.handle, name));
}

void loadLib()
{
    static bool tried = false;
    if (tried)
        return;
    tried = true;
    g.handle = dlopen(kLib, RTLD_NOW);
    if (!g.handle) {
        const char *e = dlerror();
        g.error = e ? QString::fromLocal8Bit(e) : QStringLiteral("dlopen failed");
        g.error = QStringLiteral("libcargadgetdcontrol.so is missing (%1)").arg(g.error);
        return;
    }
    bind(g.mdRunning, "cargadgetd_internal_md_running");
    bind(g.ffsReady, "cargadgetd_internal_ffs_ready");
    bind(g.aoaRunning, "cargadgetd_internal_aoa_running");
    bind(g.aoaStart, "cargadgetd_internal_aoa_start");
    bind(g.aoaStop, "cargadgetd_internal_aoa_stop");
    bind(g.aoaRestart, "cargadgetd_internal_aoa_restart");
    bind(g.logText, "cargadgetd_internal_log_text");
    bind(g.logClear, "cargadgetd_internal_log_clear");
    bind(g.certsReady, "cargadgetd_internal_certs_ready");
    bind(g.certsProbe, "cargadgetd_internal_certs_probe");
    bind(g.certsDownload, "cargadgetd_internal_certs_download");
    bind(g.certsInstall, "cargadgetd_internal_certs_install");
    bind(g.onboardingDone, "cargadgetd_internal_onboarding_done");
    bind(g.addonsPresent, "cargadgetd_internal_addons_present");
    bind(g.addonsFetch, "cargadgetd_internal_addons_fetch");
    bind(g.addonsRefresh, "cargadgetd_internal_addons_refresh");
    bind(g.addonsCount, "cargadgetd_internal_addons_count");
    bind(g.addonsGroup, "cargadgetd_internal_addons_group");
    bind(g.addonsTitle, "cargadgetd_internal_addons_title");
    bind(g.addonsDescription, "cargadgetd_internal_addons_description");
    bind(g.addonsIcon, "cargadgetd_internal_addons_icon");
    bind(g.addonsInstalled, "cargadgetd_internal_addons_installed");
    bind(g.addonsUpdate, "cargadgetd_internal_addons_update");
    bind(g.addonsUrl, "cargadgetd_internal_addons_url");
    bind(g.apkStart, "cargadgetd_internal_apk_start");
    bind(g.apkList, "cargadgetd_internal_apk_list");
    bind(g.apkName, "cargadgetd_internal_apk_name");
    bind(g.apkPath, "cargadgetd_internal_apk_path");
    bind(g.apkDir, "cargadgetd_internal_apk_dir");
    bind(g.benchStart, "cargadgetd_internal_bench_start");
    bind(g.benchRunning, "cargadgetd_internal_bench_running");
    bind(g.benchStatus, "cargadgetd_internal_bench_status");
    bind(g.benchResult, "cargadgetd_internal_bench_result");
    bind(g.benchWatch, "cargadgetd_internal_bench_watch");
    bind(g.benchClear, "cargadgetd_internal_bench_clear");
    bind(g.encoderCount, "cargadgetd_internal_encoder_count");
    bind(g.encoderId, "cargadgetd_internal_encoder_id");
    bind(g.encoderTitle, "cargadgetd_internal_encoder_title");
    bind(g.transportCount, "cargadgetd_internal_transport_count");
    bind(g.transportId, "cargadgetd_internal_transport_id");
    bind(g.transportTitle, "cargadgetd_internal_transport_title");
    bind(g.transportBody, "cargadgetd_internal_transport_body");
    bind(g.transportHint, "cargadgetd_internal_transport_hint");
    bind(g.transportNeedsPeer, "cargadgetd_internal_transport_needs_peer");
    bind(g.transportRequirePeer, "cargadgetd_internal_transport_require_peer");
}

QString takeCString(char *p)
{
    if (!p)
        return QString();
    const QString s = QString::fromUtf8(p);
    free(p);
    return s;
}

QVariantList loadEncoders()
{
    QVariantList out;
    if (!g.encoderCount || !g.encoderId || !g.encoderTitle)
        return out;
    const int n = g.encoderCount();
    for (int i = 0; i < n; ++i) {
        QVariantMap row;
        row.insert(QStringLiteral("id"), takeCString(g.encoderId(i)));
        row.insert(QStringLiteral("title"), takeCString(g.encoderTitle(i)));
        out.append(row);
    }
    return out;
}

QVariantList loadTransports()
{
    QVariantList out;
    if (!g.transportCount || !g.transportId || !g.transportTitle || !g.transportBody
        || !g.transportHint || !g.transportNeedsPeer || !g.transportRequirePeer)
        return out;
    const int n = g.transportCount();
    for (int i = 0; i < n; ++i) {
        QVariantMap row;
        row.insert(QStringLiteral("id"), takeCString(g.transportId(i)));
        row.insert(QStringLiteral("title"), takeCString(g.transportTitle(i)));
        row.insert(QStringLiteral("body"), takeCString(g.transportBody(i)));
        row.insert(QStringLiteral("hint"), takeCString(g.transportHint(i)));
        row.insert(QStringLiteral("needsPeer"), g.transportNeedsPeer(i) != 0);
        row.insert(QStringLiteral("requirePeer"), g.transportRequirePeer(i) != 0);
        out.append(row);
    }
    return out;
}

QString call0(char *(*fn)())
{
    if (!fn)
        return g.error.isEmpty() ? QStringLiteral("libcargadgetdcontrol.so is missing") : g.error;
    return takeCString(fn());
}

bool callBool(int (*fn)())
{
    if (!fn)
        return false;
    return fn() != 0;
}
} // namespace

DaemonController::DaemonController(QObject *parent)
    : QObject(parent)
{
    loadLib();
    m_encoders = loadEncoders();
    m_transports = loadTransports();
    connect(&m_poll, &QTimer::timeout, this, &DaemonController::refresh);
    m_poll.start(1500);
    refresh();
    refreshLog();
    if (!g.handle && m_statusText.isEmpty())
        setStatusText(g.error);
}

void DaemonController::refresh()
{
    setRunning(callBool(g.mdRunning));
    setFfsReady(callBool(g.ffsReady));
    setAoaRunning(callBool(g.aoaRunning));

    const bool missing = !callBool(g.certsReady);
    if (m_certsMissing != missing) {
        m_certsMissing = missing;
        emit certsMissingChanged();
        if (missing)
            setCertProbe(call0(g.certsProbe));
        else if (m_certProbeText.startsWith(QLatin1String("Certificate is missing")))
            setCertProbe(QString());
    } else if (missing && m_certProbeText.isEmpty()) {
        setCertProbe(call0(g.certsProbe));
    }

    const bool done = callBool(g.onboardingDone);
    if (m_onboardingDone != done) {
        m_onboardingDone = done;
        emit onboardingDoneChanged();
    }
}

void DaemonController::setBusy(bool v)
{
    if (m_busy == v)
        return;
    m_busy = v;
    emit busyChanged();
}

void DaemonController::setRunning(bool v)
{
    if (m_running == v)
        return;
    m_running = v;
    emit runningChanged();
}

void DaemonController::setFfsReady(bool v)
{
    if (m_ffsReady == v)
        return;
    m_ffsReady = v;
    emit ffsReadyChanged();
}

void DaemonController::setAoaRunning(bool v)
{
    if (m_aoaRunning == v)
        return;
    m_aoaRunning = v;
    emit aoaRunningChanged();
}

void DaemonController::setStatusText(const QString &v)
{
    if (m_statusText == v)
        return;
    m_statusText = v;
    emit statusTextChanged();
}

void DaemonController::setCertMessage(const QString &v)
{
    if (m_certMessage == v)
        return;
    m_certMessage = v;
    emit certMessageChanged();
}

void DaemonController::setCertProbe(const QString &v)
{
    if (m_certProbeText == v)
        return;
    m_certProbeText = v;
    emit certProbeTextChanged();
}

void DaemonController::setAddonsError(const QString &v)
{
    if (m_addonsError == v)
        return;
    m_addonsError = v;
    emit addonsErrorChanged();
}

void DaemonController::setAddonsLoading(bool v)
{
    if (m_addonsLoading == v)
        return;
    m_addonsLoading = v;
    emit addonsLoadingChanged();
}

void DaemonController::runAsync(const QString &busyMsg, const std::function<QString()> &work)
{
    if (m_busy) {
        setStatusText(QStringLiteral("Busy…"));
        return;
    }
    setBusy(true);
    setStatusText(busyMsg);
    QtConcurrent::run([this, work]() {
        const QString result = work();
        QMetaObject::invokeMethod(this, "finishAsync", Qt::QueuedConnection,
                                  Q_ARG(QString, result));
    });
}

void DaemonController::finishAsync(const QString &status)
{
    setBusy(false);
    refresh();
    refreshLog();
    setStatusText(status);
}

void DaemonController::startAoa()
{
    runAsync(QStringLiteral("Starting handshake service…"), []() { return call0(g.aoaStart); });
}

void DaemonController::stopAoa()
{
    runAsync(QStringLiteral("Stopping handshake service…"), []() { return call0(g.aoaStop); });
}

void DaemonController::restartAoa()
{
    runAsync(QStringLiteral("Restarting handshake service…"), []() { return call0(g.aoaRestart); });
}

void DaemonController::refreshLog()
{
    const QString text = call0(g.logText);
    if (m_logText == text)
        return;
    m_logText = text;
    emit logTextChanged();
}

void DaemonController::clearLog()
{
    if (g.logClear)
        g.logClear();
    refreshLog();
}

void DaemonController::probeCerts()
{
    setCertProbe(call0(g.certsProbe));
}

void DaemonController::runCertAsync(const QString &busyMsg, const std::function<QString()> &work)
{
    if (m_busy) {
        setCertMessage(QStringLiteral("Busy…"));
        return;
    }
    setBusy(true);
    setCertMessage(busyMsg);
    QtConcurrent::run([this, work]() {
        const QString result = work();
        QMetaObject::invokeMethod(this, "finishCertAsync", Qt::QueuedConnection,
                                  Q_ARG(QString, result));
    });
}

void DaemonController::finishCertAsync(const QString &status)
{
    refresh();
    const bool ok = status.isEmpty();
    setCertMessage(ok ? QStringLiteral("Certificate installed") : status);
    if (ok)
        probeCerts();
    setBusy(false);
    if (ok)
        emit certInstallFinished();
}

void DaemonController::downloadCerts()
{
    runCertAsync(QStringLiteral("Downloading Android Auto…"), []() { return call0(g.certsDownload); });
}

void DaemonController::installApk(const QString &path)
{
    runCertAsync(QStringLiteral("Reading APK…"), [path]() {
        if (!g.certsInstall)
            return g.error.isEmpty() ? QStringLiteral("libcargadgetdcontrol.so is missing") : g.error;
        const QByteArray p = path.toUtf8();
        return takeCString(g.certsInstall(p.constData()));
    });
}

void DaemonController::completeOnboarding()
{
    // Pair page already stored First run through PluginSettings.
    refresh();
}

bool DaemonController::hasAddons() const
{
    return callBool(g.addonsPresent);
}

QString DaemonController::apkStartPath() const
{
    return call0(g.apkStart);
}

QVariantList DaemonController::listApkDir(const QString &path) const
{
    QVariantList out;
    if (!g.apkList || !g.apkName || !g.apkPath || !g.apkDir)
        return out;
    const QByteArray p = path.toUtf8();
    const int n = g.apkList(p.constData());
    for (int i = 0; i < n; ++i) {
        QVariantMap row;
        row.insert(QStringLiteral("name"), takeCString(g.apkName(i)));
        row.insert(QStringLiteral("path"), takeCString(g.apkPath(i)));
        row.insert(QStringLiteral("dir"), g.apkDir(i) != 0);
        out.append(row);
    }
    return out;
}

void DaemonController::copyAddonRows()
{
    QVariantList rows;
    if (g.addonsCount && g.addonsGroup && g.addonsTitle && g.addonsDescription
        && g.addonsIcon && g.addonsInstalled && g.addonsUpdate && g.addonsUrl) {
        const int n = g.addonsCount();
        for (int i = 0; i < n; ++i) {
            QVariantMap row;
            row.insert(QStringLiteral("group"), takeCString(g.addonsGroup(i)));
            row.insert(QStringLiteral("title"), takeCString(g.addonsTitle(i)));
            row.insert(QStringLiteral("description"), takeCString(g.addonsDescription(i)));
            row.insert(QStringLiteral("icon"), takeCString(g.addonsIcon(i)));
            row.insert(QStringLiteral("installed"), g.addonsInstalled(i) != 0);
            row.insert(QStringLiteral("hasUpdate"), g.addonsUpdate(i) != 0);
            row.insert(QStringLiteral("url"), takeCString(g.addonsUrl(i)));
            rows.append(row);
        }
    }
    m_addonRows = rows;
    emit addonRowsChanged();
}

void DaemonController::loadAddons()
{
    if (m_addonsLoading)
        return;
    setAddonsLoading(true);
    setAddonsError(QString());
    m_addonRows.clear();
    emit addonRowsChanged();
    QtConcurrent::run([this]() {
        const QString err = call0(g.addonsFetch);
        QMetaObject::invokeMethod(this, "finishAddons", Qt::QueuedConnection,
                                  Q_ARG(QString, err));
    });
}

void DaemonController::finishAddons(const QString &err)
{
    if (err.isEmpty())
        copyAddonRows();
    setAddonsError(err);
    setAddonsLoading(false);
}

void DaemonController::refreshAddons()
{
    if (g.addonsRefresh)
        takeCString(g.addonsRefresh());
    copyAddonRows();
}

void DaemonController::setBenchRunning(bool v)
{
    if (m_benchRunning == v)
        return;
    m_benchRunning = v;
    emit benchRunningChanged();
}

void DaemonController::setBenchStatus(const QString &v)
{
    if (m_benchStatus == v)
        return;
    m_benchStatus = v;
    emit benchStatusChanged();
}

QVariantMap benchRow(const QJsonObject &r)
{
    QVariantMap row;
    row.insert(QStringLiteral("w"), r.value(QStringLiteral("w")).toInt());
    row.insert(QStringLiteral("h"), r.value(QStringLiteral("h")).toInt());
    row.insert(QStringLiteral("mode"), r.value(QStringLiteral("mode")).toString());
    row.insert(QStringLiteral("load"), r.value(QStringLiteral("load")).toDouble());
    row.insert(QStringLiteral("fps"), r.value(QStringLiteral("fps")).toDouble());
    row.insert(QStringLiteral("elapsed"), r.value(QStringLiteral("elapsed")).toDouble());
    row.insert(QStringLiteral("held"), r.value(QStringLiteral("held")).toBool());
    row.insert(QStringLiteral("late"), r.value(QStringLiteral("late")).toBool());
    row.insert(QStringLiteral("error"), r.value(QStringLiteral("error")).toString());
    row.insert(QStringLiteral("reason"), r.value(QStringLiteral("reason")).toString());
    return row;
}

void DaemonController::applyBenchJson(const QString &raw)
{
    QString encoder;
    bool can = false;
    QVariantList rows;
    QVariantMap live;
    const QJsonDocument doc = QJsonDocument::fromJson(raw.toUtf8());
    if (doc.isObject()) {
        const QJsonObject o = doc.object();
        encoder = o.value(QStringLiteral("encoder")).toString();
        can = o.value(QStringLiteral("canSelect")).toBool();
        if (o.value(QStringLiteral("live")).isObject())
            live = benchRow(o.value(QStringLiteral("live")).toObject());
        const QJsonArray arr = o.value(QStringLiteral("rows")).toArray();
        for (int i = 0; i < arr.size(); ++i)
            rows.append(benchRow(arr.at(i).toObject()));
    }
    if (m_benchLive != live) {
        m_benchLive = live;
        emit benchLiveChanged();
    }
    if (m_benchEncoder != encoder) {
        m_benchEncoder = encoder;
        emit benchEncoderChanged();
    }
    if (m_benchCanSelect != can) {
        m_benchCanSelect = can;
        emit benchCanSelectChanged();
    }
    m_benchRows = rows;
    emit benchRowsChanged();
}

QString DaemonController::startBench(const QString &encoder)
{
    if (!g.benchStart)
        return g.error.isEmpty() ? QStringLiteral("libcargadgetdcontrol.so is missing") : g.error;
    const QByteArray enc = encoder.toUtf8();
    const QString err = takeCString(g.benchStart(enc.constData()));
    refreshBench();
    return err;
}

void DaemonController::refreshBench()
{
    setBenchRunning(callBool(g.benchRunning));
    setBenchStatus(g.benchStatus ? takeCString(g.benchStatus()) : QString());
    const QString raw = g.benchResult ? takeCString(g.benchResult()) : QString();
    if (raw == m_benchRaw)
        return;
    m_benchRaw = raw;
    applyBenchJson(raw);
}

void DaemonController::watchBench()
{
    if (g.benchWatch)
        g.benchWatch();
    refreshBench();
}

void DaemonController::clearBench()
{
    if (g.benchClear)
        g.benchClear();
    refreshBench();
}
