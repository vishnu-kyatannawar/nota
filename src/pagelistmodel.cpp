/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "pagelistmodel.h"

#include "vault.h"

#include <QSet>

using namespace Qt::StringLiterals;

PageListModel::PageListModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

void PageListModel::setVault(Vault *vault, const QString &workplanFolder)
{
    m_vault = vault;
    m_workplanFolder = workplanFolder;
    refresh();
}

void PageListModel::setFolder(const QString &folder)
{
    if (folder == m_folder) {
        return;
    }
    m_folder = folder;
    Q_EMIT folderChanged();
    refresh();
}

QList<PageListModel::Page> PageListModel::read() const
{
    if (!m_vault) {
        return {};
    }

    QList<Page> out;
    const QList<VaultNode> notes = m_vault->notesIn(m_folder);
    for (const VaultNode &note : notes) {
        out.append(Page{note.name, note.path});
    }

    // Workplans are named for their date, so name order is date order — and
    // the day anyone wants is nearly always the newest, not the oldest.
    if (!m_workplanFolder.isEmpty() && m_folder == m_workplanFolder) {
        std::reverse(out.begin(), out.end());
    }
    return out;
}

void PageListModel::refresh()
{
    const QList<Page> target = read();

    QSet<QString> wanted;
    for (const Page &page : target) {
        wanted.insert(page.path);
    }

    // Back to front, so the rows ahead of each removal keep their numbers.
    for (int row = int(m_pages.size()) - 1; row >= 0; --row) {
        if (!wanted.contains(m_pages.at(row).path)) {
            beginRemoveRows({}, row, row);
            m_pages.removeAt(row);
            endRemoveRows();
        }
    }

    // What survived kept its relative order, because both lists come out of
    // the same sort. So anything that does not match at its own row is new.
    for (int row = 0; row < int(target.size()); ++row) {
        if (row < int(m_pages.size()) && m_pages.at(row).path == target.at(row).path) {
            continue;
        }
        beginInsertRows({}, row, row);
        m_pages.insert(row, target.at(row));
        endInsertRows();
    }

    if (m_pages.size() != target.size()) {
        // Only reachable if the two passes above disagree, which would be a
        // bug in them rather than in the vault.
        beginResetModel();
        m_pages = target;
        endResetModel();
    }
    Q_EMIT countChanged();
}

int PageListModel::rowForPath(const QString &path) const
{
    for (int row = 0; row < int(m_pages.size()); ++row) {
        if (m_pages.at(row).path == path) {
            return row;
        }
    }
    return -1;
}

int PageListModel::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : int(m_pages.size());
}

QVariant PageListModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() < 0 || index.row() >= int(m_pages.size())) {
        return {};
    }
    const Page &page = m_pages.at(index.row());
    switch (role) {
    case Qt::DisplayRole:
    case NameRole:
        return page.name;
    case PathRole:
        return page.path;
    default:
        return {};
    }
}

QHash<int, QByteArray> PageListModel::roleNames() const
{
    return {
        {NameRole, QByteArrayLiteral("name")},
        {PathRole, QByteArrayLiteral("path")},
    };
}

#include "moc_pagelistmodel.cpp"
