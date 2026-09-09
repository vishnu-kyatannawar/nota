/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The only code that touches the notes directory.
 *
 * The vault is a plain tree of markdown files that the user also opens in other
 * editors, so this treats the filesystem as the source of truth: it owns reads,
 * writes, moves and deletes, and it watches for changes made outside the
 * application. Every path crossing this boundary is vault-relative and
 * validated, because a path that escapes the root would read or write arbitrary
 * files.
 */

#pragma once

#include "mdnote.h"

#include <QList>
#include <QObject>
#include <QString>

#include <optional>

class KDirWatch;

/*! One entry in the folder tree. */
struct VaultNode {
    /*!
     * The display name: a folder's directory name, or a note's filename without
     * the .md extension.
     */
    QString name;
    /*! The vault-relative path, always with forward slashes. */
    QString path;
    bool isFolder = false;
    QList<VaultNode> children;
};

class Vault : public QObject
{
    Q_OBJECT

public:
    /*! The only file extension the vault treats as a note. */
    static constexpr QLatin1StringView NoteExt = MdNote::Ext;

    /*!
     * The vault-internal folder holding settings, templates and the trash. It
     * is skipped when walking the tree.
     */
    static constexpr QLatin1StringView AppDirName{".nota"};

    /*! The folder under the app directory that holds deleted notes. */
    static constexpr QLatin1StringView TrashDirName{"trash"};

    /*! Where pasted images are written. Skipped in the tree; it holds no notes. */
    static constexpr QLatin1StringView AttachmentDir{"attachments"};

    /*! Prepares the directory at \a root as a vault, creating it if needed. */
    explicit Vault(const QString &root, QObject *parent = nullptr);
    ~Vault() override;

    /*! The absolute path of the vault directory. */
    QString root() const
    {
        return m_root;
    }

    /*! Whether the root exists and is usable. */
    bool isOpen() const
    {
        return m_open;
    }

    /*! What the last failed call went wrong with, for the UI to show. */
    QString lastError() const
    {
        return m_lastError;
    }

    /*!
     * Turns a vault-relative path into an absolute one, refusing anything that
     * would land outside the vault. Absolute inputs, empty inputs and any path
     * climbing through ".." are rejected rather than normalised, so a caller
     * cannot accidentally address the wider filesystem.
     */
    std::optional<QString> resolve(const QString &rel) const;

    /*! Whether the note or folder exists. */
    bool exists(const QString &rel) const;

    /*! Parses the note at a vault-relative path. */
    std::optional<MdNote::Note> readNote(const QString &rel) const;

    /*! Whether a vault-relative path names a directory rather than a note. */
    bool isFolder(const QString &rel) const;

    /*! The note's bytes without parsing, for the raw markdown editor. */
    std::optional<QString> readRaw(const QString &rel) const;

    /*! Serialises a note and replaces the file at a vault-relative path. */
    bool writeNote(const QString &rel, const MdNote::Note &note);

    /*!
     * Replaces a note's contents. The write goes to a temporary file in the
     * same directory and is then renamed, so a crash mid-write cannot truncate
     * a note the user already has.
     */
    bool writeRaw(const QString &rel, const QString &content);

    /*! Creates a folder and any missing parents. */
    bool createFolder(const QString &rel);

    /*!
     * Moves a note or folder. Refuses to overwrite an existing path, since
     * silently replacing a note the user cannot see would lose their work.
     */
    bool rename(const QString &from, const QString &to);

    /*!
     * Moves a note, or a folder and everything under it, into the trash, where
     * it can be restored until it is purged. Nothing is removed outright, so a
     * mis-click in the sidebar costs nothing.
     */
    bool remove(const QString &rel);

    /*!
     * Walks the vault and returns its folders and notes. Folders sort before
     * notes and each group sorts by name, which is the order the sidebar shows.
     */
    VaultNode tree() const;

    /*! Every note path in the vault, forward-slashed and sorted. */
    QStringList listNotes() const;

    /*!
     * Starts reporting changes made outside the application. Editors often
     * write through a temporary file, so create, write and rename all surface
     * as a change.
     */
    void startWatching();

    /*!
     * Suppresses the next watcher event for \a rel. Every write this class
     * makes calls it, so the app is never told about its own saves.
     */
    void ignoreNextChange(const QString &rel);

Q_SIGNALS:
    /*! A note was written or created outside the application. */
    void noteChanged(const QString &path);
    /*! A note or folder disappeared outside the application. */
    void noteRemoved(const QString &path);
    /*! The folder structure changed, so the sidebar tree needs rebuilding. */
    void treeChanged();

private:
    QList<VaultNode> readDir(const QString &abs, const QString &rel) const;
    QString trashDir() const;
    void onDirty(const QString &absolutePath);
    QString relativeFor(const QString &absolutePath) const;
    bool fail(const QString &message) const;

    QString m_root;
    bool m_open = false;
    mutable QString m_lastError;
    KDirWatch *m_watch = nullptr;
    QStringList m_ignoreOnce;
};
