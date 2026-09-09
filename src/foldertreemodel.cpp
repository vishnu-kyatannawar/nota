/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "foldertreemodel.h"

#include "vault.h"

#include <QSet>

using namespace Qt::StringLiterals;

FolderTreeModel::FolderTreeModel(QObject *parent)
    : QAbstractItemModel(parent)
    , m_root(std::make_unique<Node>())
{
    m_root->isFolder = true;
}

FolderTreeModel::~FolderTreeModel() = default;

void FolderTreeModel::setVault(Vault *vault, const QString &workplanFolder)
{
    m_vault = vault;
    m_workplanFolder = workplanFolder;
    refresh();
}

void FolderTreeModel::build(const VaultNode &source, Node *into)
{
    for (const VaultNode &child : source.children) {
        auto node = std::make_unique<Node>();
        node->name = child.name;
        node->path = child.path;
        node->isFolder = child.isFolder;
        node->parent = into;
        build(child, node.get());
        into->children.push_back(std::move(node));
    }
}

void FolderTreeModel::syncChildren(Node *current, const QList<VaultNode> &target, const QModelIndex &parentIndex)
{
    // Both lists come out of Vault::tree(), which sorts folders before notes
    // and each group by name. So whatever survives keeps its relative order,
    // and that is the only property this needs: no comparator is duplicated
    // here, where it could drift out of step with the one in the vault.
    QSet<QString> wanted;
    for (const VaultNode &child : target) {
        wanted.insert(child.path);
    }

    // Back to front, so the indices ahead of each removal stay valid.
    for (int row = int(current->children.size()) - 1; row >= 0; --row) {
        if (!wanted.contains(current->children.at(size_t(row))->path)) {
            beginRemoveRows(parentIndex, row, row);
            current->children.erase(current->children.begin() + row);
            endRemoveRows();
        }
    }

    for (qsizetype row = 0; row < target.size(); ++row) {
        const VaultNode &want = target.at(row);
        if (size_t(row) < current->children.size() && current->children.at(size_t(row))->path == want.path) {
            continue;
        }
        auto node = std::make_unique<Node>();
        node->name = want.name;
        node->path = want.path;
        node->isFolder = want.isFolder;
        node->parent = current;
        build(want, node.get());

        beginInsertRows(parentIndex, int(row), int(row));
        current->children.insert(current->children.begin() + row, std::move(node));
        endInsertRows();
    }

    // The two lists now hold the same paths in the same order, so the rest is
    // a walk down the pairs. A node inserted above already carries its whole
    // subtree, which makes its recursion a no-op.
    for (qsizetype row = 0; row < target.size(); ++row) {
        Node *node = current->children.at(size_t(row)).get();
        syncChildren(node, target.at(row).children, index(int(row), 0, parentIndex));
    }
}

void FolderTreeModel::refresh()
{
    if (!m_vault) {
        return;
    }
    // Saving a note marks its folder dirty, so this runs every few hundred
    // milliseconds while someone types. Nothing is emitted when nothing
    // changed, and a real change arrives as rows rather than as a reset --
    // a reset closes every folder the user had expanded.
    syncChildren(m_root.get(), m_vault->tree().children, QModelIndex());
}

const FolderTreeModel::Node *FolderTreeModel::nodeFor(const QModelIndex &index) const
{
    if (!index.isValid()) {
        return m_root.get();
    }
    return static_cast<const Node *>(index.constInternalPointer());
}

QHash<int, QByteArray> FolderTreeModel::roleNames() const
{
    return {
        {NameRole, QByteArrayLiteral("name")},
        {PathRole, QByteArrayLiteral("path")},
        {IsFolderRole, QByteArrayLiteral("isFolder")},
        {IsWorkplanRole, QByteArrayLiteral("isWorkplan")},
        {IconNameRole, QByteArrayLiteral("iconName")},
    };
}

QModelIndex FolderTreeModel::index(int row, int column, const QModelIndex &parent) const
{
    if (column != 0 || row < 0 || !hasIndex(row, column, parent)) {
        return {};
    }
    const Node *parentNode = nodeFor(parent);
    return createIndex(row, column, parentNode->children.at(size_t(row)).get());
}

QModelIndex FolderTreeModel::parent(const QModelIndex &child) const
{
    if (!child.isValid()) {
        return {};
    }
    const Node *node = nodeFor(child);
    Node *parentNode = node->parent;
    if (!parentNode || parentNode == m_root.get()) {
        return {};
    }

    const Node *grandparent = parentNode->parent;
    for (size_t i = 0; i < grandparent->children.size(); ++i) {
        if (grandparent->children.at(i).get() == parentNode) {
            return createIndex(int(i), 0, parentNode);
        }
    }
    return {};
}

int FolderTreeModel::rowCount(const QModelIndex &parent) const
{
    if (parent.column() > 0) {
        return 0;
    }
    return int(nodeFor(parent)->children.size());
}

int FolderTreeModel::columnCount(const QModelIndex &) const
{
    return 1;
}

QVariant FolderTreeModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid()) {
        return {};
    }
    const Node *node = nodeFor(index);
    const bool isWorkplan = !node->isFolder && !m_workplanFolder.isEmpty() && node->path.startsWith(m_workplanFolder + u'/');

    switch (role) {
    case Qt::DisplayRole:
    case NameRole:
        return node->name;
    case PathRole:
        return node->path;
    case IsFolderRole:
        return node->isFolder;
    case IsWorkplanRole:
        return isWorkplan;
    case IconNameRole:
        if (node->isFolder) {
            return node->path == m_workplanFolder ? u"view-calendar-day"_s : u"folder-symbolic"_s;
        }
        return isWorkplan ? u"view-calendar-day"_s : u"text-markdown"_s;
    default:
        return {};
    }
}

#include "moc_foldertreemodel.cpp"
