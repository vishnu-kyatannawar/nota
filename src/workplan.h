/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * Owns the dated daily notes.
 *
 * One note per day lives in a reserved folder, named for its date. When a new
 * day starts, unfinished items roll forward from the most recent previous day
 * and completed ones are left behind on the day they were completed — that
 * carry-forward is the behaviour the whole application exists for.
 *
 * Everything here is idempotent. ensure() runs on launch, on a midnight timer,
 * and whenever the window regains focus, so running it repeatedly must never
 * duplicate an item or disturb a day the user has already edited.
 */

#pragma once

#include "mdnote.h"

#include <QDate>
#include <QString>

#include <functional>

class Vault;

namespace Workplan
{

/*! How a workplan note is named and dated. */
inline constexpr QLatin1StringView DateFormat{"yyyy-MM-dd"};

/*! The vault-relative file holding the recurring items. */
inline constexpr QLatin1StringView TemplatePath{".nota/templates/recurring.md"};

/*!
 * Day types. Anything other than work pins the day's hours at zero by default,
 * though the user may still log time against it.
 */
inline constexpr QLatin1StringView DayWork{"work"};
inline constexpr QLatin1StringView DayWeekend{"weekend"};
inline constexpr QLatin1StringView DayLeave{"leave"};
inline constexpr QLatin1StringView DayHoliday{"holiday"};

/*! Whether \a dayType is one of the four known values. */
bool isValidDayType(const QString &dayType);

/*! One recurring item and how often it appears. */
struct Template {
    QString id;
    QString text;
    QString cadence; //!< "daily", "weekdays" or "weekly:<dow>"

    /*! Whether the template should appear on a given day. */
    bool dueOn(QDate day) const;
};

/*! Mints an id. Injectable so tests can pin them down. */
using IdGenerator = std::function<QString()>;

/*! A ULID: sortable by creation time, which is why the ids read in order. */
QString newId();

/*! Creates and rolls over the daily notes. */
class Manager
{
public:
    struct Options {
        /*! The vault-relative reserved folder for daily notes. */
        QString folder = QStringLiteral("Workplans");
        /*! Whether Saturday and Sunday get a note at all. */
        bool createOnWeekends = true;
        /*! Defaults to a ULID. */
        IdGenerator newId = &Workplan::newId;
    };

    Manager(Vault *vault, Options options);

    /*! The vault-relative path of a given day's note. */
    QString pathFor(QDate day) const;

    /*!
     * Makes sure the note for \a day exists, creating it by rolling the
     * previous workplan forward and seeding any recurring items that are due.
     *
     * Returns the note's path, or an empty string when the day is a weekend
     * and weekend notes are switched off. An existing note is never touched,
     * so a day the user has already worked on is safe from every later call.
     */
    QString ensure(QDate day);

    /*!
     * The recurring items. A missing file simply means none are configured.
     * Each template needs a stable id so seeding can tell whether today
     * already has it; one is derived from the text and written back on first
     * read, so the file stays hand-editable without the user having to invent
     * ids.
     */
    QList<Template> templates();

    /*!
     * Adds an item that repeats every day.
     *
     * The id is minted rather than derived from the text, so renaming the item
     * later cannot turn it into a different one — which would seed a duplicate
     * alongside the original.
     */
    bool addTemplate(const QString &text, Template *added);

    /*!
     * Stops an item repeating. Days already written keep their copy; only the
     * template goes, so tomorrow has nothing to seed from.
     */
    bool removeTemplate(const QString &id);

    /*!
     * Changes what a repeating item says, keeping its identity and how often
     * it repeats.
     */
    bool renameTemplate(const QString &id, const QString &text);

    /*!
     * Adds any repeating item due on this day that the day does not already
     * hold, at the top. ensure() only seeds when it creates the note, so this
     * is what makes a newly added repeating item show up today, not tomorrow.
     */
    bool seedInto(QDate day);

    /*!
     * Removes the item seeded by a template from one day's note. Only that day
     * is opened, so every earlier workplan keeps its copy exactly as written.
     */
    bool dropFrom(QDate day, const QString &id);

    /*! Records the hours worked on a day. Refuses anything but "hh:mm". */
    bool setHours(const QString &path, const QString &hours);

    /*!
     * Marks a day as work, weekend, leave or holiday. Marking it pins the
     * default at zero hours but does not erase time already logged, since a
     * person may genuinely have worked on a day off.
     */
    bool setDayType(const QString &path, const QString &dayType);

    /*!
     * Totals the time logged against a day's items. Only ever a suggestion to
     * pre-fill the field: plenty of a working day — meetings, calls, helping
     * someone — never lands on an action item, so the day total stays a value
     * the user owns.
     */
    int suggestedMinutes(const QString &path) const;

    QString lastError() const
    {
        return m_lastError;
    }

private:
    QList<MdNote::Item> carryForward(QDate day);
    void previousWorkplan(QDate day, QString *path, QString *date) const;
    QList<MdNote::Item> seedRecurring(QDate day, const QList<MdNote::Item> &existing);
    std::optional<MdNote::Note> templateNote();
    bool mutate(const QString &path, const std::function<void(MdNote::Note &)> &apply);

    Vault *m_vault;
    Options m_options;
    QString m_lastError;
};

} // namespace Workplan
