/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * Parses and serialises Nota's note format.
 *
 * The format is ordinary markdown, chosen so a note stays readable and editable
 * without this application. Action items are GitHub-style task list entries;
 * labels (#label) and logged time ([hh:mm]) stay visible inline where a person
 * would write them anyway, and only machine bookkeeping — stable ids and exact
 * timestamps — hides in an HTML comment that markdown renderers do not display.
 *
 * The parser is deliberately line-based rather than built on a markdown AST.
 * Round-tripping a note without changing it is this file's entire promise, and
 * an AST would have to reconstruct formatting it never recorded. Working a line
 * at a time means the bytes we did not interpret are the bytes we write back.
 * Item text is likewise stored verbatim: labels() and minutes() are derived
 * views over that text, never a replacement for it.
 */

#pragma once

#include <QList>
#include <QString>
#include <QStringList>

namespace MdNote
{

/*! The file extension a note is stored with. */
inline constexpr QLatin1StringView Ext(".md");

/*!
 * Marks an Item that is a group heading between items rather than an action
 * item: "## Must". It has no checkbox, id or timestamps.
 */
inline constexpr QLatin1StringView KindHeading("heading");

/*!
 * Layouts decide which sections a note shows. A workplan is always both, with
 * items first; any other note may be items only, notes only, or both.
 */
inline constexpr QLatin1StringView LayoutItems("items");
inline constexpr QLatin1StringView LayoutNotes("notes");
inline constexpr QLatin1StringView LayoutBoth("both");

/*! The note type a dated daily note carries in its frontmatter. */
inline constexpr QLatin1StringView TypeWorkplan("workplan");

/*!
 * How far an item's body is indented past the item's own bullet, which lines it
 * up under the text after "- [ ] ".
 */
inline constexpr int BodyIndent = 6;

/*! How many spaces one level of item nesting uses. */
inline constexpr int IndentUnit = 2;

/*! One action item, or a group heading between items (see KindHeading). */
struct Item {
    /*! Empty for an action item, KindHeading for a heading. */
    QString kind;
    /*! The heading level (1–6) for a heading; new ones are written as 2. */
    int level = 0;
    /*!
     * The visible text, including any #labels and [hh:mm] token. Stored exactly
     * as written so serialising cannot reorder or drop anything.
     */
    QString text;
    bool done = false;
    /*! The nesting level; 0 is top level. */
    int depth = 0;
    /*! The item's own markdown lines with the common indent removed. */
    QStringList body;

    QString id;        //!< stable across rollover
    QString createdAt; //!< "hh:mm" — when the item was added
    QString doneAt;    //!< "hh:mm" — when it was ticked
    QString from;      //!< "yyyy-mm-dd" the item was first created, once carried
    int carried = 0;   //!< how many days it has rolled over
    QString recurring; //!< id of the recurring template that seeded it

    bool isHeading() const
    {
        return kind == KindHeading;
    }

    /*! The #labels written in the text, in order of appearance. */
    QStringList labels() const;

    /*! The logged time from the [hh:mm] token, in minutes, or zero. */
    int minutes() const;

    /*!
     * Writes the logged time into the text, replacing an existing token in
     * place so the rest of the line keeps its wording and order. Zero removes
     * the token entirely rather than writing a meaningless [00:00].
     */
    void setMinutes(int minutes);
};

/*! A single markdown file: frontmatter, action items, and the prose after them. */
struct Note {
    QString id;
    QString type;
    QString date;
    QString hours;
    QString dayType;
    QString layout;
    QStringList labels;

    /*! The action items, in file order and including nested ones. */
    QList<Item> items;
    /*! The markdown after the last action item, verbatim. */
    QString body;
    /*!
     * Whether the source had a frontmatter block, so a note without one does
     * not gain one just by being saved.
     */
    bool hadFrontmatter = false;

    /*! Every label on the note and its items, sorted and deduplicated. */
    QStringList allLabels() const;

    /*! The time logged against every item, in minutes. */
    int totalMinutes() const;

    /*!
     * The layout to render. A workplan is always both; an absent or
     * unrecognised value is both, so an older note never loses a section.
     */
    QString effectiveLayout() const;

    /*! The index of the item with this id, or -1. */
    int indexOfItem(const QString &id) const;

    /*! Appends an item and returns its index. The caller supplies id and clock. */
    int addItem(const QString &id, const QString &text, const QString &at, int depth);

    /*!
     * Ticks or unticks an item, stamping or clearing the completion time.
     * Unticking clears the stamp so a reopened item does not claim to be done.
     */
    bool setDone(const QString &id, bool done, const QString &at);

    /*!
     * Deletes an item and everything nested beneath it, since a child left
     * without its parent would silently change meaning.
     */
    bool removeItem(const QString &id);

    /*! Replaces an item's visible text, keeping its metadata. */
    bool setItemText(const QString &id, const QString &text);

    /*! Adds to the time logged against an item, never below zero. */
    bool addItemMinutes(const QString &id, int delta);

    /*!
     * Swaps the note's action items for the given list, keeping the
     * frontmatter and trailing body exactly as they were.
     *
     * Incoming items carry only what an editor knows — id, text, done, depth,
     * body. Everything else is metadata the editor never sees (original
     * creation time, carry counters, recurring id, completion stamp), so it is
     * looked up by id from the existing item and preserved. Ticking is treated
     * as a transition rather than a flag: the completion time is stamped when
     * an item becomes done and cleared when it is reopened, and an item that
     * stays done keeps its stamp.
     */
    void replaceItemsAt(const QList<Item> &items, const QString &at);

    /*!
     * Removes the items with the given ids — each together with the items
     * nested beneath it — and returns them in file order, with depth re-based
     * so a moved subtree keeps its shape at its new home. Unknown ids are
     * ignored. Ids, timestamps, carry counters and bodies travel untouched, so
     * an item moved into today's workplan still says when it was first added.
     */
    QList<Item> takeItems(const QStringList &ids);

    /*! Adds items to the end of the list, leaving the body in place. */
    void appendItems(const QList<Item> &items);
};

/*!
 * Reads a note. Carriage returns are dropped so a file edited on Windows parses
 * identically to one edited elsewhere.
 */
Note parse(const QString &src);

/*!
 * Writes a note back to markdown. Output is canonical: parsing and serialising
 * again yields exactly the same bytes.
 */
QString serialize(const Note &note);

/*!
 * Reads an "hh:mm" duration into minutes. Hours are zero padded to at least two
 * digits, which is the format the user writes. Returns false if it is not.
 */
bool parseDuration(const QString &text, int *minutes);

/*! Renders minutes as "hh:mm". */
QString formatDuration(int minutes);

} // namespace MdNote
