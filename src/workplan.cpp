/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "workplan.h"

#include "vault.h"

#include <KLocalizedString>

#include <QDateTime>
#include <QRandomGenerator>
#include <QRegularExpression>
#include <QSet>

using namespace Qt::StringLiterals;

namespace Workplan
{
namespace
{

const QRegularExpression &cadenceToken()
{
    static const QRegularExpression re(uR"(\s*@(daily|weekdays|weekly:(?:mon|tue|wed|thu|fri|sat|sun))\b)"_s);
    return re;
}

bool isWeekend(QDate day)
{
    return day.dayOfWeek() == 6 || day.dayOfWeek() == 7;
}

QString dayTypeFor(QDate day)
{
    return isWeekend(day) ? QString(DayWeekend) : QString(DayWork);
}

QString stripCadence(const QString &text)
{
    return QString(text).remove(cadenceToken()).trimmed();
}

QString slug(const QString &text)
{
    static const QRegularExpression nonSlug(uR"([^a-z0-9]+)"_s);
    QString out = text.toLower();
    out.replace(nonSlug, "-"_L1);
    while (out.startsWith(u'-')) {
        out.remove(0, 1);
    }
    while (out.endsWith(u'-')) {
        out.chop(1);
    }
    if (out.size() > 40) {
        out = out.first(40);
    }
    return out.isEmpty() ? u"item"_s : out;
}

} // namespace

bool isValidDayType(const QString &dayType)
{
    return dayType == DayWork || dayType == DayWeekend || dayType == DayLeave || dayType == DayHoliday;
}

QString newId()
{
    // Crockford base32, as ULID specifies: no I, L, O or U, so an id read aloud
    // or typed by hand cannot become a different one.
    static constexpr QLatin1StringView alphabet{"0123456789ABCDEFGHJKMNPQRSTVWXYZ"};

    quint64 timestamp = quint64(QDateTime::currentMSecsSinceEpoch()) & 0xFFFFFFFFFFFFULL;
    QString out(26, u'0');

    // The first 10 characters are the 48-bit millisecond timestamp, most
    // significant first, which is what makes ids sort by creation time.
    for (int i = 9; i >= 0; --i) {
        out[i] = QChar(alphabet[int(timestamp & 0x1F)]);
        timestamp >>= 5;
    }
    for (int i = 10; i < 26; ++i) {
        out[i] = QChar(alphabet[int(QRandomGenerator::global()->bounded(32))]);
    }
    return out;
}

bool Template::dueOn(QDate day) const
{
    if (cadence == "daily"_L1) {
        return true;
    }
    if (cadence == "weekdays"_L1) {
        return !isWeekend(day);
    }
    if (cadence.startsWith("weekly:"_L1)) {
        static const QHash<QString, int> weekdays = {
            {u"mon"_s, 1}, {u"tue"_s, 2}, {u"wed"_s, 3}, {u"thu"_s, 4},
            {u"fri"_s, 5}, {u"sat"_s, 6}, {u"sun"_s, 7},
        };
        const auto found = weekdays.constFind(cadence.sliced(7));
        return found != weekdays.cend() && day.dayOfWeek() == *found;
    }
    return false;
}

Manager::Manager(Vault *vault, Options options)
    : m_vault(vault)
    , m_options(std::move(options))
{
    if (m_options.folder.isEmpty()) {
        m_options.folder = u"Workplans"_s;
    }
    if (!m_options.newId) {
        m_options.newId = &Workplan::newId;
    }
}

QString Manager::pathFor(QDate day) const
{
    return m_options.folder + u'/' + day.toString(DateFormat) + MdNote::Ext;
}

QString Manager::ensure(QDate day)
{
    if (!m_options.createOnWeekends && isWeekend(day)) {
        return {};
    }

    const QString path = pathFor(day);
    if (m_vault->exists(path)) {
        // A day the user has already worked on is safe from every later call.
        return path;
    }

    MdNote::Note note;
    note.type = QString(MdNote::TypeWorkplan);
    note.date = day.toString(DateFormat);
    note.hours = MdNote::formatDuration(0);
    note.dayType = dayTypeFor(day);
    note.hadFrontmatter = true;

    note.items = carryForward(day);

    // What repeats sits at the top, above the day's own work.
    QList<MdNote::Item> seeded = seedRecurring(day, note.items);
    seeded.append(note.items);
    note.items = seeded;

    if (!m_vault->writeNote(path, note)) {
        m_lastError = m_vault->lastError();
        return {};
    }
    return path;
}

void Manager::previousWorkplan(QDate day, QString *path, QString *date) const
{
    path->clear();
    date->clear();

    const QString prefix = m_options.folder + u'/';
    const QString today = day.toString(DateFormat);

    // Deliberately the most recent workplan rather than literally yesterday, so
    // weekends, leave and holidays do not break the chain.
    QString best;
    const QStringList paths = m_vault->listNotes();
    for (const QString &p : paths) {
        if (!p.startsWith(prefix)) {
            continue;
        }
        const QString name = p.sliced(prefix.size()).chopped(MdNote::Ext.size());
        if (!QDate::fromString(name, DateFormat).isValid()) {
            continue;
        }
        if (name >= today) {
            continue;
        }
        if (name > best) {
            best = name;
            *path = p;
        }
    }
    *date = best;
}

QList<MdNote::Item> Manager::carryForward(QDate day)
{
    QString prevPath;
    QString prevDate;
    previousWorkplan(day, &prevPath, &prevDate);
    if (prevPath.isEmpty()) {
        return {};
    }

    const auto previous = m_vault->readNote(prevPath);
    if (!previous) {
        return {};
    }

    QList<MdNote::Item> out;
    std::optional<MdNote::Item> pendingHeading;

    for (const MdNote::Item &item : previous->items) {
        if (item.isHeading()) {
            // Held back until something under it turns out to carry, so a group
            // that was entirely finished disappears with its heading.
            pendingHeading = item;
            continue;
        }
        if (!item.recurring.isEmpty()) {
            // A repeating item starts each day fresh, ticked or not. Carrying
            // it would bring yesterday's tick or a growing carry badge with it,
            // and seedRecurring puts a clean copy at the top instead.
            continue;
        }
        if (item.done) {
            // Completed work stays on the day it was completed.
            continue;
        }

        if (pendingHeading) {
            out.append(*pendingHeading);
            pendingHeading.reset();
        }

        // The item keeps its identity, its original creation time and anything
        // logged against it; only the completion state and carry counters move.
        MdNote::Item carried = item;
        carried.doneAt.clear();
        if (carried.from.isEmpty()) {
            carried.from = prevDate;
        }
        ++carried.carried;
        out.append(carried);
    }
    return out;
}

QList<MdNote::Item> Manager::seedRecurring(QDate day, const QList<MdNote::Item> &existing)
{
    const QList<Template> all = templates();

    // Matching on the recurring id means an item carried over from yesterday
    // suppresses today's seed, and one the user deleted for today stays
    // deleted for today.
    QSet<QString> present;
    for (const MdNote::Item &item : existing) {
        if (!item.recurring.isEmpty()) {
            present.insert(item.recurring);
        }
    }

    // Seeded items are stamped with the clock time ensure() ran at, which for
    // the usual midnight rollover is the start of the day.
    const QString stamp = QDateTime(day, QTime::currentTime()).toString(u"HH:mm"_s);

    QList<MdNote::Item> out;
    for (const Template &tpl : all) {
        if (present.contains(tpl.id) || !tpl.dueOn(day)) {
            continue;
        }
        MdNote::Item item;
        // A seeded item needs its own id like any other: without one the editor
        // cannot address it to tick, retext or delete it.
        item.id = m_options.newId();
        item.text = tpl.text;
        item.createdAt = stamp;
        item.recurring = tpl.id;
        out.append(item);
    }
    return out;
}

QList<Template> Manager::templates()
{
    const auto raw = m_vault->readRaw(QString(TemplatePath));
    if (!raw) {
        // No template file simply means no recurring items are configured.
        return {};
    }

    MdNote::Note note = MdNote::parse(*raw);

    QList<Template> out;
    bool changed = false;

    for (MdNote::Item &item : note.items) {
        if (item.isHeading()) {
            continue;
        }
        QString cadence = u"daily"_s;
        const QRegularExpressionMatch m = cadenceToken().match(item.text);
        if (m.hasMatch()) {
            cadence = m.captured(1);
        }
        const QString text = stripCadence(item.text);
        if (text.isEmpty()) {
            continue;
        }
        if (item.recurring.isEmpty()) {
            item.recurring = slug(text);
            changed = true;
        }
        out.append(Template{item.recurring, text, cadence});
    }

    if (changed) {
        // Best effort: the ids are a convenience, and a read-only vault should
        // not stop recurring items from working.
        m_vault->writeNote(QString(TemplatePath), note);
    }
    return out;
}

std::optional<MdNote::Note> Manager::templateNote()
{
    // templates() runs first because it writes back any missing ids, which is
    // what lets every mutation below address a line by a stable id.
    templates();

    const auto raw = m_vault->readRaw(QString(TemplatePath));
    if (!raw) {
        return MdNote::Note{};
    }
    return MdNote::parse(*raw);
}

bool Manager::addTemplate(const QString &rawText, Template *added)
{
    const QString text = stripCadence(rawText);
    if (text.isEmpty()) {
        m_lastError = i18n("A repeating item needs some text.");
        return false;
    }
    auto note = templateNote();
    if (!note) {
        return false;
    }

    Template tpl{m_options.newId(), text, u"daily"_s};

    MdNote::Item item;
    item.text = text;
    item.recurring = tpl.id;
    note->items.append(item);

    if (!m_vault->writeNote(QString(TemplatePath), *note)) {
        m_lastError = m_vault->lastError();
        return false;
    }
    if (added) {
        *added = tpl;
    }
    return true;
}

bool Manager::removeTemplate(const QString &id)
{
    auto note = templateNote();
    if (!note) {
        return false;
    }

    QList<MdNote::Item> kept;
    kept.reserve(note->items.size());
    for (const MdNote::Item &item : std::as_const(note->items)) {
        if (item.recurring == id) {
            continue;
        }
        kept.append(item);
    }
    if (kept.size() == note->items.size()) {
        return true;
    }
    note->items = kept;

    if (!m_vault->writeNote(QString(TemplatePath), *note)) {
        m_lastError = m_vault->lastError();
        return false;
    }
    return true;
}

bool Manager::renameTemplate(const QString &id, const QString &rawText)
{
    const QString text = stripCadence(rawText);
    if (text.isEmpty()) {
        m_lastError = i18n("A repeating item needs some text.");
        return false;
    }
    auto note = templateNote();
    if (!note) {
        return false;
    }

    bool changed = false;
    for (MdNote::Item &item : note->items) {
        if (item.recurring != id) {
            continue;
        }
        // The cadence lives in the text as an @token; keep whatever was there.
        const QString cadence = cadenceToken().match(item.text).captured(0);
        const QString next = text + cadence;
        if (next != item.text) {
            item.text = next;
            changed = true;
        }
    }
    if (!changed) {
        return true;
    }

    if (!m_vault->writeNote(QString(TemplatePath), *note)) {
        m_lastError = m_vault->lastError();
        return false;
    }
    return true;
}

bool Manager::seedInto(QDate day)
{
    const QString path = pathFor(day);
    auto note = m_vault->readNote(path);
    if (!note) {
        m_lastError = m_vault->lastError();
        return false;
    }

    QList<MdNote::Item> seeded = seedRecurring(day, note->items);
    if (seeded.isEmpty()) {
        return true;
    }
    seeded.append(note->items);
    note->items = seeded;

    if (!m_vault->writeNote(path, *note)) {
        m_lastError = m_vault->lastError();
        return false;
    }
    return true;
}

bool Manager::dropFrom(QDate day, const QString &id)
{
    const QString path = pathFor(day);
    if (!m_vault->exists(path)) {
        return true;
    }
    auto note = m_vault->readNote(path);
    if (!note) {
        m_lastError = m_vault->lastError();
        return false;
    }

    QList<MdNote::Item> kept;
    kept.reserve(note->items.size());
    for (const MdNote::Item &item : std::as_const(note->items)) {
        if (item.recurring == id) {
            continue;
        }
        kept.append(item);
    }
    if (kept.size() == note->items.size()) {
        return true;
    }
    note->items = kept;

    if (!m_vault->writeNote(path, *note)) {
        m_lastError = m_vault->lastError();
        return false;
    }
    return true;
}

bool Manager::mutate(const QString &path, const std::function<void(MdNote::Note &)> &apply)
{
    auto note = m_vault->readNote(path);
    if (!note) {
        m_lastError = m_vault->lastError();
        return false;
    }
    apply(*note);
    if (!m_vault->writeNote(path, *note)) {
        m_lastError = m_vault->lastError();
        return false;
    }
    return true;
}

bool Manager::setHours(const QString &path, const QString &hours)
{
    if (!MdNote::parseDuration(hours, nullptr)) {
        m_lastError = i18n("Hours must be written as hh:mm, for example 07:30.");
        return false;
    }
    return mutate(path, [&hours](MdNote::Note &note) {
        note.hours = hours;
    });
}

bool Manager::setDayType(const QString &path, const QString &dayType)
{
    if (!isValidDayType(dayType)) {
        m_lastError = i18n("Unknown day type: %1", dayType);
        return false;
    }
    return mutate(path, [&dayType](MdNote::Note &note) {
        note.dayType = dayType;
    });
}

int Manager::suggestedMinutes(const QString &path) const
{
    const auto note = m_vault->readNote(path);
    return note ? note->totalMinutes() : 0;
}

} // namespace Workplan
