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
        IsRepeatingRole, //!< grouped on, so the view can pin what repeats to the top
        BodyRole, //!< the item's own markdown, newline joined
        HasBodyRole,
    };
    Q_ENUM(Role)

    /*!
     * How deep an item may nest. Beyond this the indent stops meaning
     * anything, and the file format's two-space steps get unreadable.
     */
    static constexpr int MaxDepth = 6;

    explicit ItemModel(QObject *parent = nullptr);

    /*! Replaces the list with a note's items. Resets the model. */
    void setItems(const QList<MdNote::Item> &items);

    /*!
     * Takes back the bookkeeping a save produced — minted ids, creation times,
     * completion stamps — without disturbing the text or the row count.
     *
     * setItems() would do the same job through a model reset, which destroys
     * the delegate the user is typing in. A save happens every 400 ms while
     * someone types, so that reset would make the editor unusable.
     */
    void mergeSaved(const QList<MdNote::Item> &saved);

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

    /*!
     * Splits row \a row at \a cursorPos and returns the row that should now
     * hold the caret.
     *
     * Splitting at the start pushes a fresh empty row above rather than
     * leaving the empty half holding the item's id: the identity, the creation
     * time and the carry counters follow the text, not the position.
     */
    Q_INVOKABLE int splitRow(int row, int cursorPos);

    /*!
     * Folds a row into the one above and returns where the caret belongs in
     * it, or -1 when there is nothing to fold into. Refused above a heading,
     * which would silently turn a day's work into a title.
     */
    Q_INVOKABLE int mergeWithPrevious(int row);

    /*!
     * Nests a row one level deeper, carrying everything under it. Refused on
     * the first row, past MaxDepth, and where it would leave a gap — an item
     * may never be more than one level deeper than the row above it.
     */
    Q_INVOKABLE bool indentRow(int row);

    /*! Lifts a row one level, carrying everything under it. */
    Q_INVOKABLE bool outdentRow(int row);

    /*! Replaces a row's visible text verbatim, labels and [hh:mm] included. */
    Q_INVOKABLE void setText(int row, const QString &text);

    /*! Replaces a row's own markdown body. */
    Q_INVOKABLE void setBody(int row, const QString &body);

    /*! Ticks or unticks a row. Headings have no checkbox and are ignored. */
    Q_INVOKABLE void toggleDone(int row);

    /*! Adds an empty row after \a row at \a depth and returns its index. */
    Q_INVOKABLE int insertItem(int afterRow, int depth);

    /*!
     * Deletes a row and everything nested beneath it — a child left without
     * its parent would silently change meaning — and returns the row that
     * should take the caret.
     */
    Q_INVOKABLE int removeRow(int row);

    /*! Turns a row into a group heading, and back. */
    Q_INVOKABLE void makeHeading(int row);
    Q_INVOKABLE void makeItem(int row);

    /*!
     * Splits pasted text into one row per line after \a row, honouring
     * "- [x] " prefixes and leading indent, and returns the last row added.
     */
    Q_INVOKABLE int pasteLines(int row, const QString &text);

Q_SIGNALS:
    /*!
     * Something changed that belongs on disk. The save pipeline is armed by
     * this, so a mutation that does not emit it is a lost edit.
     */
    void changed();

private:
    bool valid(int row) const;
    /*! The row after the last descendant of \a row. */
    int endOfSubtree(int row) const;

    QList<MdNote::Item> m_items;
};
