/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The single object QML talks to.
 *
 * It owns the vault, the settings and the workplan manager, and exposes the
 * open page. Everything it does is a thin call into notacore, which is where
 * the behaviour and the tests live; nothing here should ever be the only place
 * a rule is written down.
 */

#pragma once

#include "settings.h"
#include "workplan.h"

#include <QObject>
#include <QQmlEngine>
#include <QTimer>

#include <memory>

class FolderTreeModel;
class ItemModel;
class Vault;

class Nota : public QObject
{
    Q_OBJECT
    QML_ELEMENT
    QML_SINGLETON

    Q_PROPERTY(QString vaultPath READ vaultPath CONSTANT)
    Q_PROPERTY(QString version READ version CONSTANT)
    Q_PROPERTY(FolderTreeModel *folderTree READ folderTree CONSTANT)
    Q_PROPERTY(ItemModel *items READ items CONSTANT)

    Q_PROPERTY(QString currentPath READ currentPath NOTIFY currentChanged)
    Q_PROPERTY(QString title READ title NOTIFY currentChanged)
    Q_PROPERTY(QString subtitle READ subtitle NOTIFY currentChanged)
    Q_PROPERTY(QString body READ body NOTIFY currentChanged)
    Q_PROPERTY(QString hours READ hours NOTIFY currentChanged)
    Q_PROPERTY(QString dayType READ dayType NOTIFY currentChanged)
    Q_PROPERTY(bool isWorkplan READ isWorkplan NOTIFY currentChanged)
    Q_PROPERTY(int openCount READ openCount NOTIFY currentChanged)
    Q_PROPERTY(int doneCount READ doneCount NOTIFY currentChanged)
    Q_PROPERTY(QString errorMessage READ errorMessage NOTIFY errorMessageChanged)
    Q_PROPERTY(bool dirty READ isDirty NOTIFY dirtyChanged)

public:
    /*!
     * The engine builds the singleton lazily, and it needs the vault path the
     * command line may have named, so that is stashed before the engine runs.
     */
    static Nota *create(QQmlEngine *engine, QJSEngine *jsEngine);
    static void setStartupVault(const QString &path);

    explicit Nota(QObject *parent = nullptr);
    ~Nota() override;

    QString vaultPath() const;

    /*!
     * The build's version. The sidebar shows it because the first question
     * asked about any bug report is which version is actually running, and an
     * installed update does not take effect until the window is reopened.
     */
    QString version() const;
    FolderTreeModel *folderTree() const
    {
        return m_folderTree.get();
    }
    ItemModel *items() const
    {
        return m_items.get();
    }

    QString currentPath() const
    {
        return m_currentPath;
    }
    QString title() const;
    QString subtitle() const;
    QString body() const
    {
        return m_current.body;
    }
    QString hours() const
    {
        return m_current.hours;
    }
    QString dayType() const
    {
        return m_current.dayType;
    }
    bool isWorkplan() const;
    int openCount() const;
    int doneCount() const;
    QString errorMessage() const
    {
        return m_errorMessage;
    }
    /*! Whether there are edits the timers have not written out yet. */
    bool isDirty() const
    {
        return m_itemsDirty || m_bodyDirty;
    }

    /*! Opens a page by its vault-relative path. */
    Q_INVOKABLE void open(const QString &path);

    /*!
     * Makes sure today's workplan exists and opens it. Safe to call repeatedly:
     * it runs on launch, at midnight and whenever the window regains focus.
     */
    Q_INVOKABLE void openToday();

    /*! Replaces the prose under the items. Debounced like the items are. */
    Q_INVOKABLE void setBody(const QString &body);

    /*!
     * Writes any pending edit immediately. Called on a page switch, when the
     * window loses focus and on quit, so a debounce timer can never be the
     * reason an edit was lost.
     */
    Q_INVOKABLE void flush();

    /*! Records the hours worked on this day. Refuses anything but "hh:mm". */
    Q_INVOKABLE bool setHours(const QString &hours);

    /*! Marks the day work, weekend, leave or holiday. */
    Q_INVOKABLE bool setDayType(const QString &dayType);

    /*! Shows items, notes, or both. A workplan is always both. */
    Q_INVOKABLE bool setLayout(const QString &layout);

    /*!
     * Creates an untitled page in \a folder, opens it, and returns its path.
     * The name is made unique rather than overwriting whatever is there.
     */
    Q_INVOKABLE QString createNote(const QString &folder);

    /*! Creates a folder under \a parent. */
    Q_INVOKABLE bool createFolder(const QString &parent, const QString &name);

    /*! Renames a page or folder in place, keeping it where it is. */
    Q_INVOKABLE bool renamePath(const QString &path, const QString &newName);

    /*! Moves a page or folder to the trash. */
    Q_INVOKABLE bool removePath(const QString &path);

    /*!
     * Whether this path is the reserved workplan folder itself, which cannot
     * be renamed or deleted without orphaning every dated note under it.
     *
     * The notes under it are not reserved. A day is yours to throw away, and
     * refusing to delete one made every workplan in the sidebar undeletable.
     */
    Q_INVOKABLE bool isReserved(const QString &path) const;

    /*!
     * Whether this path is one of the dated notes in the workplan folder.
     *
     * It may be deleted, but not renamed: the filename is the date every
     * lookup and the rollover find the day by, so a new name orphans it.
     */
    Q_INVOKABLE bool isDatedPage(const QString &path) const;

    /*! Adds an item that repeats every day, and seeds it into today. */
    Q_INVOKABLE bool addRepeating(const QString &text);

    /*!
     * Stops an item repeating and takes it out of today. Workplans already
     * written keep their copy: what you did on a day is a record of that day.
     */
    Q_INVOKABLE bool stopRepeating(const QString &id);

    /*! Moves one row, with everything under it, into today's workplan. */
    Q_INVOKABLE bool moveRowToToday(int row);

    /*! The open page's bytes, unparsed, for the raw markdown view. */
    Q_INVOKABLE QString raw() const;

    /*!
     * Replaces the whole file. The only path that lets someone edit what the
     * editor cannot express, so it is deliberately unvalidated beyond parsing.
     */
    Q_INVOKABLE bool saveRaw(const QString &content);

    /*! Dismisses whatever went wrong last. */
    Q_INVOKABLE void clearError();

Q_SIGNALS:
    void currentChanged();
    void errorMessageChanged();
    void dirtyChanged();

private:
    void reload();
    /*! Drops the open page and any edit waiting on a debounce. */
    void closeCurrent();
    void armSave(QTimer *timer, bool *flag);
    void saveNow();
    void fail(const QString &message);
    void scheduleMidnightRoll();

    std::unique_ptr<Vault> m_vault;
    std::unique_ptr<Workplan::Manager> m_plans;
    std::unique_ptr<FolderTreeModel> m_folderTree;
    std::unique_ptr<ItemModel> m_items;

    NotaSettings::Settings m_settings;
    MdNote::Note m_current;
    QString m_currentPath;
    QString m_errorMessage;

    // Two debounces rather than one: prose is typed in longer runs than item
    // text, so it earns a slower timer.
    QTimer *m_itemSave = nullptr;
    QTimer *m_bodySave = nullptr;
    bool m_itemsDirty = false;
    bool m_bodyDirty = false;
    /*!
     * The page a pending save was armed for. A save that fires after the user
     * has moved on must not write these items into the page they moved to.
     */
    QString m_savePath;
};
