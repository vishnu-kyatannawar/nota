/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "foldertreemodel.h"

#include "vault.h"

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

bool FolderTreeModel::sameAs(const Node *a, const Node *b)
{
    if (a->children.size() != b->children.size()) {
        return false;
    }
    for (size_t i = 0; i < a->children.size(); ++i) {
        const Node *x = a->children.at(i).get();
        const Node *y = b->children.at(i).get();
        if (x->path != y->path || x->name != y->name || x->isFolder != y->isFolder || !sameAs(x, y)) {
            return false;
        }
    }
    return true;
}

void FolderTreeModel::refresh()
{
    auto rebuilt = std::make_unique<Node>();
    rebuilt->isFolder = true;
    if (m_vault) {
        build(m_vault->tree(), rebuilt.get());
    }

    // Saving a note marks its folder dirty, so this runs every few hundred
    // milliseconds while someone types. A reset then collapses every folder
    // they had expanded and drops the sidebar's scroll position, which is why
    // this compares first and only resets when the tree really did change.
    if (sameAs(m_root.get(), rebuilt.get())) {
        return;
    }

    beginResetModel();
    m_root = std::move(rebuilt);
    endResetModel();
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
