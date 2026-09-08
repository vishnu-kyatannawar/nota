/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "itemmodel.h"

#include <QRegularExpression>

#include <algorithm>

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

void ItemModel::mergeSaved(const QList<MdNote::Item> &saved)
{
    if (saved.size() != m_items.size()) {
        // Structure moved under us, so there is nothing to merge into.
        setItems(saved);
        return;
    }
    for (qsizetype i = 0; i < saved.size(); ++i) {
        MdNote::Item &mine = m_items[i];
        const MdNote::Item &theirs = saved.at(i);
        if (mine.id == theirs.id && mine.createdAt == theirs.createdAt && mine.doneAt == theirs.doneAt
            && mine.from == theirs.from && mine.carried == theirs.carried && mine.recurring == theirs.recurring) {
            continue;
        }
        mine.id = theirs.id;
        mine.createdAt = theirs.createdAt;
        mine.doneAt = theirs.doneAt;
        mine.from = theirs.from;
        mine.carried = theirs.carried;
        mine.recurring = theirs.recurring;
        Q_EMIT dataChanged(index(int(i), 0),
                           index(int(i), 0),
                           {IdRole, CreatedAtRole, DoneAtRole, FromRole, CarriedRole, RecurringRole});
    }
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
        {IsRepeatingRole, QByteArrayLiteral("isRepeating")},
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
    case IsRepeatingRole:
        return !item.recurring.isEmpty();
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

bool ItemModel::valid(int row) const
{
    return row >= 0 && row < int(m_items.size());
}

int ItemModel::endOfSubtree(int row) const
{
    const int base = m_items.at(row).depth;
    int end = row + 1;
    while (end < int(m_items.size()) && !m_items.at(end).isHeading() && m_items.at(end).depth > base) {
        ++end;
    }
    return end;
}

int ItemModel::splitRow(int row, int cursorPos)
{
    if (!valid(row)) {
        return -1;
    }
    const MdNote::Item &here = m_items.at(row);
    const int caret = std::clamp(cursorPos, 0, int(here.text.size()));

    // At the start of a row that has text, the new row goes above and the item
    // keeps its identity: an id, a creation time and a carry count describe the
    // work, and the work is the text.
    if (caret == 0 && !here.text.isEmpty()) {
        MdNote::Item blank;
        blank.depth = here.depth;
        beginInsertRows({}, row, row);
        m_items.insert(row, blank);
        endInsertRows();
        Q_EMIT changed();
        return row + 1;
    }

    MdNote::Item tail;
    tail.text = here.text.sliced(caret);
    tail.depth = here.depth;
    // The body stays with the left half, which is the row that kept the id.

    // Truncating the left half has to happen outside the insert, not inside
    // it: a row may not change while an insertion around it is in flight.
    m_items[row].text = m_items.at(row).text.first(caret);
    Q_EMIT dataChanged(index(row, 0), index(row, 0));

    beginInsertRows({}, row + 1, row + 1);
    m_items.insert(row + 1, tail);
    endInsertRows();
    Q_EMIT changed();
    return row + 1;
}

int ItemModel::mergeWithPrevious(int row)
{
    if (!valid(row) || row == 0) {
        return -1;
    }
    const int above = row - 1;
    if (m_items.at(above).isHeading() || m_items.at(row).isHeading()) {
        return -1;
    }

    const int caret = int(m_items.at(above).text.size());
    const MdNote::Item folded = m_items.at(row);

    beginRemoveRows({}, row, row);
    m_items.removeAt(row);
    endRemoveRows();

    m_items[above].text += folded.text;
    if (!folded.body.isEmpty()) {
        m_items[above].body += folded.body;
    }

    // Anything that was nested under the folded row would now be two levels
    // below its parent, which the format cannot express.
    for (int i = above + 1; i < int(m_items.size()); ++i) {
        const int ceiling = m_items.at(i - 1).isHeading() ? 0 : m_items.at(i - 1).depth + 1;
        if (m_items.at(i).depth > ceiling) {
            m_items[i].depth = ceiling;
            Q_EMIT dataChanged(index(i, 0), index(i, 0), {DepthRole});
        }
    }

    Q_EMIT dataChanged(index(above, 0), index(above, 0));
    Q_EMIT changed();
    return caret;
}

bool ItemModel::indentRow(int row)
{
    if (!valid(row) || row == 0 || m_items.at(row).isHeading()) {
        return false;
    }
    const MdNote::Item &above = m_items.at(row - 1);
    if (above.isHeading()) {
        return false;
    }
    const int wanted = m_items.at(row).depth + 1;
    if (wanted > MaxDepth || wanted > above.depth + 1) {
        return false;
    }

    const int end = endOfSubtree(row);
    for (int i = row; i < end; ++i) {
        ++m_items[i].depth;
    }
    Q_EMIT dataChanged(index(row, 0), index(end - 1, 0), {DepthRole});
    Q_EMIT changed();
    return true;
}

bool ItemModel::outdentRow(int row)
{
    if (!valid(row) || m_items.at(row).isHeading() || m_items.at(row).depth == 0) {
        return false;
    }
    const int end = endOfSubtree(row);
    for (int i = row; i < end; ++i) {
        --m_items[i].depth;
    }
    Q_EMIT dataChanged(index(row, 0), index(end - 1, 0), {DepthRole});
    Q_EMIT changed();
    return true;
}

void ItemModel::setText(int row, const QString &text)
{
    if (!valid(row) || m_items.at(row).text == text) {
        return;
    }
    // Verbatim: labels and the [hh:mm] token are part of the line the user
    // wrote, and nothing here may reorder or normalise them.
    m_items[row].text = text;
    Q_EMIT dataChanged(index(row, 0), index(row, 0));
    Q_EMIT changed();
}

void ItemModel::setBody(int row, const QString &body)
{
    if (!valid(row)) {
        return;
    }
    const QStringList lines = body.isEmpty() ? QStringList() : body.split(u'\n');
    if (m_items.at(row).body == lines) {
        return;
    }
    m_items[row].body = lines;
    Q_EMIT dataChanged(index(row, 0), index(row, 0), {BodyRole, HasBodyRole});
    Q_EMIT changed();
}

void ItemModel::toggleDone(int row)
{
    if (!valid(row) || m_items.at(row).isHeading()) {
        return;
    }
    m_items[row].done = !m_items.at(row).done;
    // The completion stamp is a transition, not a flag, and it is written when
    // the note is saved — see Note::replaceItemsAt.
    Q_EMIT dataChanged(index(row, 0), index(row, 0), {DoneRole});
    Q_EMIT changed();
}

int ItemModel::insertItem(int afterRow, int depth)
{
    const int at = std::clamp(afterRow + 1, 0, int(m_items.size()));
    MdNote::Item item;
    item.depth = std::clamp(depth, 0, MaxDepth);

    beginInsertRows({}, at, at);
    m_items.insert(at, item);
    endInsertRows();
    Q_EMIT changed();
    return at;
}

int ItemModel::removeRow(int row)
{
    if (!valid(row)) {
        return -1;
    }
    const int end = endOfSubtree(row);

    beginRemoveRows({}, row, end - 1);
    m_items.remove(row, end - row);
    endRemoveRows();
    Q_EMIT changed();

    if (m_items.isEmpty()) {
        return -1;
    }
    return std::max(0, row - 1);
}

void ItemModel::makeHeading(int row)
{
    if (!valid(row) || m_items.at(row).isHeading()) {
        return;
    }
    MdNote::Item &item = m_items[row];
    item.kind = QString(MdNote::KindHeading);
    item.level = 2;
    item.done = false;
    item.depth = 0;
    // A heading is not work, so the bookkeeping that describes work goes.
    item.doneAt.clear();
    item.from.clear();
    item.carried = 0;
    item.recurring.clear();
    Q_EMIT dataChanged(index(row, 0), index(row, 0));
    Q_EMIT changed();
}

void ItemModel::makeItem(int row)
{
    if (!valid(row) || !m_items.at(row).isHeading()) {
        return;
    }
    m_items[row].kind.clear();
    m_items[row].level = 0;
    Q_EMIT dataChanged(index(row, 0), index(row, 0));
    Q_EMIT changed();
}

int ItemModel::pasteLines(int row, const QString &text)
{
    static const QRegularExpression prefix(uR"(^(\s*)- \[([ xX])\]\s?)"_s);

    QString normalised = text;
    normalised.replace("\r\n"_L1, "\n"_L1);
    const QStringList lines = normalised.split(u'\n');

    const int baseDepth = valid(row) ? m_items.at(row).depth : 0;
    int at = std::clamp(row + 1, 0, int(m_items.size()));
    int added = 0;

    for (const QString &line : lines) {
        if (line.trimmed().isEmpty()) {
            continue;
        }
        MdNote::Item item;
        const QRegularExpressionMatch m = prefix.match(line);
        if (m.hasMatch()) {
            // "- [x] " lines arrive already ticked, and their indent is depth.
            QString lead = m.captured(1);
            lead.replace(u'\t', QString(MdNote::IndentUnit, u' '));
            item.done = m.captured(2) != " "_L1;
            item.depth = std::clamp(baseDepth + int(lead.size()) / MdNote::IndentUnit, 0, MaxDepth);
            item.text = line.sliced(m.capturedLength(0)).trimmed();
        } else {
            item.depth = baseDepth;
            item.text = line.trimmed();
        }

        beginInsertRows({}, at + added, at + added);
        m_items.insert(at + added, item);
        endInsertRows();
        ++added;
    }

    if (added == 0) {
        return row;
    }
    Q_EMIT changed();
    return at + added - 1;
}

#include "moc_itemmodel.cpp"
