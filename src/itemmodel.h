/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * One page's action items.
 *
 * A flat list with a depth role rather than a tree, because that is what the
 * file format is and what makes splicing rows on Tab and Backspace tractable.
 * Group headings live in the same list with KindRole == Heading: they occupy a
 * row, so "row index in the view" and "row index in the model" stay the same
 * number, which is what the focus handling depends on.
 */

#pragma once

#include "mdnote.h"

#include <QAbstractListModel>
#include <QQmlEngine>

class ItemModel : public QAbstractListModel
{
    Q_OBJECT
    QML_ELEMENT
    QML_UNCREATABLE("Get it from Nota.items")

public:
    enum Role {
        IdRole = Qt::UserRole + 1,
        TextRole, //!< verbatim, including #labels and the [hh:mm] token
        DoneRole,
        DepthRole,
        IsHeadingRole,
        HeadingLevelRole,
        LabelsRole,
        MinutesRole,
        DurationTextRole, //!< the logged time as "hh:mm", empty when none
        CreatedAtRole,
        DoneAtRole,
        FromRole,
        CarriedRole,
        RecurringRole, //!< the template id when this item repeats
        BodyRole, //!< the item's own markdown, newline joined
        HasBodyRole,
    };
    Q_ENUM(Role)

    explicit ItemModel(QObject *parent = nullptr);

    /*! Replaces the list with a note's items. */
    void setItems(const QList<MdNote::Item> &items);

    /*! The items as they would be written back. */
    QList<MdNote::Item> items() const
    {
        return m_items;
    }

    int rowCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role) const override;
    QHash<int, QByteArray> roleNames() const override;

    /*! How many action items are open and done, ignoring headings. */
    int openCount() const;
    int doneCount() const;

private:
    QList<MdNote::Item> m_items;
};
