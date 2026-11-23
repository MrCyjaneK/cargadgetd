#ifndef DAEMONCONTROLLER_H
#define DAEMONCONTROLLER_H

#include <QObject>
#include <QString>
#include <QTimer>
#include <QVariantList>
#include <functional>

class DaemonController : public QObject
{
    Q_OBJECT
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(bool running READ running NOTIFY runningChanged)
    Q_PROPERTY(bool ffsReady READ ffsReady NOTIFY ffsReadyChanged)
    Q_PROPERTY(bool aoaRunning READ aoaRunning NOTIFY aoaRunningChanged)
    Q_PROPERTY(QString statusText READ statusText NOTIFY statusTextChanged)
    Q_PROPERTY(QString logText READ logText NOTIFY logTextChanged)

    Q_PROPERTY(bool certsMissing READ certsMissing NOTIFY certsMissingChanged)
    Q_PROPERTY(QString certProbeText READ certProbeText NOTIFY certProbeTextChanged)
    Q_PROPERTY(QString certMessage READ certMessage NOTIFY certMessageChanged)
    Q_PROPERTY(bool onboardingDone READ onboardingDone NOTIFY onboardingDoneChanged)

    Q_PROPERTY(QVariantList addonRows READ addonRows NOTIFY addonRowsChanged)
    Q_PROPERTY(QString addonsError READ addonsError NOTIFY addonsErrorChanged)
    Q_PROPERTY(bool addonsLoading READ addonsLoading NOTIFY addonsLoadingChanged)

    Q_PROPERTY(bool benchRunning READ benchRunning NOTIFY benchRunningChanged)
    Q_PROPERTY(QString benchStatus READ benchStatus NOTIFY benchStatusChanged)
    Q_PROPERTY(QString benchEncoder READ benchEncoder NOTIFY benchEncoderChanged)
    Q_PROPERTY(bool benchCanSelect READ benchCanSelect NOTIFY benchCanSelectChanged)
    Q_PROPERTY(QVariantList benchRows READ benchRows NOTIFY benchRowsChanged)
    Q_PROPERTY(QVariantMap benchLive READ benchLive NOTIFY benchLiveChanged)
    Q_PROPERTY(QVariantList encoders READ encoders CONSTANT)
    Q_PROPERTY(QVariantList transports READ transports CONSTANT)

public:
    explicit DaemonController(QObject *parent = nullptr);

    bool busy() const { return m_busy; }
    bool running() const { return m_running; }
    bool ffsReady() const { return m_ffsReady; }
    bool aoaRunning() const { return m_aoaRunning; }
    QString statusText() const { return m_statusText; }
    QString logText() const { return m_logText; }

    bool certsMissing() const { return m_certsMissing; }
    QString certProbeText() const { return m_certProbeText; }
    QString certMessage() const { return m_certMessage; }
    bool onboardingDone() const { return m_onboardingDone; }

    QVariantList addonRows() const { return m_addonRows; }
    QString addonsError() const { return m_addonsError; }
    bool addonsLoading() const { return m_addonsLoading; }

    bool benchRunning() const { return m_benchRunning; }
    QString benchStatus() const { return m_benchStatus; }
    QString benchEncoder() const { return m_benchEncoder; }
    bool benchCanSelect() const { return m_benchCanSelect; }
    QVariantList benchRows() const { return m_benchRows; }
    QVariantMap benchLive() const { return m_benchLive; }
    QVariantList encoders() const { return m_encoders; }
    QVariantList transports() const { return m_transports; }

    Q_INVOKABLE void startAoa();
    Q_INVOKABLE void stopAoa();
    Q_INVOKABLE void restartAoa();
    Q_INVOKABLE void refreshLog();
    Q_INVOKABLE void clearLog();
    Q_INVOKABLE void probeCerts();
    Q_INVOKABLE void downloadCerts();
    Q_INVOKABLE void installApk(const QString &path);
    Q_INVOKABLE void completeOnboarding();
    Q_INVOKABLE bool hasAddons() const;
    Q_INVOKABLE QString apkStartPath() const;
    Q_INVOKABLE QVariantList listApkDir(const QString &path) const;
    Q_INVOKABLE void loadAddons();
    Q_INVOKABLE void refreshAddons();
    Q_INVOKABLE QString startBench(const QString &encoder);
    Q_INVOKABLE void refreshBench();
    Q_INVOKABLE void watchBench();
    Q_INVOKABLE void clearBench();

signals:
    void busyChanged();
    void runningChanged();
    void ffsReadyChanged();
    void aoaRunningChanged();
    void statusTextChanged();
    void logTextChanged();
    void certsMissingChanged();
    void certProbeTextChanged();
    void certMessageChanged();
    void onboardingDoneChanged();
    void certInstallFinished();
    void addonRowsChanged();
    void addonsErrorChanged();
    void addonsLoadingChanged();
    void benchRunningChanged();
    void benchStatusChanged();
    void benchEncoderChanged();
    void benchCanSelectChanged();
    void benchRowsChanged();
    void benchLiveChanged();

private slots:
    void finishAsync(const QString &status);
    void finishCertAsync(const QString &status);
    void finishAddons(const QString &err);

private:
    void refresh();
    void copyAddonRows();
    void runAsync(const QString &busyMsg, const std::function<QString()> &work);
    void runCertAsync(const QString &busyMsg, const std::function<QString()> &work);
    void setBusy(bool v);
    void setRunning(bool v);
    void setFfsReady(bool v);
    void setAoaRunning(bool v);
    void setStatusText(const QString &v);
    void setCertMessage(const QString &v);
    void setCertProbe(const QString &v);
    void setAddonsError(const QString &v);
    void setAddonsLoading(bool v);
    void setBenchRunning(bool v);
    void setBenchStatus(const QString &v);
    void applyBenchJson(const QString &raw);

    QTimer m_poll;
    bool m_busy = false;
    bool m_running = false;
    bool m_ffsReady = false;
    bool m_aoaRunning = false;
    QString m_statusText;
    QString m_logText;
    bool m_certsMissing = true;
    QString m_certProbeText;
    QString m_certMessage;
    bool m_onboardingDone = false;
    QVariantList m_addonRows;
    QString m_addonsError;
    bool m_addonsLoading = false;
    bool m_benchRunning = false;
    QString m_benchStatus;
    QString m_benchEncoder;
    bool m_benchCanSelect = false;
    QVariantList m_benchRows;
    QVariantMap m_benchLive;
    QString m_benchRaw;
    QVariantList m_encoders;
    QVariantList m_transports;
};

#endif
