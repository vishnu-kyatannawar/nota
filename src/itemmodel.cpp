/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "itemmodel.h"

using namespace Qt::StringLiterals;

ItemModel::ItemModel(QObject *parent)
    : QAbstractListModel(parent)
{
}

void ItemModel::setItems(const QList<MdNote::Item> &items)
{
    beginResetModel();
    m_items = items;
    endResetModel();
}

int ItemModel::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : int(m_items.size());
}

QHash<int, QByteArray> ItemModel::roleNames() const
{
    return {
        {IdRole, QByteArrayLiteral("itemId")},
        {TextRole, QByteArrayLiteral("text")},
        {DoneRole, QByteArrayLiteral("done")},
        {DepthRole, QByteArrayLiteral("depth")},
        {IsHeadingRole, QByteArrayLiteral("isHeading")},
        {HeadingLevelRole, QByteArrayLiteral("headingLevel")},
        {LabelsRole, QByteArrayLiteral("labels")},
        {MinutesRole, QByteArrayLiteral("minutes")},
        {DurationTextRole, QByteArrayLiteral("durationText")},
        {CreatedAtRole, QByteArrayLiteral("createdAt")},
        {DoneAtRole, QByteArrayLiteral("doneAt")},
        {FromRole, QByteArrayLiteral("from")},
        {CarriedRole, QByteArrayLiteral("carried")},
        {RecurringRole, QByteArrayLiteral("recurring")},
        {BodyRole, QByteArrayLiteral("body")},
        {HasBodyRole, QByteArrayLiteral("hasBody")},
    };
}

QVariant ItemModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.row() < 0 || index.row() >= m_items.size()) {
        return {};
    }
    const MdNote::Item &item = m_items.at(index.row());

    switch (role) {
    case Qt::DisplayRole:
    case TextRole:
        return item.text;
    case IdRole:
        return item.id;
    case DoneRole:
        return item.done;
    case DepthRole:
        return item.depth;
    case IsHeadingRole:
        return item.isHeading();
    case HeadingLevelRole:
        return item.level;
    case LabelsRole:
        return item.labels();
    case MinutesRole:
        return item.minutes();
    case DurationTextRole: {
        const int minutes = item.minutes();
        return minutes > 0 ? MdNote::formatDuration(minutes) : QString();
    }
    case CreatedAtRole:
        return item.createdAt;
    case DoneAtRole:
        return item.doneAt;
    case FromRole:
        return item.from;
    case CarriedRole:
        return item.carried;
    case RecurringRole:
        return item.recurring;
    case BodyRole:
        return item.body.join(u'\n');
    case HasBodyRole:
        return !item.body.isEmpty();
    default:
        return {};
    }
}

int ItemModel::openCount() const
{
    int n = 0;
    for (const MdNote::Item &item : m_items) {
        if (!item.isHeading() && !item.done) {
            ++n;
        }
    }
    return n;
}

int ItemModel::doneCount() const
{
    int n = 0;
    for (const MdNote::Item &item : m_items) {
        if (!item.isHeading() && item.done) {
            ++n;
        }
    }
    return n;
}

#include "moc_itemmodel.cpp"
