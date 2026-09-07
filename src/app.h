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

    /*! Opens a page by its vault-relative path. */
    Q_INVOKABLE void open(const QString &path);

    /*!
     * Makes sure today's workplan exists and opens it. Safe to call repeatedly:
     * it runs on launch, at midnight and whenever the window regains focus.
     */
    Q_INVOKABLE void openToday();

    /*! Dismisses whatever went wrong last. */
    Q_INVOKABLE void clearError();

Q_SIGNALS:
    void currentChanged();
    void errorMessageChanged();

private:
    void reload();
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
};
