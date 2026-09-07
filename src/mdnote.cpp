/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "mdnote.h"

#include <QRegularExpression>
#include <QSet>

#include <optional>

using namespace Qt::StringLiterals;

namespace MdNote
{
namespace
{

// A markdown heading. Whether it is a group heading or the start of the body
// depends on what follows it (see isGroupHeadingAt).
const QRegularExpression &headingLine()
{
    static const QRegularExpression re(uR"(^(#{1,6})\s+(.*?)\s*$)"_s);
    return re;
}

const QRegularExpression &itemLine()
{
    static const QRegularExpression re(uR"(^([ \t]*)- \[([ xX])\] ?(.*)$)"_s);
    return re;
}

const QRegularExpression &metaBlock()
{
    static const QRegularExpression re(uR"(\s*<!--n ([^>]*)-->\s*$)"_s);
    return re;
}

const QRegularExpression &labelTok()
{
    static const QRegularExpression re(uR"(#([\p{L}\p{N}][\p{L}\p{N}_/-]*))"_s,
                                       QRegularExpression::UseUnicodePropertiesOption);
    return re;
}

const QRegularExpression &timeTok()
{
    static const QRegularExpression re(uR"(\[(\d{2}):([0-5]\d)\])"_s);
    return re;
}

const QRegularExpression &fenceLine()
{
    static const QRegularExpression re(uR"(^\s*(```|~~~))"_s);
    return re;
}

const QRegularExpression &durationRe()
{
    static const QRegularExpression re(uR"(^(\d{2,}):([0-5]\d)$)"_s);
    return re;
}

int indentOf(QStringView line)
{
    int n = 0;
    for (const QChar c : line) {
        if (c == u' ') {
            ++n;
        } else if (c == u'\t') {
            n += IndentUnit;
        } else {
            return n;
        }
    }
    return n;
}

/*!
 * Removes up to \a width leading spaces, leaving deeper indentation intact so
 * nested code keeps its shape.
 */
QString dedent(const QString &line, int width)
{
    int cut = 0;
    while (cut < width && cut < line.size() && line.at(cut) == u' ') {
        ++cut;
    }
    QString out = line.sliced(cut);
    while (!out.isEmpty() && (out.back() == u' ' || out.back() == u'\t')) {
        out.chop(1);
    }
    return out;
}

QStringList trimBlankEdges(QStringList lines)
{
    while (!lines.isEmpty() && lines.first().trimmed().isEmpty()) {
        lines.removeFirst();
    }
    while (!lines.isEmpty() && lines.last().trimmed().isEmpty()) {
        lines.removeLast();
    }
    return lines;
}

/*! Removes inline code spans so a # inside code is not read as a label. */
QString stripFences(const QString &s)
{
    QString out;
    out.reserve(s.size());
    bool inCode = false;
    for (const QChar c : s) {
        if (c == u'`') {
            inCode = !inCode;
            continue;
        }
        if (!inCode) {
            out.append(c);
        }
    }
    return out;
}

QString collapseSpaces(QString s)
{
    while (s.contains("  "_L1)) {
        s.replace("  "_L1, " "_L1);
    }
    return s;
}

QString trimTrailing(const QString &s, QLatin1StringView chars)
{
    qsizetype end = s.size();
    while (end > 0 && chars.contains(s.at(end - 1))) {
        --end;
    }
    return s.first(end);
}

/*!
 * Reports whether the heading on line \a i is followed, after any blank lines,
 * by an item line — or by another heading that is.
 */
bool isGroupHeadingAt(const QStringList &lines, qsizetype i)
{
    for (qsizetype j = i + 1; j < lines.size(); ++j) {
        if (lines.at(j).trimmed().isEmpty()) {
            continue;
        }
        if (itemLine().match(lines.at(j)).hasMatch()) {
            return true;
        }
        if (headingLine().match(lines.at(j)).hasMatch()) {
            return isGroupHeadingAt(lines, j);
        }
        return false;
    }
    return false;
}

/*! Separates the visible text from the trailing <!--n ...--> comment. */
void splitMeta(const QString &s, QString *text, QString *meta)
{
    const QRegularExpressionMatch m = metaBlock().match(s);
    if (!m.hasMatch()) {
        *text = s;
        meta->clear();
        return;
    }
    *text = s.first(m.capturedStart(0));
    *meta = m.captured(1).trimmed();
}

/*!
 * Repairs an hh:mm value, since splitting a field on its first colon leaves
 * only the hour behind.
 */
void rejoinClock(QString *dst, const QString &meta, QLatin1StringView key)
{
    const QRegularExpression re(uR"(\b)"_s + QString(key) + uR"(:(\d{2}:[0-5]\d)\b)"_s);
    const QRegularExpressionMatch m = re.match(meta);
    if (m.hasMatch()) {
        *dst = m.captured(1);
    }
}

void applyMeta(Item *it, const QString &meta)
{
    const QStringList fields = meta.split(QRegularExpression(uR"(\s+)"_s), Qt::SkipEmptyParts);
    for (const QString &field : fields) {
        const qsizetype colon = field.indexOf(u':');
        if (colon < 0) {
            continue;
        }
        const QString key = field.first(colon);
        const QString value = field.sliced(colon + 1);
        if (key == "id"_L1) {
            it->id = value;
        } else if (key == "t"_L1) {
            it->createdAt = value;
        } else if (key == "done"_L1) {
            it->doneAt = value;
        } else if (key == "from"_L1) {
            it->from = value;
        } else if (key == "rec"_L1) {
            it->recurring = value;
        } else if (key == "carried"_L1) {
            bool ok = false;
            const int n = value.toInt(&ok);
            if (ok) {
                it->carried = n;
            }
        }
    }
    // "t" and "done" are clock times, so a colon inside them was split above.
    // Re-join the minute part that the split removed.
    rejoinClock(&it->createdAt, meta, "t"_L1);
    rejoinClock(&it->doneAt, meta, "done"_L1);
}

/*!
 * Reads the frontmatter block.
 *
 * The writer below emits a fixed set of keys in a fixed order, so this reader
 * only has to understand that shape plus the hand-written variations of it: a
 * quoted or bare scalar, and labels as either a flow sequence or a block one.
 * Unknown keys are ignored, which matches what serialising has always done with
 * them.
 */
void parseFrontmatter(const QString &front, Note *note)
{
    const QStringList lines = front.split(u'\n');
    QString blockKey;

    const auto unquote = [](QString v) {
        v = v.trimmed();
        if (v.size() >= 2 && ((v.startsWith(u'"') && v.endsWith(u'"')) || (v.startsWith(u'\'') && v.endsWith(u'\'')))) {
            return v.sliced(1, v.size() - 2);
        }
        return v;
    };

    for (const QString &line : lines) {
        const QString trimmed = line.trimmed();
        if (trimmed.isEmpty() || trimmed.startsWith(u'#')) {
            continue;
        }

        // A "- value" line continues the block sequence the last key opened.
        if (trimmed.startsWith("- "_L1) || trimmed == "-"_L1) {
            if (blockKey == "labels"_L1) {
                const QString value = unquote(trimmed.sliced(1));
                if (!value.isEmpty()) {
                    note->labels.append(value);
                }
            }
            continue;
        }

        const qsizetype colon = trimmed.indexOf(u':');
        if (colon < 0) {
            continue;
        }
        const QString key = trimmed.first(colon).trimmed();
        const QString raw = trimmed.sliced(colon + 1).trimmed();
        blockKey = key;

        if (key == "labels"_L1) {
            note->labels.clear();
            if (raw.startsWith(u'[') && raw.endsWith(u']')) {
                const QStringList parts = raw.sliced(1, raw.size() - 2).split(u',');
                for (const QString &part : parts) {
                    const QString value = unquote(part);
                    if (!value.isEmpty()) {
                        note->labels.append(value);
                    }
                }
            }
            continue;
        }

        const QString value = unquote(raw);
        if (key == "id"_L1) {
            note->id = value;
        } else if (key == "type"_L1) {
            note->type = value;
        } else if (key == "date"_L1) {
            note->date = value;
        } else if (key == "hours"_L1) {
            note->hours = value;
        } else if (key == "daytype"_L1) {
            note->dayType = value;
        } else if (key == "layout"_L1) {
            note->layout = value;
        }
    }
}

/*!
 * Walks the body a line at a time, collecting action items with their bodies
 * and returning whatever trailing markdown is left over.
 */
void parseItems(const QString &src, QList<Item> *items, QString *body)
{
    items->clear();
    body->clear();
    if (src.isEmpty()) {
        return;
    }

    QString trimmed = src;
    if (trimmed.endsWith(u'\n')) {
        trimmed.chop(1);
    }
    const QStringList lines = trimmed.split(u'\n');

    std::optional<Item> current;
    QStringList currentBody;
    bool inFence = false;
    int curIndent = 0;

    const auto flush = [&] {
        if (!current.has_value()) {
            return;
        }
        current->body = trimBlankEdges(currentBody);
        items->append(*current);
        current.reset();
        currentBody.clear();
    };

    for (qsizetype i = 0; i < lines.size(); ++i) {
        const QString &line = lines.at(i);

        // A fence inside an item body swallows everything until it closes, so
        // checklist syntax used as example content is never read as an item.
        if (current.has_value() && fenceLine().match(line).hasMatch() && indentOf(line) >= curIndent + IndentUnit) {
            inFence = !inFence;
            currentBody.append(dedent(line, curIndent + BodyIndent));
            continue;
        }
        if (inFence) {
            currentBody.append(dedent(line, curIndent + BodyIndent));
            continue;
        }

        // A heading is a group heading when an item follows it; a heading that
        // leads into prose starts the body exactly as it always did.
        const QRegularExpressionMatch heading = headingLine().match(line);
        if (heading.hasMatch() && isGroupHeadingAt(lines, i)) {
            flush();
            Item it;
            it.kind = KindHeading;
            it.level = int(heading.captured(1).size());
            it.text = heading.captured(2);
            items->append(it);
            continue;
        }

        // Blank lines between groups are spacing, not the start of the body.
        if (!current.has_value() && !items->isEmpty() && line.trimmed().isEmpty()) {
            continue;
        }

        const QRegularExpressionMatch item = itemLine().match(line);
        if (item.hasMatch()) {
            flush();
            QString lead = item.captured(1);
            lead.replace(u'\t', QString(IndentUnit, u' '));
            const int indent = int(lead.size());

            QString text;
            QString meta;
            splitMeta(item.captured(3), &text, &meta);

            Item it;
            it.text = trimTrailing(text, " \t"_L1);
            it.done = item.captured(2) == "x"_L1 || item.captured(2) == "X"_L1;
            it.depth = indent / IndentUnit;
            applyMeta(&it, meta);

            current = it;
            curIndent = indent;
            continue;
        }

        if (current.has_value() && (line.trimmed().isEmpty() || indentOf(line) >= curIndent + IndentUnit)) {
            currentBody.append(dedent(line, curIndent + BodyIndent));
            continue;
        }

        // An unindented, non-item line ends the item list; the rest is body.
        flush();
        *body = lines.sliced(i).join(u'\n') + u'\n';
        return;
    }

    flush();
}

/*! Emits the known keys in a fixed order so saving never reshuffles them. */
QString frontmatter(const Note &n)
{
    QString out;
    if (!n.id.isEmpty()) {
        out += "id: "_L1 + n.id + u'\n';
    }
    if (!n.type.isEmpty()) {
        out += "type: "_L1 + n.type + u'\n';
    }
    if (!n.date.isEmpty()) {
        out += "date: "_L1 + n.date + u'\n';
    }
    if (!n.hours.isEmpty()) {
        // Quoted so YAML reads "09:00" as a string, not a sexagesimal number.
        out += "hours: \""_L1 + n.hours + "\"\n"_L1;
    }
    if (!n.dayType.isEmpty()) {
        out += "daytype: "_L1 + n.dayType + u'\n';
    }
    if (!n.layout.isEmpty()) {
        out += "layout: "_L1 + n.layout + u'\n';
    }
    if (!n.labels.isEmpty()) {
        out += "labels: ["_L1 + n.labels.join(", "_L1) + "]\n"_L1;
    }
    return out;
}

QString formatMeta(const Item &it)
{
    QStringList parts;
    if (!it.id.isEmpty()) {
        parts.append("id:"_L1 + it.id);
    }
    if (!it.createdAt.isEmpty()) {
        parts.append("t:"_L1 + it.createdAt);
    }
    if (!it.doneAt.isEmpty()) {
        parts.append("done:"_L1 + it.doneAt);
    }
    if (!it.from.isEmpty()) {
        parts.append("from:"_L1 + it.from);
    }
    if (it.carried > 0) {
        parts.append("carried:"_L1 + QString::number(it.carried));
    }
    if (!it.recurring.isEmpty()) {
        parts.append("rec:"_L1 + it.recurring);
    }
    if (parts.isEmpty()) {
        return {};
    }
    return "<!--n "_L1 + parts.join(u' ') + "-->"_L1;
}

} // namespace

QStringList Item::labels() const
{
    QStringList out;
    QRegularExpressionMatchIterator it = labelTok().globalMatch(stripFences(text));
    while (it.hasNext()) {
        out.append(it.next().captured(1));
    }
    return out;
}

int Item::minutes() const
{
    const QRegularExpressionMatch m = timeTok().match(text);
    if (!m.hasMatch()) {
        return 0;
    }
    return m.captured(1).toInt() * 60 + m.captured(2).toInt();
}

void Item::setMinutes(int mins)
{
    if (mins <= 0) {
        text = collapseSpaces(QString(text).remove(timeTok())).trimmed();
        return;
    }
    const QString token = u'[' + formatDuration(mins) + u']';
    if (timeTok().match(text).hasMatch()) {
        text.replace(timeTok(), token);
        return;
    }
    text = trimTrailing(text, " "_L1) + u' ' + token;
}

QStringList Note::allLabels() const
{
    QSet<QString> seen(labels.cbegin(), labels.cend());
    for (const Item &it : items) {
        if (it.isHeading()) {
            continue;
        }
        const QStringList itemLabels = it.labels();
        seen.unite(QSet<QString>(itemLabels.cbegin(), itemLabels.cend()));
    }
    QStringList out(seen.cbegin(), seen.cend());
    out.sort();
    return out;
}

int Note::totalMinutes() const
{
    int total = 0;
    for (const Item &it : items) {
        if (it.isHeading()) {
            continue;
        }
        total += it.minutes();
    }
    return total;
}

QString Note::effectiveLayout() const
{
    if (type == TypeWorkplan) {
        return LayoutBoth;
    }
    if (layout == LayoutItems || layout == LayoutNotes) {
        return layout;
    }
    return LayoutBoth;
}

int Note::indexOfItem(const QString &wanted) const
{
    for (qsizetype i = 0; i < items.size(); ++i) {
        if (items.at(i).id == wanted) {
            return int(i);
        }
    }
    return -1;
}

int Note::addItem(const QString &newId, const QString &text, const QString &at, int depth)
{
    Item it;
    it.id = newId;
    it.text = text.trimmed();
    it.createdAt = at;
    it.depth = depth;
    items.append(it);
    return int(items.size()) - 1;
}

bool Note::setDone(const QString &wanted, bool done, const QString &at)
{
    const int i = indexOfItem(wanted);
    if (i < 0) {
        return false;
    }
    items[i].done = done;
    items[i].doneAt = done ? at : QString();
    return true;
}

bool Note::removeItem(const QString &wanted)
{
    const int i = indexOfItem(wanted);
    if (i < 0) {
        return false;
    }
    qsizetype end = i + 1;
    while (end < items.size() && items.at(end).depth > items.at(i).depth) {
        ++end;
    }
    items.remove(i, end - i);
    return true;
}

bool Note::setItemText(const QString &wanted, const QString &text)
{
    const int i = indexOfItem(wanted);
    if (i < 0) {
        return false;
    }
    items[i].text = text.trimmed();
    return true;
}

bool Note::addItemMinutes(const QString &wanted, int delta)
{
    const int i = indexOfItem(wanted);
    if (i < 0) {
        return false;
    }
    items[i].setMinutes(std::max(0, items.at(i).minutes() + delta));
    return true;
}

void Note::replaceItemsAt(const QList<Item> &incoming, const QString &at)
{
    QHash<QString, Item> existing;
    existing.reserve(items.size());
    for (const Item &it : items) {
        if (!it.id.isEmpty()) {
            existing.insert(it.id, it);
        }
    }

    QList<Item> out;
    out.reserve(incoming.size());
    for (const Item &in : incoming) {
        if (in.isHeading()) {
            Item heading;
            heading.kind = KindHeading;
            heading.level = (in.level < 1 || in.level > 6) ? 2 : in.level;
            heading.text = in.text.trimmed();
            out.append(heading);
            continue;
        }

        Item it;
        it.id = in.id;
        it.text = in.text.trimmed();
        it.done = in.done;
        it.depth = std::max(0, in.depth);
        it.body = in.body;
        if (!in.createdAt.isEmpty()) {
            it.createdAt = in.createdAt;
        }

        const auto found = existing.constFind(in.id);
        if (found != existing.cend()) {
            const Item &prev = *found;
            it.createdAt = prev.createdAt;
            it.from = prev.from;
            it.carried = prev.carried;
            it.recurring = prev.recurring;
            if (in.done && prev.done) {
                it.doneAt = prev.doneAt;
            } else if (in.done && !prev.done) {
                it.doneAt = at;
            }
        } else if (in.done) {
            it.doneAt = at;
        }
        out.append(it);
    }
    items = out;
}

QList<Item> Note::takeItems(const QStringList &ids)
{
    QSet<QString> want;
    for (const QString &id : ids) {
        if (!id.isEmpty()) {
            want.insert(id);
        }
    }

    QList<Item> taken;
    QList<Item> kept;
    for (qsizetype i = 0; i < items.size();) {
        const Item &it = items.at(i);
        if (it.isHeading() || !want.contains(it.id)) {
            kept.append(it);
            ++i;
            continue;
        }
        const int base = it.depth;
        qsizetype end = i + 1;
        while (end < items.size() && items.at(end).depth > base) {
            ++end;
        }
        for (qsizetype j = i; j < end; ++j) {
            Item moved = items.at(j);
            moved.depth -= base;
            taken.append(moved);
        }
        i = end;
    }
    items = kept;
    return taken;
}

void Note::appendItems(const QList<Item> &more)
{
    items.append(more);
}

Note parse(const QString &source)
{
    QString src = source;
    src.replace("\r\n"_L1, "\n"_L1);
    src.replace(u'\r', u'\n');

    Note note;
    QString rest = src;

    if (src.startsWith("---\n"_L1)) {
        qsizetype end = src.sliced(4).indexOf("\n---\n"_L1);
        qsizetype closing = 5;
        if (end < 0) {
            // A frontmatter block may also close at end of file with no body.
            if (src.endsWith("\n---\n"_L1)) {
                end = src.size() - 4 - 5;
                closing = 5;
            } else if (src.endsWith("\n---"_L1)) {
                end = src.size() - 4 - 4;
                closing = 4;
            }
        }
        if (end >= 0) {
            parseFrontmatter(src.sliced(4, end), &note);
            note.hadFrontmatter = true;
            rest = src.sliced(4 + end + closing);
        }
    }

    while (rest.startsWith(u'\n')) {
        rest.remove(0, 1);
    }
    parseItems(rest, &note.items, &note.body);
    return note;
}

QString serialize(const Note &n)
{
    QString out;

    if (n.hadFrontmatter || !n.id.isEmpty() || !n.type.isEmpty() || !n.date.isEmpty() || !n.hours.isEmpty()
        || !n.dayType.isEmpty() || !n.layout.isEmpty() || !n.labels.isEmpty()) {
        out += "---\n"_L1;
        out += frontmatter(n);
        out += "---\n"_L1;
        if (!n.items.isEmpty() || !n.body.isEmpty()) {
            out += u'\n';
        }
    }

    for (qsizetype i = 0; i < n.items.size(); ++i) {
        const Item &it = n.items.at(i);

        if (it.isHeading()) {
            const int level = (it.level < 1 || it.level > 6) ? 2 : it.level;
            // A heading sits in its own paragraph: one blank line before
            // (unless it opens the list) and one after.
            if (!out.isEmpty() && !out.endsWith("\n\n"_L1)) {
                out += u'\n';
            }
            out += QString(level, u'#') + u' ' + it.text + "\n\n"_L1;
            continue;
        }

        out += QString(it.depth * IndentUnit, u' ');
        out += "- ["_L1 + QString(it.done ? u'x' : u' ') + "] "_L1 + it.text;
        const QString meta = formatMeta(it);
        if (!meta.isEmpty()) {
            out += u' ' + meta;
        }
        out += u'\n';

        if (!it.body.isEmpty()) {
            const QString pad(it.depth * IndentUnit + BodyIndent, u' ');
            for (const QString &line : it.body) {
                if (line.isEmpty()) {
                    out += u'\n';
                    continue;
                }
                out += pad + line + u'\n';
            }
            // A body reads as a block, so it is followed by a blank line
            // unless the note ends here.
            if (i < n.items.size() - 1 || !n.body.isEmpty()) {
                out += u'\n';
            }
        }
    }

    if (!n.body.isEmpty()) {
        // Exactly one blank line separates the item list from the trailing
        // prose, however the source happened to space them.
        if (!n.items.isEmpty() && !out.endsWith("\n\n"_L1)) {
            out += u'\n';
        }
        out += n.body;
    }
    return out;
}

bool parseDuration(const QString &text, int *minutes)
{
    const QRegularExpressionMatch m = durationRe().match(text);
    if (!m.hasMatch()) {
        return false;
    }
    if (minutes) {
        *minutes = m.captured(1).toInt() * 60 + m.captured(2).toInt();
    }
    return true;
}

QString formatDuration(int minutes)
{
    const int total = std::max(0, minutes);
    return QStringLiteral("%1:%2")
        .arg(total / 60, 2, 10, QLatin1Char('0'))
        .arg(total % 60, 2, 10, QLatin1Char('0'));
}

} // namespace MdNote
