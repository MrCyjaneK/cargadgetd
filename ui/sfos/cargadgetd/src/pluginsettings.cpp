#include "pluginsettings.h"

#include <algorithm>
#include <cstring>
#include <dlfcn.h>

#include <QDir>
#include <QFileInfo>
#include <QVariantMap>

namespace {

const char *kPluginDir = "/usr/lib/cargadgetd/plugins";

struct Found {
    QString path;
    QString id;
    QString prefix;
    int rank;
};

bool idOk(const QString &id)
{
    if (id.isEmpty())
        return false;
    const QChar c0 = id.at(0);
    if (!(c0.isLetter() || c0 == QLatin1Char('_')))
        return false;
    for (int i = 1; i < id.size(); ++i) {
        const QChar c = id.at(i);
        if (!(c.isLetterOrNumber() || c == QLatin1Char('_')))
            return false;
    }
    return true;
}

QVector<Found> listPlugins()
{
    QVector<Found> out;
    const QFileInfoList infos = QDir(QString::fromUtf8(kPluginDir))
                                    .entryInfoList(QDir::Files, QDir::Name);
    for (const QFileInfo &fi : infos) {
        const QString name = fi.fileName();
        Found f;
        f.path = fi.absoluteFilePath();
        if (name.startsWith(QLatin1String("libcargadgetd_app_")) && name.endsWith(QLatin1String(".so"))) {
            f.id = name.mid(int(strlen("libcargadgetd_app_")));
            f.id.chop(3);
            f.prefix = QStringLiteral("cargadgetd_app_") + f.id + QLatin1Char('_');
            f.rank = f.id == QLatin1String("settings") ? 0 : 1;
        } else if (name.startsWith(QLatin1String("libcargadgetd_shell_")) && name.endsWith(QLatin1String(".so"))) {
            f.id = name.mid(int(strlen("libcargadgetd_shell_")));
            f.id.chop(3);
            f.prefix = QStringLiteral("cargadgetd_shell_") + f.id + QLatin1Char('_');
            f.rank = 2;
        } else {
            continue;
        }
        if (!idOk(f.id))
            continue;
        out.append(f);
    }
    std::sort(out.begin(), out.end(), [](const Found &a, const Found &b) {
        if (a.rank != b.rank)
            return a.rank < b.rank;
        return a.id < b.id;
    });
    return out;
}

template <typename T>
T sym(void *lib, const QString &name)
{
    return reinterpret_cast<T>(dlsym(lib, name.toUtf8().constData()));
}

} // namespace

PluginSettings::PluginSettings(QObject *parent)
    : QObject(parent)
{
}

PluginSettings::~PluginSettings()
{
    // Go c-shared runtimes are not safe to dlclose.
}

void PluginSettings::reload()
{
    if (!m_scanned)
        scan();
    rebuild();
}

void PluginSettings::scan()
{
    m_scanned = true;
    const QVector<Found> found = listPlugins();
    for (const Found &f : found) {
        void *lib = dlopen(f.path.toUtf8().constData(), RTLD_NOW | RTLD_GLOBAL);
        if (!lib)
            continue;
        Api api;
        api.lib = lib;
        const QString p = f.prefix;
        api.count = sym<int (*)()>(lib, p + QStringLiteral("settings_count"));
        api.label = sym<const char *(*)(int)>(lib, p + QStringLiteral("settings_label"));
        api.type = sym<int (*)(int)>(lib, p + QStringLiteral("settings_type"));
        api.enums = sym<const char *(*)(int)>(lib, p + QStringLiteral("settings_enum_values"));
        api.getBool = sym<int (*)(int)>(lib, p + QStringLiteral("settings_get_bool"));
        api.getEnum = sym<const char *(*)(int)>(lib, p + QStringLiteral("settings_get_enum"));
        api.getInt = sym<int (*)(int)>(lib, p + QStringLiteral("settings_get_int"));
        api.getString = sym<const char *(*)(int)>(lib, p + QStringLiteral("settings_get_string"));
        api.setBool = sym<int (*)(int, int)>(lib, p + QStringLiteral("settings_set_bool"));
        api.setEnum = sym<int (*)(int, const char *)>(lib, p + QStringLiteral("settings_set_enum"));
        api.setInt = sym<int (*)(int, int)>(lib, p + QStringLiteral("settings_set_int"));
        api.setString = sym<int (*)(int, const char *)>(lib, p + QStringLiteral("settings_set_string"));
        api.err = sym<const char *(*)()>(lib, p + QStringLiteral("settings_error"));
        if (!api.count || !api.label || !api.type || !api.enums || !api.getBool || !api.getEnum
            || !api.getInt || !api.setBool || !api.setEnum || !api.setInt || !api.err
            || api.count() < 1) {
            continue;
        }
        api.title = f.id;
        if (f.id == QLatin1String("settings"))
            api.title = QStringLiteral("Cargadgetd");
        else if (auto name = sym<const char *(*)()>(lib, p + QStringLiteral("name"))) {
            const QString n = take(name());
            if (!n.isEmpty())
                api.title = n;
        }
        m_apis.append(api);
    }
}

QString PluginSettings::take(const char *s)
{
    return s ? QString::fromUtf8(s) : QString();
}

void PluginSettings::rebuild()
{
    QVariantList sections;
    for (int s = 0; s < m_apis.size(); ++s) {
        const Api &api = m_apis.at(s);
        QVariantList rows;
        const int n = api.count();
        for (int i = 0; i < n; ++i) {
            const int kind = api.type(i);
            QVariantMap row;
            row.insert(QStringLiteral("section"), s);
            row.insert(QStringLiteral("row"), i);
            row.insert(QStringLiteral("label"), take(api.label(i)));
            row.insert(QStringLiteral("type"), kind);
            QStringList enums;
            const QString raw = take(api.enums(i));
            if (!raw.isEmpty()) {
                const QStringList parts = raw.split(QLatin1Char(','));
                for (const QString &part : parts) {
                    const QString t = part.trimmed();
                    if (!t.isEmpty())
                        enums.append(t);
                }
            }
            row.insert(QStringLiteral("enums"), enums);
            QString value;
            switch (kind) {
            case 0:
                value = api.getBool(i) ? QStringLiteral("1") : QStringLiteral("0");
                break;
            case 1:
                value = take(api.getEnum(i));
                break;
            case 2:
                value = QString::number(api.getInt(i));
                break;
            case 3:
                if (!api.getString || !api.setString)
                    continue;
                value = take(api.getString(i));
                break;
            default:
                continue;
            }
            row.insert(QStringLiteral("value"), value);
            rows.append(row);
        }
        if (rows.isEmpty())
            continue;
        QVariantMap sec;
        sec.insert(QStringLiteral("title"), api.title);
        sec.insert(QStringLiteral("rows"), rows);
        sections.append(sec);
    }
    m_sections = sections;
    emit sectionsChanged();
}

void PluginSettings::setError(const QString &err)
{
    if (m_error == err)
        return;
    m_error = err;
    emit errorChanged();
}

QString PluginSettings::setRow(int section, int row, const QString &value)
{
    if (section < 0 || section >= m_apis.size())
        return QStringLiteral("no such section");
    Api &api = m_apis[section];
    if (row < 0 || row >= api.count())
        return QStringLiteral("no such row");
    const int kind = api.type(row);
    QString cur;
    int rc = 1;
    switch (kind) {
    case 0:
        cur = api.getBool(row) ? QStringLiteral("1") : QStringLiteral("0");
        if (cur == value)
            return QString();
        rc = api.setBool(row, value == QLatin1String("1") ? 1 : 0);
        break;
    case 1:
        cur = take(api.getEnum(row));
        if (cur == value)
            return QString();
        rc = api.setEnum(row, value.toUtf8().constData());
        break;
    case 2: {
        bool ok = false;
        const int n = value.toInt(&ok);
        cur = QString::number(api.getInt(row));
        if (ok && cur == value)
            return QString();
        if (!ok)
            return QStringLiteral("not a number");
        rc = api.setInt(row, n);
        break;
    }
    case 3:
        if (!api.setString)
            return QStringLiteral("not a string");
        cur = take(api.getString(row));
        if (cur == value)
            return QString();
        rc = api.setString(row, value.toUtf8().constData());
        break;
    default:
        return QStringLiteral("unknown setting");
    }
    QString err;
    if (rc != 0) {
        err = api.err ? take(api.err()) : QString();
        if (err.isEmpty())
            err = QStringLiteral("rejected");
    }
    setError(err);
    rebuild();
    return err;
}
