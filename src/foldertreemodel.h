/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The sidebar tree, which is the directory tree.
 *
 * A real QAbstractItemModel rather than a pre-flattened list, so the view can
 * flatten it with KDescendantsProxyModel and so QAbstractItemModelTester can
 * check it. Nothing here caches beyond one snapshot: refresh() rebuilds from
 * the vault, which is cheap for a tree of markdown files and always right.
 */

#pragma once

#include <QAbstractItemModel>
#include <QQmlEngine>

#include <memory>
#include <vector>

class Vault;
struct VaultNode;

class FolderTreeModel : public QAbstractItemModel
{
    Q_OBJECT
    QML_ELEMENT
    QML_UNCREATABLE("Get it from Nota.folderTree")

public:
    enum Role {
        NameRole = Qt::UserRole + 1, //!< the display name, without the .md
        PathRole, //!< the vault-relative path
        IsFolderRole,
        IsWorkplanRole, //!< a dated note in the reserved folder
        IconNameRole,
    };
    Q_ENUM(Role)

    explicit FolderTreeModel(QObject *parent = nullptr);
    ~FolderTreeModel() override;

    /*! Points the model at a vault and reads it. */
    void setVault(Vault *vault, const QString &workplanFolder);

    QHash<int, QByteArray> roleNames() const override;
    QModelIndex index(int row, int column, const QModelIndex &parent = {}) const override;
    QModelIndex parent(const QModelIndex &child) const override;
    int rowCount(const QModelIndex &parent = {}) const override;
    int columnCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;

    /*! Rereads the vault. Called on any change the watcher reports. */
    Q_INVOKABLE void refresh();

private:
    struct Node {
        QString name;
        QString path;
        bool isFolder = false;
        Node *parent = nullptr;
        std::vector<std::unique_ptr<Node>> children;
    };

    void build(const VaultNode &source, Node *into);
    /*! Whether two trees hold the same folders and notes, in the same order. */
    static bool sameAs(const Node *a, const Node *b);
    const Node *nodeFor(const QModelIndex &index) const;

    Vault *m_vault = nullptr;
    QString m_workplanFolder;
    std::unique_ptr<Node> m_root;
};
