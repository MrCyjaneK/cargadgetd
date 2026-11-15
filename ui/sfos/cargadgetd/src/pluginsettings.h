#ifndef PLUGINSETTINGS_H
#define PLUGINSETTINGS_H

#include <QObject>
#include <QVariantList>
#include <QVector>

// Reads the same settings ABI the in-car Settings app uses:
// cargadgetd_app_<id>_settings_* and cargadgetd_shell_<id>_settings_*.
class PluginSettings : public QObject
{
    Q_OBJECT
    Q_PROPERTY(QVariantList sections READ sections NOTIFY sectionsChanged)
    Q_PROPERTY(QString error READ error NOTIFY errorChanged)

public:
    explicit PluginSettings(QObject *parent = nullptr);
    ~PluginSettings() override;

    QVariantList sections() const { return m_sections; }
    QString error() const { return m_error; }

    Q_INVOKABLE void reload();
    // value is "0"/"1", an enum token, a decimal, or a string.
    // Empty return means the value was stored.
    Q_INVOKABLE QString setRow(int section, int row, const QString &value);

signals:
    void sectionsChanged();
    void errorChanged();

private:
    struct Api {
        void *lib = nullptr;
        QString title;
        int (*count)() = nullptr;
        const char *(*label)(int) = nullptr;
        int (*type)(int) = nullptr;
        const char *(*enums)(int) = nullptr;
        int (*getBool)(int) = nullptr;
        const char *(*getEnum)(int) = nullptr;
        int (*getInt)(int) = nullptr;
        const char *(*getString)(int) = nullptr;
        int (*setBool)(int, int) = nullptr;
        int (*setEnum)(int, const char *) = nullptr;
        int (*setInt)(int, int) = nullptr;
        int (*setString)(int, const char *) = nullptr;
        const char *(*err)() = nullptr;
    };

    void scan();
    void rebuild();
    void setError(const QString &err);
    static QString take(const char *s);

    bool m_scanned = false;
    QVector<Api> m_apis;
    QVariantList m_sections;
    QString m_error;
};

#endif
