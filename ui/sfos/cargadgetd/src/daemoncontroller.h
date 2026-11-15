#ifndef DAEMONCONTROLLER_H
#define DAEMONCONTROLLER_H

#include <QObject>
#include <QString>
#include <QTimer>
#include <functional>

// Controls on-device USB gadget (setuid cargadgetd-gadget) + MD daemon.
// Daemon YAML keys are Q_PROPERTY; UI-only paths stay in QSettings.
class DaemonController : public QObject
{
    Q_OBJECT
    Q_PROPERTY(bool busy READ busy NOTIFY busyChanged)
    Q_PROPERTY(bool running READ running NOTIFY runningChanged)
    Q_PROPERTY(bool gadgetRunning READ gadgetRunning NOTIFY gadgetRunningChanged)
    Q_PROPERTY(bool ffsReady READ ffsReady NOTIFY ffsReadyChanged)
    Q_PROPERTY(bool aoaRunning READ aoaRunning NOTIFY aoaRunningChanged)
    Q_PROPERTY(QString statusText READ statusText NOTIFY statusTextChanged)
    Q_PROPERTY(QString logText READ logText NOTIFY logTextChanged)

    Q_PROPERTY(QString binaryPath READ binaryPath WRITE setBinaryPath NOTIFY settingsChanged)
    Q_PROPERTY(QString gadgetPath READ gadgetPath WRITE setGadgetPath NOTIFY settingsChanged)
    Q_PROPERTY(QString logPath READ logPath WRITE setLogPath NOTIFY settingsChanged)
    Q_PROPERTY(QString logArchives READ logArchives NOTIFY logTextChanged)

    Q_PROPERTY(QString listenUrl READ listenUrl WRITE setListenUrl NOTIFY settingsChanged)
    Q_PROPERTY(QString controlUrl READ controlUrl WRITE setControlUrl NOTIFY settingsChanged)
    Q_PROPERTY(QString certsDir READ certsDir WRITE setCertsDir NOTIFY settingsChanged)
    Q_PROPERTY(bool certsMissing READ certsMissing NOTIFY certsMissingChanged)
    Q_PROPERTY(QString certProbeText READ certProbeText NOTIFY certProbeTextChanged)

    Q_PROPERTY(QString encoder READ encoder WRITE setEncoder NOTIFY settingsChanged)
    Q_PROPERTY(QString videoSize READ videoSize WRITE setVideoSize NOTIFY settingsChanged)
    Q_PROPERTY(int videoConfigIndex READ videoConfigIndex WRITE setVideoConfigIndex NOTIFY settingsChanged)
    Q_PROPERTY(int frameMs READ frameMs WRITE setFrameMs NOTIFY settingsChanged)
    Q_PROPERTY(int fps READ fps NOTIFY settingsChanged)
    Q_PROPERTY(bool videoLive READ videoLive WRITE setVideoLive NOTIFY settingsChanged)

    Q_PROPERTY(bool audioEnabled READ audioEnabled WRITE setAudioEnabled NOTIFY settingsChanged)
    Q_PROPERTY(bool audioForcePrime READ audioForcePrime WRITE setAudioForcePrime NOTIFY settingsChanged)
    Q_PROPERTY(int audioTickMs READ audioTickMs WRITE setAudioTickMs NOTIFY settingsChanged)
    Q_PROPERTY(int audioChannels READ audioChannels WRITE setAudioChannels NOTIFY settingsChanged)
    Q_PROPERTY(int audioConfigIndex READ audioConfigIndex WRITE setAudioConfigIndex NOTIFY settingsChanged)
    Q_PROPERTY(QString audioType READ audioType WRITE setAudioType NOTIFY settingsChanged)

    Q_PROPERTY(int usbMediaGapMs READ usbMediaGapMs WRITE setUsbMediaGapMs NOTIFY settingsChanged)
    Q_PROPERTY(QString usbPcmDump READ usbPcmDump WRITE setUsbPcmDump NOTIFY settingsChanged)
    Q_PROPERTY(QString usbFfsDiag READ usbFfsDiag WRITE setUsbFfsDiag NOTIFY settingsChanged)

    Q_PROPERTY(QString gadgetConfigfs READ gadgetConfigfs WRITE setGadgetConfigfs NOTIFY settingsChanged)
    Q_PROPERTY(QString gadgetBin READ gadgetBin WRITE setGadgetBin NOTIFY settingsChanged)

    Q_PROPERTY(QString shellId READ shellId WRITE setShellId NOTIFY settingsChanged)
    Q_PROPERTY(bool appsStrict READ appsStrict WRITE setAppsStrict NOTIFY settingsChanged)
    Q_PROPERTY(QString appsUnhandledLog READ appsUnhandledLog WRITE setAppsUnhandledLog NOTIFY settingsChanged)

    Q_PROPERTY(bool musicLoopback READ musicLoopback WRITE setMusicLoopback NOTIFY settingsChanged)
    Q_PROPERTY(QString musicSink READ musicSink WRITE setMusicSink NOTIFY settingsChanged)

public:
    explicit DaemonController(QObject *parent = nullptr);
    Q_INVOKABLE void reloadYaml();

    bool busy() const { return m_busy; }
    bool running() const { return m_running; }
    bool gadgetRunning() const { return m_gadgetRunning; }
    bool ffsReady() const { return m_ffsReady; }
    bool aoaRunning() const { return m_aoaRunning; }
    QString statusText() const { return m_statusText; }
    QString logText() const { return m_logText; }

    QString binaryPath() const { return m_binaryPath; }
    void setBinaryPath(const QString &v);

    QString gadgetPath() const { return m_gadgetPath; }
    void setGadgetPath(const QString &v);

    QString logPath() const { return m_logPath; }
    QString logArchives() const { return m_logArchives; }
    void setLogPath(const QString &v);

    QString listenUrl() const { return m_listenUrl; }
    void setListenUrl(const QString &v);

    QString controlUrl() const { return m_controlUrl; }
    void setControlUrl(const QString &v);

    QString certsDir() const { return m_certsDir; }
    void setCertsDir(const QString &v);
    bool certsMissing() const { return m_certsMissing; }
    QString certProbeText() const { return m_certProbeText; }

    QString encoder() const { return m_encoder; }
    void setEncoder(const QString &v);

    QString videoSize() const { return m_videoSize; }
    void setVideoSize(const QString &v);

    int videoConfigIndex() const { return m_videoConfigIndex; }
    void setVideoConfigIndex(int v);

    int frameMs() const { return m_frameMs; }
    void setFrameMs(int v);
    int fps() const;

    bool videoLive() const { return m_videoLive; }
    void setVideoLive(bool v);

    bool audioEnabled() const { return m_audioEnabled; }
    void setAudioEnabled(bool v);

    bool audioForcePrime() const { return m_audioForcePrime; }
    void setAudioForcePrime(bool v);

    int audioTickMs() const { return m_audioTickMs; }
    void setAudioTickMs(int v);

    int audioChannels() const { return m_audioChannels; }
    void setAudioChannels(int v);

    int audioConfigIndex() const { return m_audioConfigIndex; }
    void setAudioConfigIndex(int v);

    QString audioType() const { return m_audioType; }
    void setAudioType(const QString &v);

    int usbMediaGapMs() const { return m_usbMediaGapMs; }
    void setUsbMediaGapMs(int v);

    QString usbPcmDump() const { return m_usbPcmDump; }
    void setUsbPcmDump(const QString &v);

    QString usbFfsDiag() const { return m_usbFfsDiag; }
    void setUsbFfsDiag(const QString &v);

    QString gadgetConfigfs() const { return m_gadgetConfigfs; }
    void setGadgetConfigfs(const QString &v);

    QString gadgetBin() const { return m_gadgetBin; }
    void setGadgetBin(const QString &v);

    QString shellId() const { return m_shellId; }
    void setShellId(const QString &v);

    bool appsStrict() const { return m_appsStrict; }
    void setAppsStrict(bool v);

    QString appsUnhandledLog() const { return m_appsUnhandledLog; }
    void setAppsUnhandledLog(const QString &v);

    bool musicLoopback() const { return m_musicLoopback; }
    void setMusicLoopback(bool v);

    QString musicSink() const { return m_musicSink; }
    void setMusicSink(const QString &v);

    Q_INVOKABLE void refresh();
    Q_INVOKABLE void startAoa();
    Q_INVOKABLE void stopAoa();
    Q_INVOKABLE void restartAoa();
    Q_INVOKABLE void refreshLog();
    Q_INVOKABLE void refreshGadgetLog();
    Q_INVOKABLE void clearLog();
    Q_INVOKABLE void probeCerts();

signals:
    void busyChanged();
    void runningChanged();
    void gadgetRunningChanged();
    void ffsReadyChanged();
    void statusTextChanged();
    void logTextChanged();
    void settingsChanged();
    void certsMissingChanged();
    void certProbeTextChanged();
    void aoaRunningChanged();

private slots:
    void finishAsync(const QString &status);

private:
    void loadSettings();
    void saveSettings() const;
    void commit();
    void applyYamlText(const QString &text);
    QString mdListenArg() const;
    QString resolveCertsPath(const QString &v) const;
    QString probeCertsWork() const;
    void setBusy(bool v);
    void setRunning(bool v);
    void setGadgetRunning(bool v);
    void setFfsReady(bool v);
    void setAoaRunning(bool v);
    QString aoaServiceCtl(const QString &verb);
    void setStatusText(const QString &v);
    void runAsync(const QString &busyMsg, const std::function<QString()> &work);
    QString expandHome(const QString &path) const;
    QString daemonConfigPath() const;
    bool writeDaemonConfig() const;
    bool processMatches(const QString &pattern) const;
    void killMatchingUser(const QString &pattern);
    bool gadgetIsSetuid(const QString &bin) const;
    QString mdProcPattern() const;
    QString gadgetProcPattern() const;
    bool waitForFfs(int tries) const;
    QString rotateSharedLog() const;
    void appendLogBanner(const QString &banner) const;
    QString sharedLogPath() const;

    QString startGadgetWork();
    QString startGadgetWorkInner(bool clearLog);
    QString stopGadgetWork();
    QString startDaemonWork();
    QString stopDaemonWork();
    QString startStackWork();
    QString stopStackWork();

    QTimer m_poll;
    bool m_busy = false;
    bool m_running = false;
    bool m_gadgetRunning = false;
    bool m_ffsReady = false;
    bool m_aoaRunning = false;
    QString m_statusText;
    QString m_logText;
    QString m_logArchives;

    QString m_binaryPath;
    QString m_gadgetPath;
    QString m_logPath;

    QString m_listenUrl = QStringLiteral("ffs:///dev/usb-ffs/aa");
    QString m_controlUrl = QStringLiteral("tcp://127.0.0.1:9277");
    QString m_certsDir = QStringLiteral("/usr/lib/cargadgetd/plugins/reversegearhead.so");
    bool m_certsMissing = true;
    QString m_certProbeText;

    QString m_encoder = QStringLiteral("droidmedia");
    QString m_videoSize = QStringLiteral("auto");
    int m_videoConfigIndex = -1;
    int m_frameMs = 33;
    bool m_videoLive = true;

    bool m_audioEnabled = true;
    bool m_audioForcePrime = false;
    int m_audioTickMs = 0;
    int m_audioChannels = 0;
    int m_audioConfigIndex = -1;
    QString m_audioType = QStringLiteral("auto");

    int m_usbMediaGapMs = 0;
    QString m_usbPcmDump;
    QString m_usbFfsDiag;

    QString m_gadgetConfigfs = QStringLiteral("/sys/kernel/config/usb_gadget/aa");
    QString m_gadgetBin = QStringLiteral("/usr/share/cargadgetd/bin/cargadgetd-gadget");

    QString m_shellId = QStringLiteral("basic");
    bool m_appsStrict = false;
    QString m_appsUnhandledLog;

    bool m_musicLoopback = true;
    QString m_musicSink = QStringLiteral("sink.null");
};

#endif // DAEMONCONTROLLER_H
