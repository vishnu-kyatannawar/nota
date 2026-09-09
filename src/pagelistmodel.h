/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The pages inside one folder.
 *
 * The sidebar shows folders and this shows what is in the one that is
 * selected, which is why neither model carries the other's rows. Order is by
 * name, except in the workplan folder: those are named for their date, and the
 * day you want is nearly always the most recent one, so they run newest first.
 *
 * Like the folder tree, this reports change as rows rather than as a reset. A
 * reset would throw away the selection and the scroll position every time a
 * save touched the folder.
 */

#pragma once

#include <QAbstractListModel>
#include <QQmlEngine>

class Vault;

class PageListModel : public QAbstractListModel
{
    Q_OBJECT
    QML_ELEMENT
    QML_UNCREATABLE("Get it from Nota.pages")

    /*! The folder being shown. Empty means the vault root. */
    Q_PROPERTY(QString folder READ folder NOTIFY folderChanged)
    Q_PROPERTY(int count READ count NOTIFY countChanged)

public:
    enum Role {
        NameRole = Qt::UserRole + 1, //!< the filename without the .md
        PathRole, //!< the vault-relative path
    };
    Q_ENUM(Role)

    explicit PageListModel(QObject *parent = nullptr);

    void setVault(Vault *vault, const QString &workplanFolder);

    QString folder() const
    {
        return m_folder;
    }
    int count() const
    {
        return int(m_pages.size());
    }

    /*! Shows \a folder. Passing the folder already shown does nothing. */
    void setFolder(const QString &folder);

    /*! Rereads the folder. Emits rows only where something actually changed. */
    Q_INVOKABLE void refresh();

    /*! The row holding \a path, or -1. */
    Q_INVOKABLE int rowForPath(const QString &path) const;

    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

Q_SIGNALS:
    void folderChanged();
    void countChanged();

private:
    struct Page {
        QString name;
        QString path;
    };

    QList<Page> read() const;

    Vault *m_vault = nullptr;
    QString m_workplanFolder;
    QString m_folder;
    QList<Page> m_pages;
};
