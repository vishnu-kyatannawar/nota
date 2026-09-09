/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "app.h"

#include "foldertreemodel.h"
#include "itemmodel.h"
#include "update.h"
#include "vault.h"

#include <KLocalizedString>

#include <QCoreApplication>
#include <QDateTime>
#include <QDir>
#include <QTimer>

using namespace Qt::StringLiterals;

namespace
{
QString startupVault;
}

void Nota::setStartupVault(const QString &path)
{
    startupVault = path;
}

Nota *Nota::create(QQmlEngine *, QJSEngine *)
{
    auto *instance = new Nota;
    // The engine owns singletons it creates through the factory.
    QJSEngine::setObjectOwnership(instance, QJSEngine::CppOwnership);
    return instance;
}

Nota::Nota(QObject *parent)
    : QObject(parent)
    , m_folderTree(std::make_unique<FolderTreeModel>())
    , m_items(std::make_unique<ItemModel>())
{
    m_settings = NotaSettings::resolve();
    if (!startupVault.isEmpty()) {
        m_settings.vaultPath = NotaSettings::expandHome(startupVault);
    }

    m_vault = std::make_unique<Vault>(m_settings.vaultPath);
    if (!m_vault->isOpen()) {
        fail(m_vault->lastError());
        return;
    }

    Workplan::Manager::Options options;
    options.folder = m_settings.workplanFolder;
    options.createOnWeekends = m_settings.createOnWeekends;
    m_plans = std::make_unique<Workplan::Manager>(m_vault.get(), options);

    m_folderTree->setVault(m_vault.get(), m_settings.workplanFolder);

    // 400 ms for items, 600 ms for prose: the same figures the previous build
    // settled on, and prose is typed in longer runs so it earns the slower one.
    m_itemSave = new QTimer(this);
    m_itemSave->setSingleShot(true);
    m_itemSave->setInterval(400);
    connect(m_itemSave, &QTimer::timeout, this, &Nota::saveNow);

    m_bodySave = new QTimer(this);
    m_bodySave->setSingleShot(true);
    m_bodySave->setInterval(600);
    connect(m_bodySave, &QTimer::timeout, this, &Nota::saveNow);

    connect(m_items.get(), &ItemModel::changed, this, [this] {
        armSave(m_itemSave, &m_itemsDirty);
    });

    // An external edit reloads the open page; a new or removed file rebuilds
    // the sidebar. The vault suppresses the events our own writes cause.
    connect(m_vault.get(), &Vault::treeChanged, m_folderTree.get(), &FolderTreeModel::refresh);
    connect(m_vault.get(), &Vault::noteChanged, this, [this](const QString &path) {
        m_folderTree->refresh();
        // Reloading over unsaved edits would discard what the user is typing.
        if (path == m_currentPath && !isDirty()) {
            reload();
        }
    });
    connect(m_vault.get(), &Vault::noteRemoved, this, [this](const QString &path) {
        // The prefix case is a folder deleted outside the application: the
        // page inside it is just as gone as one deleted directly, and leaving
        // it open lets the next debounce write the file back.
        if (path == m_currentPath || m_currentPath.startsWith(path + u'/')) {
            closeCurrent();
        }
    });
    m_vault->startWatching();

    openToday();
    scheduleMidnightRoll();

    if (m_settings.checkForUpdates) {
        checkForUpdate();
    }
}

Nota::~Nota() = default;

QString Nota::vaultPath() const
{
    return m_settings.vaultPath;
}

QString Nota::version() const
{
    return QStringLiteral(NOTA_VERSION_STRING);
}

bool Nota::canSelfUpdate() const
{
    return Update::kindFor(QCoreApplication::applicationFilePath(), QDir::homePath()) == Update::Kind::UserManaged;
}

bool Nota::isUpdateRunning() const
{
    return m_installer && m_installer->isRunning();
}

void Nota::checkForUpdate()
{
    if (!m_checker) {
        m_checker = std::make_unique<Update::Checker>();
        connect(m_checker.get(), &Update::Checker::found, this, [this](const Update::Release &release) {
            // Only ever forward. A build newer than the newest release is a
            // development build, and telling someone to downgrade is noise.
            if (Update::compare(release.version, version()) <= 0) {
                return;
            }
            m_updateVersion = release.version;
            m_updateUrl = release.url;
            Q_EMIT updateChanged();
        });
    }
    m_checker->check();
}

void Nota::startUpdate()
{
    if (isUpdateRunning()) {
        return;
    }
    if (!canSelfUpdate()) {
        fail(i18n("This copy of Nota was installed by your package manager, so update it there."));
        return;
    }

    if (!m_installer) {
        m_installer = std::make_unique<Update::Installer>();
        connect(m_installer.get(), &Update::Installer::output, this, &Nota::updateOutput);
        connect(m_installer.get(), &Update::Installer::finished, this, [this](bool ok, const QString &message) {
            Q_EMIT updateRunningChanged();
            Q_EMIT updateFinished(ok, message);
        });
    }

    // Whatever is being typed goes to disk first: the installer replaces the
    // binary under a window that keeps running until it is reopened.
    flush();
    m_installer->start();
    Q_EMIT updateRunningChanged();
}

void Nota::fail(const QString &message)
{
    m_errorMessage = message;
    Q_EMIT errorMessageChanged();
}

void Nota::clearError()
{
    if (m_errorMessage.isEmpty()) {
        return;
    }
    m_errorMessage.clear();
    Q_EMIT errorMessageChanged();
}

void Nota::open(const QString &path)
{
    if (path.isEmpty()) {
        return;
    }
    // A folder is a legitimate thing to click in a sidebar that shows folders.
    // Routing it here rather than letting readNote() fail is what stops "file
    // to open is a directory" reaching the user, whoever calls open().
    if (m_vault && m_vault->isFolder(path)) {
        openFolder(path);
        return;
    }
    if (path == m_currentPath) {
        return;
    }
    // Anything still pending belongs to the page being left, not the new one.
    flush();
    m_currentFolder.clear();
    m_currentPath = path;
    reload();
}

void Nota::openFolder(const QString &path)
{
    if (path.isEmpty() || path == m_currentFolder) {
        return;
    }
    flush();
    closeCurrent();
    m_currentFolder = path;
    Q_EMIT currentChanged();
}

void Nota::armSave(QTimer *timer, bool *flag)
{
    const bool was = isDirty();
    *flag = true;
    m_savePath = m_currentPath;
    timer->start();
    if (!was) {
        Q_EMIT dirtyChanged();
    }
}

void Nota::setBody(const QString &body)
{
    if (m_currentPath.isEmpty() || m_current.body == body) {
        return;
    }
    m_current.body = body;
    armSave(m_bodySave, &m_bodyDirty);
}

void Nota::flush()
{
    if (!isDirty()) {
        return;
    }
    m_itemSave->stop();
    m_bodySave->stop();
    saveNow();
}

void Nota::saveNow()
{
    if (!isDirty()) {
        return;
    }
    m_itemSave->stop();
    m_bodySave->stop();

    const QString path = m_savePath;
    m_itemsDirty = false;
    m_bodyDirty = false;
    Q_EMIT dirtyChanged();

    if (path.isEmpty()) {
        return;
    }
    // The guard the previous build needed four commits to get right: a save
    // armed before a page switch must never write these items into the page
    // the user switched to.
    if (path != m_currentPath) {
        return;
    }

    const QString now = QTime::currentTime().toString(u"HH:mm"_s);

    // A row typed into the editor has no id yet, and without one nothing can
    // address it to tick, retext or delete it later. Minting happens here, at
    // the moment it first reaches the file.
    QList<MdNote::Item> edited = m_items->items();
    for (MdNote::Item &item : edited) {
        if (item.isHeading() || !item.id.isEmpty()) {
            continue;
        }
        item.id = Workplan::newId();
        if (item.createdAt.isEmpty()) {
            item.createdAt = now;
        }
    }

    // Metadata the editor never sees — creation time, carry counters, the
    // recurring id, the completion stamp — is looked up by id and preserved.
    m_current.replaceItemsAt(edited, now);

    if (!m_vault->writeNote(path, m_current)) {
        fail(m_vault->lastError());
        return;
    }
    // Take the bookkeeping back without a model reset: the user is very likely
    // still typing in one of these rows.
    m_items->mergeSaved(m_current.items);
    Q_EMIT currentChanged();
}

void Nota::reload()
{
    m_itemSave->stop();
    m_bodySave->stop();
    m_itemsDirty = false;
    m_bodyDirty = false;
    Q_EMIT dirtyChanged();

    const auto note = m_vault->readNote(m_currentPath);
    if (!note) {
        fail(m_vault->lastError());
        return;
    }
    m_current = *note;
    m_items->setItems(m_current.items);
    Q_EMIT currentChanged();
}

void Nota::openToday()
{
    if (!m_plans) {
        return;
    }
    const QString path = m_plans->ensure(QDate::currentDate());
    if (path.isEmpty()) {
        // Weekend notes are switched off, so today simply has no workplan.
        if (!m_plans->lastError().isEmpty()) {
            fail(m_plans->lastError());
        }
        return;
    }
    m_folderTree->refresh();
    open(path);
}

void Nota::scheduleMidnightRoll()
{
    // Re-armed to the next computed midnight each time rather than left on a
    // 24-hour interval, which drifts across a DST change and stops firing
    // altogether when the machine suspends.
    const QDateTime midnight(QDate::currentDate().addDays(1), QTime(0, 0, 30));
    const qint64 wait = QDateTime::currentDateTime().msecsTo(midnight);
    QTimer::singleShot(std::max<qint64>(wait, 1000), this, [this] {
        openToday();
        scheduleMidnightRoll();
    });
}

bool Nota::isWorkplan() const
{
    return m_current.type == MdNote::TypeWorkplan;
}

QString Nota::title() const
{
    if (m_currentPath.isEmpty()) {
        return {};
    }
    if (isWorkplan() && !m_current.date.isEmpty()) {
        // The filename stays the bare date; the hours are frontmatter, so
        // logging time never renames a file or churns git history.
        if (!m_current.hours.isEmpty()) {
            return m_current.date + " - "_L1 + m_current.hours;
        }
        return m_current.date;
    }
    const QString base = m_currentPath.section(u'/', -1);
    return base.endsWith(MdNote::Ext) ? base.chopped(MdNote::Ext.size()) : base;
}

QString Nota::subtitle() const
{
    return m_currentPath;
}

int Nota::openCount() const
{
    return m_items->openCount();
}

int Nota::doneCount() const
{
    return m_items->doneCount();
}

bool Nota::setHours(const QString &hours)
{
    if (m_currentPath.isEmpty() || !m_plans) {
        return false;
    }
    flush();
    if (!m_plans->setHours(m_currentPath, hours)) {
        fail(m_plans->lastError());
        return false;
    }
    reload();
    return true;
}

bool Nota::setDayType(const QString &dayType)
{
    if (m_currentPath.isEmpty() || !m_plans) {
        return false;
    }
    flush();
    if (!m_plans->setDayType(m_currentPath, dayType)) {
        fail(m_plans->lastError());
        return false;
    }
    reload();
    return true;
}

bool Nota::setLayout(const QString &layout)
{
    if (m_currentPath.isEmpty() || isWorkplan()) {
        // A workplan always shows both sections; there is nothing to choose.
        return false;
    }
    if (layout != MdNote::LayoutItems && layout != MdNote::LayoutNotes && layout != MdNote::LayoutBoth) {
        return false;
    }
    flush();
    m_current.layout = layout;
    m_current.hadFrontmatter = true;
    if (!m_vault->writeNote(m_currentPath, m_current)) {
        fail(m_vault->lastError());
        return false;
    }
    Q_EMIT currentChanged();
    return true;
}

bool Nota::isReserved(const QString &path) const
{
    const QString folder = m_settings.workplanFolder;
    return !folder.isEmpty() && path == folder;
}

bool Nota::isDatedPage(const QString &path) const
{
    const QString folder = m_settings.workplanFolder;
    if (folder.isEmpty() || !path.startsWith(folder + u'/')) {
        return false;
    }
    // Directly under the folder and named for a date. Anything else someone
    // filed in there is an ordinary page and renames like one.
    const QString name = path.sliced(folder.size() + 1);
    if (name.contains(u'/') || !name.endsWith(MdNote::Ext, Qt::CaseInsensitive)) {
        return false;
    }
    return QDate::fromString(name.chopped(MdNote::Ext.size()), Workplan::DateFormat).isValid();
}

QString Nota::createNote(const QString &folder)
{
    if (!m_vault) {
        return {};
    }
    const QString prefix = folder.isEmpty() ? QString() : folder + u'/';

    // A name that is already taken would silently replace someone's page.
    QString path;
    for (int n = 1; n < 1000; ++n) {
        const QString name = n == 1 ? i18n("Untitled") : i18nc("@item a numbered new page", "Untitled %1", n);
        path = prefix + name + MdNote::Ext;
        if (!m_vault->exists(path)) {
            break;
        }
        path.clear();
    }
    if (path.isEmpty()) {
        return {};
    }

    MdNote::Note note;
    if (!m_vault->writeNote(path, note)) {
        fail(m_vault->lastError());
        return {};
    }
    m_folderTree->refresh();
    open(path);
    return path;
}

QString Nota::createFolder(const QString &parent, const QString &name)
{
    const QString clean = name.trimmed();
    if (clean.isEmpty() || clean.contains(u'/')) {
        fail(i18n("A folder name cannot be empty or contain a slash."));
        return {};
    }
    const QString path = parent.isEmpty() ? clean : parent + u'/' + clean;
    if (!m_vault->createFolder(path)) {
        fail(m_vault->lastError());
        return {};
    }
    return path;
}

bool Nota::renamePath(const QString &path, const QString &newName)
{
    const QString clean = newName.trimmed();
    if (clean.isEmpty() || clean.contains(u'/')) {
        fail(i18n("A name cannot be empty or contain a slash."));
        return false;
    }
    if (isReserved(path)) {
        fail(i18n("The workplan folder is reserved and cannot be renamed."));
        return false;
    }
    if (isDatedPage(path)) {
        fail(i18n("A workplan is named for its date, so it cannot be renamed."));
        return false;
    }

    const bool isNote = path.endsWith(MdNote::Ext, Qt::CaseInsensitive);
    const QString parent = path.contains(u'/') ? path.section(u'/', 0, -2) + u'/' : QString();
    const QString target = parent + clean + (isNote ? QString(MdNote::Ext) : QString());
    if (target == path) {
        return true;
    }

    flush();
    if (!m_vault->rename(path, target)) {
        fail(m_vault->lastError());
        return false;
    }
    if (m_currentPath == path) {
        m_currentPath = target;
        reload();
    }
    return true;
}

bool Nota::removePath(const QString &path)
{
    if (isReserved(path)) {
        fail(i18n("The workplan folder is reserved and cannot be deleted."));
        return false;
    }
    if (!m_vault->remove(path)) {
        fail(m_vault->lastError());
        return false;
    }
    if (m_currentPath == path || m_currentPath.startsWith(path + u'/')) {
        closeCurrent();
    }
    return true;
}

void Nota::closeCurrent()
{
    m_currentFolder.clear();
    // The page is gone, so an edit still sitting on a debounce has nowhere to
    // go. Dropping it is what stops the timer writing the file back.
    m_itemSave->stop();
    m_bodySave->stop();
    const bool wasDirty = isDirty();
    m_itemsDirty = false;
    m_bodyDirty = false;
    m_savePath.clear();
    m_currentPath.clear();
    m_current = {};
    m_items->setItems({});
    if (wasDirty) {
        Q_EMIT dirtyChanged();
    }
    Q_EMIT currentChanged();
}

bool Nota::addRepeating(const QString &text)
{
    if (!m_plans) {
        return false;
    }
    flush();
    Workplan::Template added;
    if (!m_plans->addTemplate(text, &added)) {
        fail(m_plans->lastError());
        return false;
    }
    // Seed it into today rather than waiting for tomorrow, which is what
    // someone adding a daily item at 09:00 plainly meant.
    if (!m_plans->seedInto(QDate::currentDate())) {
        fail(m_plans->lastError());
        return false;
    }
    if (m_currentPath == m_plans->pathFor(QDate::currentDate())) {
        reload();
    }
    return true;
}

bool Nota::stopRepeating(const QString &id)
{
    if (!m_plans || id.isEmpty()) {
        return false;
    }
    flush();
    if (!m_plans->removeTemplate(id) || !m_plans->dropFrom(QDate::currentDate(), id)) {
        fail(m_plans->lastError());
        return false;
    }
    if (m_currentPath == m_plans->pathFor(QDate::currentDate())) {
        reload();
    }
    return true;
}

bool Nota::moveRowToToday(int row)
{
    if (!m_plans || m_currentPath.isEmpty()) {
        return false;
    }
    const QString today = m_plans->ensure(QDate::currentDate());
    if (today.isEmpty() || today == m_currentPath) {
        return false;
    }

    flush();
    m_current.replaceItemsAt(m_items->items(), QTime::currentTime().toString(u"HH:mm"_s));

    const QList<MdNote::Item> all = m_current.items;
    if (row < 0 || row >= all.size() || all.at(row).isHeading()) {
        return false;
    }
    // takeItems addresses by id, so mint one for a row that has never been
    // saved rather than moving nothing.
    QString id = all.at(row).id;
    if (id.isEmpty()) {
        id = Workplan::newId();
        m_current.items[row].id = id;
    }

    const QList<MdNote::Item> moved = m_current.takeItems({id});
    if (moved.isEmpty()) {
        return false;
    }

    auto destination = m_vault->readNote(today);
    if (!destination) {
        fail(m_vault->lastError());
        return false;
    }
    destination->appendItems(moved);

    if (!m_vault->writeNote(today, *destination) || !m_vault->writeNote(m_currentPath, m_current)) {
        fail(m_vault->lastError());
        return false;
    }
    reload();
    return true;
}

QString Nota::raw() const
{
    if (m_currentPath.isEmpty()) {
        return {};
    }
    return m_vault->readRaw(m_currentPath).value_or(QString());
}

bool Nota::saveRaw(const QString &content)
{
    if (m_currentPath.isEmpty()) {
        return false;
    }
    // Drop anything the structured editor had pending: the raw text is now the
    // more recent statement of what this file should say.
    m_itemSave->stop();
    m_bodySave->stop();
    m_itemsDirty = false;
    m_bodyDirty = false;
    Q_EMIT dirtyChanged();

    if (!m_vault->writeRaw(m_currentPath, content)) {
        fail(m_vault->lastError());
        return false;
    }
    reload();
    return true;
}

#include "moc_app.cpp"
