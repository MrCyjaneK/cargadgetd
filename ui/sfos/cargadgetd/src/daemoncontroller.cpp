#include "daemoncontroller.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QMap>
#include <QMetaObject>
#include <QProcess>
#include <QRegExp>
#include <QSettings>
#include <QStringList>
#include <QtConcurrent>

#include <dlfcn.h>
#include <sys/stat.h>

namespace {
const char *kSettingsOrg = "cargadgetd";
const char *kSettingsApp = "ui";
const char *kPackagedMd = "/usr/share/cargadgetd/bin/cargadgetd-md";
const char *kPackagedGadget = "/usr/share/cargadgetd/bin/cargadgetd-gadget";
const char *kPackagedConfig = "/usr/share/cargadgetd/config.yaml";
const char *kFfsEp1 = "/dev/usb-ffs/aa/ep1";
const char *kFfsListen = "ffs:///dev/usb-ffs/aa";
const char *kDefaultCerts = "/usr/lib/cargadgetd/plugins/reversegearhead.so";

static int clampInt(int v, int lo, int hi)
{
    if (v < lo)
        return lo;
    if (v > hi)
        return hi;
    return v;
}

static QString stripComment(const QString &line)
{
    bool inQ = false;
    QChar q;
    for (int i = 0; i < line.size(); ++i) {
        const QChar c = line.at(i);
        if (!inQ && c == QLatin1Char('#'))
            return line.left(i);
        if (c == QLatin1Char('"') || c == QLatin1Char('\'')) {
            if (!inQ) {
                inQ = true;
                q = c;
            } else if (c == q && (i == 0 || line.at(i - 1) != QLatin1Char('\\'))) {
                inQ = false;
            }
        }
    }
    return line;
}

static QString yamlUnquote(QString v)
{
    v = v.trimmed();
    if (v.size() >= 2
        && ((v.startsWith(QLatin1Char('"')) && v.endsWith(QLatin1Char('"')))
            || (v.startsWith(QLatin1Char('\'')) && v.endsWith(QLatin1Char('\'')))))
        return v.mid(1, v.size() - 2);
    return v;
}

static QString yamlStr(const QString &s)
{
    QString t = s;
    t.replace(QLatin1Char('\\'), QLatin1String("\\\\"));
    t.replace(QLatin1Char('"'), QLatin1String("\\\""));
    return QLatin1Char('"') + t + QLatin1Char('"');
}

static QString yamlBool(bool v)
{
    return v ? QStringLiteral("true") : QStringLiteral("false");
}

static int lineIndent(const QString &line)
{
    int i = 0;
    while (i < line.size()
           && (line.at(i) == QLatin1Char(' ') || line.at(i) == QLatin1Char('\t')))
        ++i;
    return i;
}

static QString lineKey(const QString &line, int *indentOut)
{
    const QString stripped = stripComment(line);
    const int indent = lineIndent(stripped);
    if (indentOut)
        *indentOut = indent;
    const QString rest = stripped.mid(indent).trimmed();
    if (rest.isEmpty())
        return QString();
    const int c = rest.indexOf(QLatin1Char(':'));
    if (c <= 0)
        return QString();
    return rest.left(c).trimmed();
}

static QString lineValue(const QString &line)
{
    const QString stripped = stripComment(line);
    const int indent = lineIndent(stripped);
    const QString rest = stripped.mid(indent).trimmed();
    const int c = rest.indexOf(QLatin1Char(':'));
    if (c < 0)
        return QString();
    return yamlUnquote(rest.mid(c + 1));
}

using YamlDoc = QMap<QString, QMap<QString, QString>>;

static YamlDoc parseYamlSections(QString text)
{
    YamlDoc doc;
    text.replace(QLatin1String("\r\n"), QLatin1String("\n"));
    QString section;
    const QStringList lines = text.split(QLatin1Char('\n'));
    for (int i = 0; i < lines.size(); ++i) {
        int indent = 0;
        const QString key = lineKey(lines.at(i), &indent);
        if (key.isEmpty())
            continue;
        if (indent == 0) {
            section = key;
            continue;
        }
        if (section.isEmpty())
            continue;
        doc[section][key] = lineValue(lines.at(i));
    }
    return doc;
}

static bool yamlToBool(const QString &v, bool def)
{
    const QString s = v.trimmed().toLower();
    if (s == QLatin1String("true") || s == QLatin1String("yes") || s == QLatin1String("1")
        || s == QLatin1String("on"))
        return true;
    if (s == QLatin1String("false") || s == QLatin1String("no") || s == QLatin1String("0")
        || s == QLatin1String("off"))
        return false;
    return def;
}

static bool yamlToInt(const QString &v, int *out)
{
    bool ok = false;
    const int n = v.trimmed().toInt(&ok);
    if (!ok)
        return false;
    *out = n;
    return true;
}

static bool replaceYamlSectionKey(QString &text, const QString &section, const QString &key,
                                  const QString &value)
{
    text.replace(QLatin1String("\r\n"), QLatin1String("\n"));
    QStringList lines = text.split(QLatin1Char('\n'));
    bool in = false;
    int lastInSection = -1;
    for (int i = 0; i < lines.size(); ++i) {
        int indent = 0;
        const QString k = lineKey(lines.at(i), &indent);
        if (k.isEmpty()) {
            if (in)
                lastInSection = i;
            continue;
        }
        if (indent == 0) {
            if (in)
                break;
            if (k == section) {
                in = true;
                lastInSection = i;
            }
            continue;
        }
        if (in) {
            lastInSection = i;
            if (k == key) {
                const int ind = qMax(lineIndent(lines.at(i)), 2);
                lines[i] = QString(ind, QLatin1Char(' ')) + key + QLatin1String(": ") + value;
                text = lines.join(QLatin1Char('\n'));
                return true;
            }
        }
    }
    if (in) {
        lines.insert(lastInSection + 1, QStringLiteral("  ") + key + QLatin1String(": ") + value);
        text = lines.join(QLatin1Char('\n'));
        return true;
    }
    if (!text.isEmpty() && !text.endsWith(QLatin1Char('\n')))
        text += QLatin1Char('\n');
    text += section + QLatin1String(":\n  ") + key + QLatin1String(": ") + value + QLatin1Char('\n');
    return true;
}

static QString readFileUtf8(const QString &path)
{
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return QString();
    return QString::fromUtf8(f.readAll());
}
} // namespace

DaemonController::DaemonController(QObject *parent)
    : QObject(parent)
{
    loadSettings();
    connect(&m_poll, &QTimer::timeout, this, &DaemonController::refresh);
    m_poll.start(1500);
    refresh();
    refreshLog();
}

void DaemonController::applyYamlText(const QString &text)
{
    if (text.trimmed().isEmpty())
        return;
    const YamlDoc doc = parseYamlSections(text);
    const QMap<QString, QString> daemon = doc.value(QStringLiteral("daemon"));
    const QMap<QString, QString> video = doc.value(QStringLiteral("video"));
    const QMap<QString, QString> audio = doc.value(QStringLiteral("audio"));
    const QMap<QString, QString> usb = doc.value(QStringLiteral("usb"));
    const QMap<QString, QString> gadget = doc.value(QStringLiteral("gadget"));
    const QMap<QString, QString> apps = doc.value(QStringLiteral("apps"));
    const QMap<QString, QString> shell = doc.value(QStringLiteral("shell"));
    const QMap<QString, QString> music = doc.value(QStringLiteral("music"));

    if (daemon.contains(QStringLiteral("listen")))
        m_listenUrl = daemon.value(QStringLiteral("listen"));
    if (daemon.contains(QStringLiteral("control")))
        m_controlUrl = daemon.value(QStringLiteral("control"));
    if (daemon.contains(QStringLiteral("certs")))
        m_certsDir = resolveCertsPath(daemon.value(QStringLiteral("certs")));

    if (video.contains(QStringLiteral("encoder")))
        m_encoder = video.value(QStringLiteral("encoder"));
    if (video.contains(QStringLiteral("size"))) {
        const QString n = video.value(QStringLiteral("size")).trimmed();
        if (!n.isEmpty())
            m_videoSize = n;
    }
    int n = 0;
    if (video.contains(QStringLiteral("config_index"))
        && yamlToInt(video.value(QStringLiteral("config_index")), &n))
        m_videoConfigIndex = clampInt(n, -1, 32);
    if (video.contains(QStringLiteral("frame_ms"))
        && yamlToInt(video.value(QStringLiteral("frame_ms")), &n)) {
        if (n > 0)
            m_frameMs = clampInt(n, 16, 200);
    }
    if (video.contains(QStringLiteral("live")))
        m_videoLive = yamlToBool(video.value(QStringLiteral("live")), m_videoLive);

    if (audio.contains(QStringLiteral("enabled")))
        m_audioEnabled = yamlToBool(audio.value(QStringLiteral("enabled")), m_audioEnabled);
    if (audio.contains(QStringLiteral("force_prime")))
        m_audioForcePrime = yamlToBool(audio.value(QStringLiteral("force_prime")), m_audioForcePrime);
    if (audio.contains(QStringLiteral("tick_ms"))
        && yamlToInt(audio.value(QStringLiteral("tick_ms")), &n))
        m_audioTickMs = clampInt(n, 0, 1000);
    if (audio.contains(QStringLiteral("channels"))
        && yamlToInt(audio.value(QStringLiteral("channels")), &n)) {
        if (n == 0 || n == 1 || n == 2)
            m_audioChannels = n;
    }
    if (audio.contains(QStringLiteral("config_index"))
        && yamlToInt(audio.value(QStringLiteral("config_index")), &n))
        m_audioConfigIndex = clampInt(n, -1, 32);
    if (audio.contains(QStringLiteral("type"))) {
        const QString t = audio.value(QStringLiteral("type")).trimmed().toLower();
        if (t == QLatin1String("media") || t == QLatin1String("system") || t == QLatin1String("speech")
            || t == QLatin1String("auto"))
            m_audioType = t;
    }

    if (usb.contains(QStringLiteral("media_gap_ms"))
        && yamlToInt(usb.value(QStringLiteral("media_gap_ms")), &n))
        m_usbMediaGapMs = clampInt(n, 0, 100);
    if (usb.contains(QStringLiteral("pcm_dump")))
        m_usbPcmDump = usb.value(QStringLiteral("pcm_dump"));
    if (usb.contains(QStringLiteral("ffs_diag")))
        m_usbFfsDiag = usb.value(QStringLiteral("ffs_diag"));

    if (gadget.contains(QStringLiteral("path"))) {
        const QString p = gadget.value(QStringLiteral("path")).trimmed();
        if (!p.isEmpty())
            m_gadgetConfigfs = p;
    }
    if (gadget.contains(QStringLiteral("bin")))
        m_gadgetBin = gadget.value(QStringLiteral("bin"));

    if (shell.contains(QStringLiteral("id"))) {
        const QString id = shell.value(QStringLiteral("id")).trimmed();
        m_shellId = id.isEmpty() ? QStringLiteral("basic") : id;
    }

    if (apps.contains(QStringLiteral("strict")))
        m_appsStrict = yamlToBool(apps.value(QStringLiteral("strict")), m_appsStrict);
    if (apps.contains(QStringLiteral("unhandled_log")))
        m_appsUnhandledLog = apps.value(QStringLiteral("unhandled_log"));

    if (music.contains(QStringLiteral("loopback")))
        m_musicLoopback = yamlToBool(music.value(QStringLiteral("loopback")), m_musicLoopback);
    if (music.contains(QStringLiteral("sink")))
        m_musicSink = music.value(QStringLiteral("sink"));
}

void DaemonController::reloadYaml()
{
    loadSettings();
    emit settingsChanged();
}

void DaemonController::loadSettings()
{
    QSettings s(kSettingsOrg, kSettingsApp);
    const QString home = QDir::homePath();
    m_binaryPath = s.value(QStringLiteral("binaryPath"), QString::fromUtf8(kPackagedMd)).toString();
    if (m_binaryPath.contains(QLatin1String("linux-arm64"))
        && QFileInfo::exists(QString::fromUtf8(kPackagedMd))) {
        m_binaryPath = QString::fromUtf8(kPackagedMd);
    }
    m_gadgetPath = s.value(QStringLiteral("gadgetPath"), QString::fromUtf8(kPackagedGadget)).toString();
    if (m_gadgetPath.contains(QLatin1String("linux-arm64"))
        && QFileInfo::exists(QString::fromUtf8(kPackagedGadget))) {
        m_gadgetPath = QString::fromUtf8(kPackagedGadget);
    }
    m_logPath = s.value(QStringLiteral("logPath"),
                        home + QStringLiteral("/cargadgetd.log")).toString();
    if (m_logPath.endsWith(QStringLiteral("cargadgetd-usb.log")))
        m_logPath = home + QStringLiteral("/cargadgetd.log");

    // User YAML wins. /usr/share/cargadgetd/config.yaml is an optional overlay.
    applyYamlText(readFileUtf8(QString::fromUtf8(kPackagedConfig)));
    const QString userPath = daemonConfigPath();
    const bool haveUser = QFileInfo::exists(userPath);
    if (haveUser)
        applyYamlText(readFileUtf8(userPath));
    else {
        // Migrate pre-YAML QSettings knobs once if the user file is not there yet.
        if (s.contains(QStringLiteral("listenUrl")))
            m_listenUrl = s.value(QStringLiteral("listenUrl")).toString();
        if (s.contains(QStringLiteral("fps"))) {
            const int fps = s.value(QStringLiteral("fps")).toInt();
            if (fps >= 1) {
                int ms = (1000 + fps / 2) / fps;
                if (ms < 16)
                    ms = 16;
                m_frameMs = clampInt(ms, 16, 200);
            }
        }
        if (s.contains(QStringLiteral("noAudio")))
            m_audioEnabled = !s.value(QStringLiteral("noAudio")).toBool();
        if (s.contains(QStringLiteral("noLive")))
            m_videoLive = !s.value(QStringLiteral("noLive")).toBool();
        if (s.contains(QStringLiteral("audioForcePrime")))
            m_audioForcePrime = s.value(QStringLiteral("audioForcePrime")).toBool();
        if (s.contains(QStringLiteral("videoSize")))
            m_videoSize = s.value(QStringLiteral("videoSize")).toString();
    }

    if (m_listenUrl.trimmed().isEmpty())
        m_listenUrl = QString::fromUtf8(kFfsListen);
    if (m_videoSize.trimmed().isEmpty())
        m_videoSize = QStringLiteral("auto");
    if (m_audioType.trimmed().isEmpty())
        m_audioType = QStringLiteral("auto");
}

void DaemonController::saveSettings() const
{
    QSettings s(kSettingsOrg, kSettingsApp);
    s.setValue(QStringLiteral("binaryPath"), m_binaryPath);
    s.setValue(QStringLiteral("gadgetPath"), m_gadgetPath);
    s.setValue(QStringLiteral("logPath"), m_logPath);
    writeDaemonConfig();
}

void DaemonController::commit()
{
    saveSettings();
    emit settingsChanged();
}

QString DaemonController::mdListenArg() const
{
    const QString u = m_listenUrl.trimmed();
    if (u.isEmpty())
        return QString::fromUtf8(kFfsListen);
    return u;
}

QString DaemonController::resolveCertsPath(const QString &v) const
{
    const QString n = v.trimmed();
    if (n.isEmpty()
        || n.compare(QLatin1String("embedded"), Qt::CaseInsensitive) == 0
        || n.compare(QLatin1String("reversegearhead"), Qt::CaseInsensitive) == 0)
        return QString::fromUtf8(kDefaultCerts);
    return n;
}

void DaemonController::setBinaryPath(const QString &v)
{
    if (m_binaryPath == v)
        return;
    m_binaryPath = v;
    commit();
}

void DaemonController::setGadgetPath(const QString &v)
{
    if (m_gadgetPath == v)
        return;
    m_gadgetPath = v;
    commit();
}

void DaemonController::setLogPath(const QString &v)
{
    if (m_logPath == v)
        return;
    m_logPath = v;
    commit();
    refreshLog();
}

void DaemonController::setListenUrl(const QString &v)
{
    if (m_listenUrl == v)
        return;
    m_listenUrl = v;
    commit();
}

void DaemonController::setControlUrl(const QString &v)
{
    const QString n = v.trimmed();
    if (m_controlUrl == n)
        return;
    m_controlUrl = n;
    commit();
}

void DaemonController::setCertsDir(const QString &v)
{
    const QString n = resolveCertsPath(v);
    if (m_certsDir == n)
        return;
    m_certsDir = n;
    commit();
}

void DaemonController::setEncoder(const QString &v)
{
    QString n = v.trimmed().toLower();
    if (n == QLatin1String("software"))
        n = QStringLiteral("software");
    else
        n = QStringLiteral("droidmedia");
    if (m_encoder == n)
        return;
    m_encoder = n;
    commit();
}

void DaemonController::setVideoSize(const QString &v)
{
    QString n = v.trimmed();
    QRegExp wxh(QStringLiteral("^\\d+x\\d+$"));
    if (n.isEmpty() || n == QLatin1String("auto"))
        n = QStringLiteral("auto");
    else if (!wxh.exactMatch(n))
        n = QStringLiteral("auto");
    if (m_videoSize == n)
        return;
    m_videoSize = n;
    commit();
}

void DaemonController::setVideoConfigIndex(int v)
{
    v = clampInt(v, -1, 32);
    if (m_videoConfigIndex == v)
        return;
    m_videoConfigIndex = v;
    commit();
}

void DaemonController::setFrameMs(int v)
{
    v = clampInt(v, 16, 200);
    if (m_frameMs == v)
        return;
    m_frameMs = v;
    commit();
}

int DaemonController::fps() const
{
    if (m_frameMs <= 0)
        return 30;
    return (1000 + m_frameMs / 2) / m_frameMs;
}

void DaemonController::setVideoLive(bool v)
{
    if (m_videoLive == v)
        return;
    m_videoLive = v;
    commit();
}

void DaemonController::setAudioEnabled(bool v)
{
    if (m_audioEnabled == v)
        return;
    m_audioEnabled = v;
    commit();
}

void DaemonController::setAudioForcePrime(bool v)
{
    if (m_audioForcePrime == v)
        return;
    m_audioForcePrime = v;
    commit();
}

void DaemonController::setAudioTickMs(int v)
{
    v = clampInt(v, 0, 1000);
    if (m_audioTickMs == v)
        return;
    m_audioTickMs = v;
    commit();
}

void DaemonController::setAudioChannels(int v)
{
    if (v != 0 && v != 1 && v != 2)
        v = 0;
    if (m_audioChannels == v)
        return;
    m_audioChannels = v;
    commit();
}

void DaemonController::setAudioConfigIndex(int v)
{
    v = clampInt(v, -1, 32);
    if (m_audioConfigIndex == v)
        return;
    m_audioConfigIndex = v;
    commit();
}

void DaemonController::setAudioType(const QString &v)
{
    QString n = v.trimmed().toLower();
    if (n != QLatin1String("media") && n != QLatin1String("system") && n != QLatin1String("speech"))
        n = QStringLiteral("auto");
    if (m_audioType == n)
        return;
    m_audioType = n;
    commit();
}

void DaemonController::setUsbMediaGapMs(int v)
{
    v = clampInt(v, 0, 100);
    if (m_usbMediaGapMs == v)
        return;
    m_usbMediaGapMs = v;
    commit();
}

void DaemonController::setUsbPcmDump(const QString &v)
{
    const QString n = v.trimmed();
    if (m_usbPcmDump == n)
        return;
    m_usbPcmDump = n;
    commit();
}

void DaemonController::setUsbFfsDiag(const QString &v)
{
    const QString n = v.trimmed();
    if (m_usbFfsDiag == n)
        return;
    m_usbFfsDiag = n;
    commit();
}

void DaemonController::setGadgetConfigfs(const QString &v)
{
    QString n = v.trimmed();
    if (n.isEmpty())
        n = QStringLiteral("/sys/kernel/config/usb_gadget/aa");
    if (m_gadgetConfigfs == n)
        return;
    m_gadgetConfigfs = n;
    commit();
}

void DaemonController::setGadgetBin(const QString &v)
{
    const QString n = v.trimmed();
    if (m_gadgetBin == n)
        return;
    m_gadgetBin = n;
    commit();
}

void DaemonController::setShellId(const QString &v)
{
    QString n = v.trimmed();
    if (n.isEmpty())
        n = QStringLiteral("basic");
    if (m_shellId == n)
        return;
    m_shellId = n;
    commit();
}

void DaemonController::setAppsStrict(bool v)
{
    if (m_appsStrict == v)
        return;
    m_appsStrict = v;
    commit();
}

void DaemonController::setAppsUnhandledLog(const QString &v)
{
    const QString n = v.trimmed();
    if (m_appsUnhandledLog == n)
        return;
    m_appsUnhandledLog = n;
    commit();
}

void DaemonController::setMusicLoopback(bool v)
{
    if (m_musicLoopback == v)
        return;
    m_musicLoopback = v;
    commit();
}

void DaemonController::setMusicSink(const QString &v)
{
    const QString n = v.trimmed();
    if (m_musicSink == n)
        return;
    m_musicSink = n;
    commit();
}

QString DaemonController::expandHome(const QString &path) const
{
    if (path.startsWith(QLatin1String("~/")))
        return QDir::homePath() + path.mid(1);
    if (path == QLatin1String("~"))
        return QDir::homePath();
    return path;
}

QString DaemonController::daemonConfigPath() const
{
    return QDir::homePath() + QStringLiteral("/.config/cargadgetd/config.yaml");
}

bool DaemonController::writeDaemonConfig() const
{
    const QString path = daemonConfigPath();
    if (!QDir().mkpath(QFileInfo(path).absolutePath()))
        return false;

    QString text = readFileUtf8(path);
    if (text.trimmed().isEmpty())
        text = readFileUtf8(QString::fromUtf8(kPackagedConfig));
    if (text.trimmed().isEmpty()) {
        text = QStringLiteral(
            "daemon:\nvideo:\naudio:\nusb:\ngadget:\nshell:\napps:\nmusic:\n");
    }

    replaceYamlSectionKey(text, QStringLiteral("daemon"), QStringLiteral("listen"), yamlStr(m_listenUrl));
    replaceYamlSectionKey(text, QStringLiteral("daemon"), QStringLiteral("control"), yamlStr(m_controlUrl));
    replaceYamlSectionKey(text, QStringLiteral("daemon"), QStringLiteral("certs"), yamlStr(m_certsDir));

    replaceYamlSectionKey(text, QStringLiteral("video"), QStringLiteral("encoder"), yamlStr(m_encoder));
    replaceYamlSectionKey(text, QStringLiteral("video"), QStringLiteral("size"), yamlStr(m_videoSize));
    replaceYamlSectionKey(text, QStringLiteral("video"), QStringLiteral("config_index"),
                          QString::number(m_videoConfigIndex));
    replaceYamlSectionKey(text, QStringLiteral("video"), QStringLiteral("frame_ms"),
                          QString::number(m_frameMs));
    replaceYamlSectionKey(text, QStringLiteral("video"), QStringLiteral("live"), yamlBool(m_videoLive));

    replaceYamlSectionKey(text, QStringLiteral("audio"), QStringLiteral("enabled"), yamlBool(m_audioEnabled));
    replaceYamlSectionKey(text, QStringLiteral("audio"), QStringLiteral("force_prime"),
                          yamlBool(m_audioForcePrime));
    replaceYamlSectionKey(text, QStringLiteral("audio"), QStringLiteral("tick_ms"),
                          QString::number(m_audioTickMs));
    replaceYamlSectionKey(text, QStringLiteral("audio"), QStringLiteral("channels"),
                          QString::number(m_audioChannels));
    replaceYamlSectionKey(text, QStringLiteral("audio"), QStringLiteral("config_index"),
                          QString::number(m_audioConfigIndex));
    replaceYamlSectionKey(text, QStringLiteral("audio"), QStringLiteral("type"), yamlStr(m_audioType));

    replaceYamlSectionKey(text, QStringLiteral("usb"), QStringLiteral("media_gap_ms"),
                          QString::number(m_usbMediaGapMs));
    replaceYamlSectionKey(text, QStringLiteral("usb"), QStringLiteral("pcm_dump"), yamlStr(m_usbPcmDump));
    replaceYamlSectionKey(text, QStringLiteral("usb"), QStringLiteral("ffs_diag"), yamlStr(m_usbFfsDiag));

    replaceYamlSectionKey(text, QStringLiteral("gadget"), QStringLiteral("path"), yamlStr(m_gadgetConfigfs));
    replaceYamlSectionKey(text, QStringLiteral("gadget"), QStringLiteral("bin"), yamlStr(m_gadgetBin));

    replaceYamlSectionKey(text, QStringLiteral("shell"), QStringLiteral("id"), yamlStr(m_shellId));

    replaceYamlSectionKey(text, QStringLiteral("apps"), QStringLiteral("strict"), yamlBool(m_appsStrict));
    replaceYamlSectionKey(text, QStringLiteral("apps"), QStringLiteral("unhandled_log"), yamlStr(m_appsUnhandledLog));

    replaceYamlSectionKey(text, QStringLiteral("music"), QStringLiteral("loopback"), yamlBool(m_musicLoopback));
    replaceYamlSectionKey(text, QStringLiteral("music"), QStringLiteral("sink"), yamlStr(m_musicSink));

    QFile::remove(QDir::homePath() + QStringLiteral("/.config/cargadgetd/ui.yaml"));

    QFile out(path);
    if (!out.open(QIODevice::WriteOnly | QIODevice::Truncate | QIODevice::Text))
        return false;
    const QByteArray bytes = text.toUtf8();
    return out.write(bytes) == bytes.size();
}

QString DaemonController::mdProcPattern() const
{
    const QString base = QFileInfo(expandHome(m_binaryPath)).fileName();
    const QString name = base.isEmpty() ? QStringLiteral("cargadgetd-md") : base;
    return QStringLiteral("(^|/| )%1( |$)").arg(QRegExp::escape(name));
}

QString DaemonController::gadgetProcPattern() const
{
    const QString base = QFileInfo(expandHome(m_gadgetPath)).fileName();
    const QString name = base.isEmpty() ? QStringLiteral("cargadgetd-gadget") : base;
    return QStringLiteral("(^|/| )%1( |$)").arg(QRegExp::escape(name));
}

bool DaemonController::processMatches(const QString &pattern) const
{
    QProcess p;
    p.start(QStringLiteral("pgrep"), QStringList() << QStringLiteral("-f") << pattern);
    if (!p.waitForFinished(2000)) {
        p.kill();
        return false;
    }
    return p.exitCode() == 0 && !QString::fromUtf8(p.readAllStandardOutput()).trimmed().isEmpty();
}

void DaemonController::killMatchingUser(const QString &pattern)
{
    QProcess::execute(QStringLiteral("pkill"), QStringList() << QStringLiteral("-f") << pattern);
    for (int i = 0; i < 50; ++i) {
        if (!processMatches(pattern))
            break;
        QProcess::execute(QStringLiteral("sleep"), QStringList() << QStringLiteral("0.1"));
    }
    if (processMatches(pattern)) {
        QProcess::execute(QStringLiteral("pkill"),
                          QStringList() << QStringLiteral("-9") << QStringLiteral("-f") << pattern);
        QProcess::execute(QStringLiteral("sleep"), QStringList() << QStringLiteral("0.2"));
    }
}

bool DaemonController::gadgetIsSetuid(const QString &bin) const
{
    struct stat st;
    if (stat(bin.toLocal8Bit().constData(), &st) != 0)
        return false;
    return (st.st_mode & S_ISUID) != 0 && st.st_uid == 0;
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

void DaemonController::setGadgetRunning(bool v)
{
    if (m_gadgetRunning == v)
        return;
    m_gadgetRunning = v;
    emit gadgetRunningChanged();
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

bool DaemonController::waitForFfs(int tries) const
{
    for (int i = 0; i < tries; ++i) {
        if (QFileInfo::exists(QString::fromUtf8(kFfsEp1)))
            return true;
        QProcess::execute(QStringLiteral("sleep"), QStringList() << QStringLiteral("0.25"));
    }
    return QFileInfo::exists(QString::fromUtf8(kFfsEp1));
}

QString DaemonController::sharedLogPath() const
{
    return expandHome(m_logPath);
}

QString DaemonController::rotateSharedLog() const
{
    const QString path = sharedLogPath();
    const QFileInfo fi(path);
    QDir().mkpath(fi.absolutePath());
    QString archived;
    if (fi.exists() && fi.size() > 0) {
        const QString ts = QDateTime::currentDateTime().toString(QStringLiteral("yyyyMMdd-HHmmss"));
        QString stem = path;
        if (stem.endsWith(QStringLiteral(".log")))
            stem.chop(4);
        QString dest = stem + QLatin1Char('-') + ts + QStringLiteral(".log");
        for (int n = 1; QFileInfo::exists(dest) && n < 100; ++n)
            dest = stem + QLatin1Char('-') + ts + QLatin1Char('-') + QString::number(n) + QStringLiteral(".log");
        if (QFile::rename(path, dest))
            archived = dest;
    }
    QFile fresh(path);
    if (fresh.open(QIODevice::WriteOnly | QIODevice::Append))
        fresh.close();

    const QString prefix = fi.completeBaseName() + QLatin1Char('-');
    const QFileInfoList old = fi.dir().entryInfoList(QStringList() << (prefix + QStringLiteral("[0-9]*.log")),
                                                     QDir::Files, QDir::Time);
    const int keep = 30;
    for (int i = keep; i < old.size(); ++i)
        QFile::remove(old.at(i).absoluteFilePath());
    return archived;
}

void DaemonController::appendLogBanner(const QString &banner) const
{
    QFile f(sharedLogPath());
    if (!f.open(QIODevice::WriteOnly | QIODevice::Append | QIODevice::Text))
        return;
    f.write(banner.toUtf8());
    f.write("\n");
    f.close();
}

void DaemonController::refresh()
{
    setRunning(processMatches(mdProcPattern()));
    setGadgetRunning(processMatches(gadgetProcPattern()));
    setFfsReady(QFileInfo::exists(QString::fromUtf8(kFfsEp1)));
    setAoaRunning(processMatches(QStringLiteral("usb-aoa-launch")));

    const QString so = resolveCertsPath(m_certsDir);
    const bool missing = !QFileInfo::exists(so);
    if (m_certsMissing != missing) {
        m_certsMissing = missing;
        emit certsMissingChanged();
    }
    if (missing) {
        const QString msg = QStringLiteral("reversegearhead.so is missing (%1)").arg(so);
        if (m_certProbeText != msg) {
            m_certProbeText = msg;
            emit certProbeTextChanged();
        }
    } else if (m_certProbeText.contains(QLatin1String("is missing"))) {
        m_certProbeText.clear();
        emit certProbeTextChanged();
    }

    if (m_busy)
        return;
}

QString DaemonController::aoaServiceCtl(const QString &verb)
{
    const QString bin = expandHome(m_gadgetPath);
    if (!QFileInfo::exists(bin))
        return QStringLiteral("Gadget binary missing: %1").arg(bin);
    if (!gadgetIsSetuid(bin)) {
        return QStringLiteral("Gadget needs setuid root (RPM install). Missing setuid on %1")
            .arg(bin);
    }
    QProcess p;
    p.start(bin, QStringList() << QStringLiteral("-aoa-service") << verb);
    if (!p.waitForFinished(20000)) {
        p.kill();
        return QStringLiteral("Handshake %1 timed out").arg(verb);
    }
    const QString err = QString::fromUtf8(p.readAllStandardError()).trimmed();
    const QString out = QString::fromUtf8(p.readAllStandardOutput()).trimmed();
    if (p.exitCode() != 0) {
        return QStringLiteral("Handshake %1 failed: %2")
            .arg(verb, err.isEmpty() ? QStringLiteral("exit %1").arg(p.exitCode()) : err);
    }
    Q_UNUSED(out);
    if (verb == QLatin1String("stop"))
        return QStringLiteral("Handshake service stopped");
    if (verb == QLatin1String("restart"))
        return QStringLiteral("Handshake service restarted");
    return QStringLiteral("Handshake service started");
}

void DaemonController::startAoa()
{
    runAsync(QStringLiteral("Starting handshake service…"), [this]() { return aoaServiceCtl(QStringLiteral("start")); });
}

void DaemonController::stopAoa()
{
    runAsync(QStringLiteral("Stopping handshake service…"), [this]() { return aoaServiceCtl(QStringLiteral("stop")); });
}

void DaemonController::restartAoa()
{
    runAsync(QStringLiteral("Restarting handshake service…"), [this]() { return aoaServiceCtl(QStringLiteral("restart")); });
}

QString DaemonController::startGadgetWork()
{
    return startGadgetWorkInner(true);
}

QString DaemonController::startGadgetWorkInner(bool clearLog)
{
    const QString bin = expandHome(m_gadgetPath);
    if (!QFileInfo::exists(bin))
        return QStringLiteral("Gadget binary missing: %1").arg(bin);
    if (!QFileInfo(bin).isExecutable())
        return QStringLiteral("Gadget binary not executable: %1").arg(bin);
    if (!gadgetIsSetuid(bin)) {
        return QStringLiteral("Gadget needs setuid root (RPM install). Missing setuid on %1")
            .arg(bin);
    }
    if (processMatches(gadgetProcPattern()) && QFileInfo::exists(QString::fromUtf8(kFfsEp1)))
        return QStringLiteral("Gadget already up");

    // Restart agent if FFS nodes are gone (stale process).
    {
        QProcess stop;
        stop.start(bin, QStringList() << QStringLiteral("-stop"));
        if (!stop.waitForFinished(30000))
            stop.kill();
    }

    const QString log = sharedLogPath();
    if (clearLog) {
        const QString archived = rotateSharedLog();
        if (!archived.isEmpty())
            appendLogBanner(QStringLiteral("kept previous log as %1").arg(archived));
    }
    appendLogBanner(QStringLiteral("=== gadget start ==="));
    const QString cmd = QStringLiteral("nohup \"%1\" >>\"%2\" 2>&1 & echo $!")
                            .arg(bin, log);

    QProcess p;
    p.start(QStringLiteral("/bin/bash"), QStringList() << QStringLiteral("-c") << cmd);
    if (!p.waitForFinished(5000)) {
        p.kill();
        return QStringLiteral("Gadget start timed out");
    }
    const QString out = QString::fromUtf8(p.readAllStandardOutput() + p.readAllStandardError()).trimmed();
    const bool ok = waitForFfs(40);
    if (ok)
        return QStringLiteral("Gadget up · FFS ready (pid %1)")
            .arg(out.isEmpty() ? QStringLiteral("?") : out);
    return QStringLiteral("Gadget start: no /dev/usb-ffs/aa/ep1 — see log");
}

QString DaemonController::stopGadgetWork()
{
    // MD must die before tearing down FFS or it wedges mid-write.
    killMatchingUser(mdProcPattern());

    const QString bin = expandHome(m_gadgetPath);
    if (QFileInfo::exists(bin) && gadgetIsSetuid(bin)) {
        QProcess stop;
        stop.start(bin, QStringList() << QStringLiteral("-stop"));
        if (!stop.waitForFinished(30000)) {
            stop.kill();
            return QStringLiteral("Gadget -stop timed out");
        }
        if (stop.exitCode() != 0) {
            const QString err = QString::fromUtf8(stop.readAllStandardError()).trimmed();
            return QStringLiteral("Gadget -stop failed: %1")
                .arg(err.isEmpty() ? QStringLiteral("exit %1").arg(stop.exitCode()) : err);
        }
    } else {
        // Best-effort for non-packaged paths (won't kill root agent).
        killMatchingUser(gadgetProcPattern());
        if (processMatches(gadgetProcPattern()))
            return QStringLiteral("Gadget still running (needs setuid -stop)");
    }
    return QStringLiteral("Gadget stopped");
}

QString DaemonController::startDaemonWork()
{
    const QString bin = expandHome(m_binaryPath);
    if (!QFileInfo::exists(bin))
        return QStringLiteral("MD binary missing: %1").arg(bin);
    if (!QFileInfo(bin).isExecutable())
        return QStringLiteral("MD binary not executable: %1").arg(bin);
    if (!QFileInfo::exists(QString::fromUtf8(kFfsEp1)))
        return QStringLiteral("FFS not ready — start USB gadget first");

    const QString so = resolveCertsPath(m_certsDir);
    if (!QFileInfo::exists(so))
        return QStringLiteral("reversegearhead.so is missing (%1)").arg(so);

    killMatchingUser(mdProcPattern());

    const QString log = sharedLogPath();
    QDir().mkpath(QFileInfo(log).absolutePath());
    appendLogBanner(QStringLiteral("=== MD start ==="));

    if (!writeDaemonConfig())
        return QStringLiteral("Could not write %1").arg(daemonConfigPath());

    const QString cmd = QStringLiteral("nohup \"%1\" -listen \"%2\" >>\"%3\" 2>&1 & echo $!")
                            .arg(bin, mdListenArg(), log);

    QProcess p;
    p.setWorkingDirectory(QFileInfo(bin).absolutePath());
    p.start(QStringLiteral("/bin/bash"), QStringList() << QStringLiteral("-c") << cmd);
    if (!p.waitForFinished(5000)) {
        p.kill();
        return QStringLiteral("MD start timed out");
    }
    const QString out = QString::fromUtf8(p.readAllStandardOutput() + p.readAllStandardError()).trimmed();
    QProcess::execute(QStringLiteral("sleep"), QStringList() << QStringLiteral("0.5"));
    if (processMatches(mdProcPattern()))
        return QStringLiteral("MD started (pid %1, %2 fps)")
            .arg(out.isEmpty() ? QStringLiteral("?") : out)
            .arg(fps());
    return QStringLiteral("MD start failed — see log");
}

QString DaemonController::stopDaemonWork()
{
    killMatchingUser(mdProcPattern());
    return QStringLiteral("MD stopped");
}

QString DaemonController::startStackWork()
{
    const QString archived = rotateSharedLog();
    if (!archived.isEmpty())
        appendLogBanner(QStringLiteral("kept previous log as %1").arg(archived));
    appendLogBanner(QStringLiteral("=== stack start ==="));
    const QString g = startGadgetWorkInner(false);
    if (!QFileInfo::exists(QString::fromUtf8(kFfsEp1)))
        return g;
    const QString m = startDaemonWork();
    if (processMatches(mdProcPattern()) && QFileInfo::exists(QString::fromUtf8(kFfsEp1)))
        return QStringLiteral("Stack up · %1 fps").arg(fps());
    return m;
}

QString DaemonController::stopStackWork()
{
    killMatchingUser(mdProcPattern());
    const QString g = stopGadgetWork();
    if (g != QStringLiteral("Gadget stopped"))
        return g;
    return QStringLiteral("Stack stopped");
}

void DaemonController::refreshLog()
{
    const QString path = sharedLogPath();
    QFile f(path);
    const QFileInfo fi(path);
    const QString prefix = fi.completeBaseName() + QLatin1Char('-');
    const QFileInfoList old = fi.dir().entryInfoList(QStringList() << (prefix + QStringLiteral("[0-9]*.log")),
                                                     QDir::Files, QDir::Time);
    QStringList names;
    const int show = qMin(old.size(), 8);
    for (int i = 0; i < show; ++i)
        names << old.at(i).fileName();
    const QString archives = names.join(QStringLiteral("\n"));
    if (m_logArchives != archives) {
        m_logArchives = archives;
    }
    if (!f.exists()) {
        m_logText = QStringLiteral("(no log yet: %1)").arg(path);
        emit logTextChanged();
        return;
    }
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text)) {
        m_logText = QStringLiteral("(cannot read %1: %2)").arg(path, f.errorString());
        emit logTextChanged();
        return;
    }
    const qint64 size = f.size();
    const qint64 maxTail = 64 * 1024;
    if (size > maxTail)
        f.seek(size - maxTail);
    QByteArray data = f.readAll();
    if (size > maxTail) {
        const int nl = data.indexOf('\n');
        if (nl >= 0)
            data = data.mid(nl + 1);
    }
    m_logText = QString::fromUtf8(data);
    if (m_logText.isEmpty())
        m_logText = QStringLiteral("(log empty)");
    emit logTextChanged();
}

void DaemonController::refreshGadgetLog()
{
    // Same shared log as MD (kept for older QML menu entries).
    refreshLog();
}

void DaemonController::clearLog()
{
    rotateSharedLog();
    refreshLog();
}

void DaemonController::probeCerts()
{
    const QString text = probeCertsWork();
    if (m_certProbeText == text)
        return;
    m_certProbeText = text;
    emit certProbeTextChanged();
}

QString DaemonController::probeCertsWork() const
{
    const QString so = resolveCertsPath(m_certsDir);
    if (!QFileInfo::exists(so))
        return QStringLiteral("reversegearhead.so is missing (%1)").arg(so);

    void *h = dlopen(qPrintable(so), RTLD_NOW | RTLD_GLOBAL);
    if (!h) {
        const char *err = dlerror();
        return QStringLiteral("reversegearhead.so failed to load (%1): %2")
            .arg(so, err ? QString::fromLocal8Bit(err) : QStringLiteral("dlopen failed"));
    }
    dlerror();
    char *(*probe)() = reinterpret_cast<char *(*)()>(dlsym(h, "reversegearhead_probe"));
    const char *err = dlerror();
    if (!probe) {
        return QStringLiteral("reversegearhead.so failed to load (%1): %2")
            .arg(so, err ? QString::fromLocal8Bit(err) : QStringLiteral("missing reversegearhead_probe"));
    }
    const char *text = probe();
    if (!text || !text[0])
        return QStringLiteral("(reversegearhead_probe returned empty)");
    return QString::fromUtf8(text);
}
