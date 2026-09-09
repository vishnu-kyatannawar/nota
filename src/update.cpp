/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "update.h"

#include <KLocalizedString>

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QStandardPaths>
#include <QTemporaryDir>

using namespace Qt::StringLiterals;

namespace Update
{

QByteArray userAgent()
{
    return QByteArrayLiteral("Nota/") + QByteArrayLiteral(NOTA_VERSION_STRING);
}

Kind kindFor(const QString &binaryPath, const QString &home)
{
    if (home.isEmpty()) {
        return Kind::SystemManaged;
    }
    const QString path = QDir::cleanPath(binaryPath);
    const QString root = QDir::cleanPath(home);
    // The separator matters: /home/someone-else starts with /home/someone as
    // text, and is emphatically not inside it.
    return path.startsWith(root + u'/') ? Kind::UserManaged : Kind::SystemManaged;
}

int compare(const QString &a, const QString &b)
{
    const auto parts = [](const QString &v) {
        QString clean = v.trimmed();
        if (clean.startsWith(u'v') || clean.startsWith(u'V')) {
            clean = clean.sliced(1);
        }
        QList<int> out;
        const QStringList pieces = clean.split(u'.');
        for (const QString &piece : pieces) {
            bool ok = false;
            const int n = piece.toInt(&ok);
            out.append(ok ? n : 0);
        }
        return out;
    };

    const QList<int> left = parts(a);
    const QList<int> right = parts(b);
    // A missing part is a zero, so 5.1 and 5.1.0 are the same version.
    for (qsizetype i = 0; i < std::max(left.size(), right.size()); ++i) {
        const int x = i < left.size() ? left.at(i) : 0;
        const int y = i < right.size() ? right.at(i) : 0;
        if (x != y) {
            return x < y ? -1 : 1;
        }
    }
    return 0;
}

Release parseLatest(const QByteArray &json)
{
    const QJsonDocument doc = QJsonDocument::fromJson(json);
    if (!doc.isObject()) {
        return {};
    }
    const QJsonObject root = doc.object();

    QString tag = root.value("tag_name"_L1).toString();
    if (tag.startsWith(u'v') || tag.startsWith(u'V')) {
        tag = tag.sliced(1);
    }
    if (tag.isEmpty()) {
        return {};
    }
    return Release{tag, root.value("html_url"_L1).toString()};
}

Checker::Checker(QObject *parent)
    : QObject(parent)
    , m_net(new QNetworkAccessManager(this))
{
}

void Checker::check()
{
    QNetworkRequest request{QUrl(QString(LatestReleaseApi))};
    request.setRawHeader(QByteArrayLiteral("Accept"), QByteArrayLiteral("application/vnd.github+json"));
    request.setRawHeader(QByteArrayLiteral("User-Agent"), userAgent());
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::NoLessSafeRedirectPolicy);

    QNetworkReply *reply = m_net->get(request);
    connect(reply, &QNetworkReply::finished, this, [this, reply] {
        reply->deleteLater();
        if (reply->error() != QNetworkReply::NoError) {
            // Offline is not an error worth a message: nobody asked.
            return;
        }
        const Release release = parseLatest(reply->readAll());
        if (release.isValid()) {
            Q_EMIT found(release);
        }
    });
}

Installer::Installer(QObject *parent)
    : QObject(parent)
    , m_net(new QNetworkAccessManager(this))
{
}

bool Installer::isRunning() const
{
    return m_process != nullptr;
}

void Installer::start()
{
    if (isRunning()) {
        return;
    }

    QNetworkRequest request{QUrl(QString(InstallScriptUrl))};
    request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::NoLessSafeRedirectPolicy);
    request.setRawHeader(QByteArrayLiteral("User-Agent"), userAgent());

    Q_EMIT output(i18n("Fetching the installer…"));
    QNetworkReply *reply = m_net->get(request);
    connect(reply, &QNetworkReply::finished, this, [this, reply] {
        reply->deleteLater();
        if (reply->error() != QNetworkReply::NoError) {
            Q_EMIT finished(false, i18n("Could not download the installer: %1", reply->errorString()));
            return;
        }
        const QByteArray script = reply->readAll();
        // A truncated download must not run as half a script.
        if (!script.startsWith(QByteArrayLiteral("#!"))) {
            Q_EMIT finished(false, i18n("The installer that came back was not a script."));
            return;
        }

        const QString dir = QStandardPaths::writableLocation(QStandardPaths::TempLocation);
        const QString path = dir + "/nota-install.sh"_L1;
        QFile file(path);
        if (!file.open(QIODevice::WriteOnly)) {
            Q_EMIT finished(false, i18n("Could not write the installer to %1.", path));
            return;
        }
        file.write(script);
        file.close();

        m_scriptPath = path;
        run(path);
    });
}

void Installer::run(const QString &scriptPath)
{
    m_process = new QProcess(this);
    m_process->setProcessChannelMode(QProcess::MergedChannels);

    connect(m_process, &QProcess::readyRead, this, [this] {
        const QString text = QString::fromUtf8(m_process->readAll());
        const QStringList lines = text.split(u'\n', Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            Q_EMIT output(line.trimmed());
        }
    });

    connect(m_process, &QProcess::finished, this, [this](int code, QProcess::ExitStatus status) {
        const bool ok = status == QProcess::NormalExit && code == 0;
        m_process->deleteLater();
        m_process = nullptr;
        QFile::remove(m_scriptPath);
        m_scriptPath.clear();

        if (ok) {
            Q_EMIT finished(true, {});
        } else {
            Q_EMIT finished(false, i18n("The installer stopped with code %1. The output above says why.", code));
        }
    });

    connect(m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError) {
        if (!m_process) {
            return;
        }
        const QString message = m_process->errorString();
        m_process->deleteLater();
        m_process = nullptr;
        Q_EMIT finished(false, message);
    });

    Q_EMIT output(i18n("Building. This takes a few minutes."));
    m_process->start(u"sh"_s, {scriptPath});
}

}

#include "moc_update.cpp"
