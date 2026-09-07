/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "app.h"

#include "foldertreemodel.h"
#include "itemmodel.h"
#include "vault.h"

#include <KLocalizedString>

#include <QDateTime>
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

    // An external edit reloads the open page; a new or removed file rebuilds
    // the sidebar. The vault suppresses the events our own writes cause.
    connect(m_vault.get(), &Vault::treeChanged, m_folderTree.get(), &FolderTreeModel::refresh);
    connect(m_vault.get(), &Vault::noteChanged, this, [this](const QString &path) {
        m_folderTree->refresh();
        if (path == m_currentPath) {
            reload();
        }
    });
    connect(m_vault.get(), &Vault::noteRemoved, this, [this](const QString &path) {
        if (path == m_currentPath) {
            m_currentPath.clear();
            m_current = {};
            m_items->setItems({});
            Q_EMIT currentChanged();
        }
    });
    m_vault->startWatching();

    openToday();
    scheduleMidnightRoll();
}

Nota::~Nota() = default;

QString Nota::vaultPath() const
{
    return m_settings.vaultPath;
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
    m_currentPath = path;
    reload();
}

void Nota::reload()
{
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

#include "moc_app.cpp"
