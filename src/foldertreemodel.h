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

    void build(const QList<VaultNode> &source, Node *into);
    /*!
     * The folders among \a children, with the workplan folder first when this
     * is the top level.
     *
     * Notes are not in this model at all: the sidebar shows folders and the
     * page list shows what is inside the one selected. The workplan folder is
     * pinned because it is the one folder someone opens every day, and it
     * would otherwise sit wherever its name happened to sort.
     */
    QList<VaultNode> foldersIn(const QList<VaultNode> &children, bool topLevel) const;
    /*!
     * Brings \a current in line with \a target, reporting the difference as
     * row insertions and removals rather than a reset.
     *
     * A reset is what collapses the sidebar: the view has no way to tell that
     * the node it had expanded is the same node afterwards, so everything
     * closes and the scroll position goes with it. Creating one page must not
     * cost the user the shape of their tree.
     */
    void syncChildren(Node *current, const QList<VaultNode> &target, const QModelIndex &parentIndex);
    const Node *nodeFor(const QModelIndex &index) const;

    Vault *m_vault = nullptr;
    QString m_workplanFolder;
    std::unique_ptr<Node> m_root;
};
