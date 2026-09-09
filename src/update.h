/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * Finding out whether a newer Nota exists, and fetching it.
 *
 * There is no binary to swap: a KDE Frameworks application links the libraries
 * its own distribution ships, so Nota is installed by building it. An update is
 * therefore a build, which is slow, needs the development packages, and must
 * never be done behind a package manager's back — so who owns the running
 * binary decides what this is allowed to offer.
 *
 * The comparison, the parsing and that ownership rule are plain functions with
 * no network and no process in them, because they are the parts that can be
 * wrong in a way a user would notice.
 */

#pragma once

#include <QObject>
#include <QProcess>
#include <QString>

class QNetworkAccessManager;

namespace Update
{

/*! Where the release information comes from. */
inline constexpr QLatin1StringView LatestReleaseApi{"https://api.github.com/repos/vishnu-kyatannawar/nota/releases/latest"};

/*! The installer the self-update runs. */
inline constexpr QLatin1StringView InstallScriptUrl{"https://raw.githubusercontent.com/vishnu-kyatannawar/nota/main/install.sh"};

/*!
 * What we identify as. GitHub's API answers 403 to a request with no
 * User-Agent, and Qt does not set one — so without this the check fails
 * silently on every machine and the updater is dead on arrival.
 */
QByteArray userAgent();

/*! Who owns the running binary, which decides what an update may do. */
enum class Kind {
    /*!
     * Installed under the user's own home by install.sh, so replacing it is
     * this application's business and nobody else's.
     */
    UserManaged,
    /*!
     * Installed under a system prefix, so a package manager owns those files.
     * Writing over them would leave its database describing files that are no
     * longer there, so the offer becomes "your package manager has this".
     */
    SystemManaged,
};

/*! Classifies \a binaryPath against the user's \a home directory. */
Kind kindFor(const QString &binaryPath, const QString &home);

/*!
 * Orders two dotted version strings, like strcmp: negative, zero or positive.
 *
 * A leading "v" is ignored and a missing part counts as zero, so "5.1" and
 * "v5.1.0" are the same version. Anything unparsable in a part sorts as zero
 * rather than throwing the comparison off.
 */
int compare(const QString &a, const QString &b);

/*! What a releases/latest payload says. */
struct Release {
    /*! The tag with any leading "v" removed, so it compares against ours. */
    QString version;
    /*! The release page, for someone who would rather read before updating. */
    QString url;

    bool isValid() const
    {
        return !version.isEmpty();
    }
};

/*! Reads a GitHub releases/latest payload. An unusable body yields an invalid Release. */
Release parseLatest(const QByteArray &json);

/*!
 * Asks GitHub what the newest release is.
 *
 * One request, no retry and no schedule: this runs once at launch, and a
 * failure is silence rather than an error — someone who is offline is not
 * asking to be told about it.
 */
class Checker : public QObject
{
    Q_OBJECT

public:
    explicit Checker(QObject *parent = nullptr);

    void check();

Q_SIGNALS:
    /*! Emitted only when the payload parsed. Failures are dropped. */
    void found(const Update::Release &release);

private:
    QNetworkAccessManager *m_net = nullptr;
};

/*!
 * Runs the installer, which builds the newest release from source.
 *
 * The script is fetched to a file and handed to sh rather than piped into it,
 * so a truncated download cannot execute as half a script.
 */
class Installer : public QObject
{
    Q_OBJECT

public:
    explicit Installer(QObject *parent = nullptr);

    /*! Whether a run is in progress. */
    bool isRunning() const;

    /*! Fetches the installer and runs it. Does nothing if already running. */
    void start();

Q_SIGNALS:
    /*! A line of the installer's output, for the progress view. */
    void output(const QString &line);
    /*! The run ended. \a message carries the reason when it failed. */
    void finished(bool ok, const QString &message);

private:
    void run(const QString &scriptPath);

    QNetworkAccessManager *m_net = nullptr;
    QProcess *m_process = nullptr;
    QString m_scriptPath;
};

}

Q_DECLARE_METATYPE(Update::Release)
