/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "vault.h"

#include <KDirWatch>
#include <KLocalizedString>

#include <QDir>
#include <QDirIterator>
#include <QFileInfo>
#include <QRandomGenerator>
#include <QSaveFile>
#include <QTimer>

using namespace Qt::StringLiterals;

Vault::Vault(const QString &root, QObject *parent)
    : QObject(parent)
    , m_root(QFileInfo(root).absoluteFilePath())
{
    QDir dir;
    if (!dir.mkpath(m_root)) {
        fail(i18n("Could not create the notes folder at %1.", m_root));
        return;
    }
    m_open = true;
}

Vault::~Vault() = default;

bool Vault::fail(const QString &message) const
{
    m_lastError = message;
    return false;
}

std::optional<QString> Vault::resolve(const QString &rel) const
{
    if (rel.isEmpty()) {
        fail(i18n("An empty path is outside the vault."));
        return std::nullopt;
    }
    if (rel.startsWith(u'/') || QDir::isAbsolutePath(rel)) {
        fail(i18n("The path %1 is outside the vault.", rel));
        return std::nullopt;
    }

    QString normalised = rel;
    normalised.replace(u'\\', u'/');
    const QString clean = QDir::cleanPath(normalised);
    if (clean == "."_L1 || clean == ".."_L1 || clean.startsWith("../"_L1) || clean.startsWith(u'/')) {
        fail(i18n("The path %1 is outside the vault.", rel));
        return std::nullopt;
    }

    const QString abs = QDir::cleanPath(m_root + u'/' + clean);

    // cleanPath already removed any interior "..", but compare anyway so an
    // unusual root cannot widen what is reachable.
    if (abs != m_root && !abs.startsWith(m_root + u'/')) {
        fail(i18n("The path %1 is outside the vault.", rel));
        return std::nullopt;
    }
    return abs;
}

bool Vault::exists(const QString &rel) const
{
    const auto abs = resolve(rel);
    return abs.has_value() && QFileInfo::exists(*abs);
}

bool Vault::isFolder(const QString &rel) const
{
    const auto abs = resolve(rel);
    return abs.has_value() && QFileInfo(*abs).isDir();
}

std::optional<QString> Vault::readRaw(const QString &rel) const
{
    const auto abs = resolve(rel);
    if (!abs) {
        return std::nullopt;
    }
    QFile f(*abs);
    if (!f.open(QIODevice::ReadOnly)) {
        fail(i18n("Could not read %1: %2", rel, f.errorString()));
        return std::nullopt;
    }
    return QString::fromUtf8(f.readAll());
}

std::optional<MdNote::Note> Vault::readNote(const QString &rel) const
{
    const auto raw = readRaw(rel);
    if (!raw) {
        return std::nullopt;
    }
    return MdNote::parse(*raw);
}

bool Vault::writeNote(const QString &rel, const MdNote::Note &note)
{
    return writeRaw(rel, MdNote::serialize(note));
}

bool Vault::writeRaw(const QString &rel, const QString &content)
{
    const auto abs = resolve(rel);
    if (!abs) {
        return false;
    }
    const QString dir = QFileInfo(*abs).absolutePath();
    if (!QDir().mkpath(dir)) {
        return fail(i18n("Could not create the folder for %1.", rel));
    }

    // QSaveFile writes to a temporary file and renames, so a crash mid-write
    // cannot truncate a note the user already has.
    ignoreNextChange(rel);
    QSaveFile out(*abs);
    if (!out.open(QIODevice::WriteOnly)) {
        return fail(i18n("Could not write %1: %2", rel, out.errorString()));
    }
    out.write(content.toUtf8());
    if (!out.commit()) {
        return fail(i18n("Could not save %1: %2", rel, out.errorString()));
    }
    return true;
}

bool Vault::createFolder(const QString &rel)
{
    const auto abs = resolve(rel);
    if (!abs) {
        return false;
    }
    if (!QDir().mkpath(*abs)) {
        return fail(i18n("Could not create the folder %1.", rel));
    }
    Q_EMIT treeChanged();
    return true;
}

bool Vault::rename(const QString &from, const QString &to)
{
    const auto src = resolve(from);
    if (!src) {
        return false;
    }
    const auto dst = resolve(to);
    if (!dst) {
        return false;
    }
    if (QFileInfo::exists(*dst)) {
        return fail(i18n("%1 already exists.", to));
    }
    if (!QDir().mkpath(QFileInfo(*dst).absolutePath())) {
        return fail(i18n("Could not create the folder for %1.", to));
    }
    if (!QDir().rename(*src, *dst)) {
        return fail(i18n("Could not rename %1 to %2.", from, to));
    }
    Q_EMIT treeChanged();
    return true;
}

QString Vault::trashDir() const
{
    return m_root + u'/' + AppDirName + u'/' + TrashDirName;
}

bool Vault::remove(const QString &rel)
{
    const auto abs = resolve(rel);
    if (!abs) {
        return false;
    }
    QString clean = rel;
    clean.replace(u'\\', u'/');
    clean = QDir::cleanPath(clean);
    if (clean == AppDirName || clean.startsWith(AppDirName + u'/')) {
        return fail(i18n("The application folder cannot be deleted."));
    }

    const QFileInfo info(*abs);
    if (!info.exists()) {
        return fail(i18n("%1 does not exist.", rel));
    }

    // A sortable timestamp plus a little randomness, so two deletes in the same
    // millisecond cannot collide.
    const QString id = QStringLiteral("%1-%2")
                           .arg(QDateTime::currentMSecsSinceEpoch())
                           .arg(QRandomGenerator::global()->bounded(0x1000000), 6, 16, QLatin1Char('0'));
    const QString entryDir = trashDir() + u'/' + id;
    const QString dest = entryDir + u'/' + clean;

    if (!QDir().mkpath(QFileInfo(dest).absolutePath())) {
        return fail(i18n("Could not prepare the trash folder."));
    }
    if (!QDir().rename(*abs, dest)) {
        return fail(i18n("Could not move %1 to the trash.", rel));
    }

    // Nothing is removed outright, so a mis-click in the sidebar costs nothing.
    QFile meta(entryDir + "/meta.json"_L1);
    if (meta.open(QIODevice::WriteOnly)) {
        const QString json = QStringLiteral(R"({
  "id": "%1",
  "path": "%2",
  "name": "%3",
  "isFolder": %4,
  "deletedAt": "%5"
}
)")
                                 .arg(id, clean, clean.section(u'/', -1), info.isDir() ? u"true"_s : u"false"_s,
                                      QDateTime::currentDateTime().toString(Qt::ISODate));
        meta.write(json.toUtf8());
        meta.close();
    }

    Q_EMIT treeChanged();
    return true;
}

QList<VaultNode> Vault::readDir(const QString &abs, const QString &rel) const
{
    QList<VaultNode> folders;
    QList<VaultNode> notes;

    QDir dir(abs);
    const QFileInfoList entries = dir.entryInfoList(QDir::Files | QDir::Dirs | QDir::NoDotAndDotDot | QDir::Hidden);
    for (const QFileInfo &entry : entries) {
        const QString name = entry.fileName();
        // A vault lives alongside other tools' metadata: skip our own folder
        // and anything hidden, rather than showing .git and .obsidian.
        if (name.startsWith(u'.')) {
            continue;
        }
        // Pasted images live here. It holds no notes, so it would show as an
        // empty folder and invite someone to put pages in it.
        if (rel.isEmpty() && name == AttachmentDir && entry.isDir()) {
            continue;
        }

        const QString childRel = rel.isEmpty() ? name : rel + u'/' + name;

        if (entry.isDir()) {
            VaultNode node;
            node.name = name;
            node.path = childRel;
            node.isFolder = true;
            node.children = readDir(entry.absoluteFilePath(), childRel);
            folders.append(node);
            continue;
        }
        if (!name.endsWith(NoteExt, Qt::CaseInsensitive)) {
            continue;
        }
        VaultNode node;
        node.name = name.chopped(NoteExt.size());
        node.path = childRel;
        notes.append(node);
    }

    const auto byName = [](const VaultNode &a, const VaultNode &b) {
        return a.name < b.name;
    };
    std::sort(folders.begin(), folders.end(), byName);
    std::sort(notes.begin(), notes.end(), byName);

    folders.append(notes);
    return folders;
}

VaultNode Vault::tree() const
{
    VaultNode root;
    root.name = QFileInfo(m_root).fileName();
    root.isFolder = true;
    root.children = readDir(m_root, QString());
    return root;
}

QStringList Vault::listNotes() const
{
    QStringList out;
    QDirIterator it(m_root, QDir::Files | QDir::Dirs | QDir::NoDotAndDotDot, QDirIterator::Subdirectories);
    while (it.hasNext()) {
        const QString abs = it.next();
        const QFileInfo info = it.fileInfo();
        if (info.isDir()) {
            continue;
        }
        const QString name = info.fileName();
        if (name.startsWith(u'.') || !name.endsWith(NoteExt, Qt::CaseInsensitive)) {
            continue;
        }
        const QString rel = QDir(m_root).relativeFilePath(abs);
        // Hidden folders — our own .nota included — hold no user notes.
        if (rel.startsWith(u'.') || rel.contains("/."_L1)) {
            continue;
        }
        out.append(rel);
    }
    out.sort();
    return out;
}

QString Vault::relativeFor(const QString &absolutePath) const
{
    if (absolutePath == m_root) {
        return {};
    }
    if (!absolutePath.startsWith(m_root + u'/')) {
        return {};
    }
    return absolutePath.sliced(m_root.size() + 1);
}

void Vault::ignoreNextChange(const QString &rel)
{
    if (!m_ignoreOnce.contains(rel)) {
        m_ignoreOnce.append(rel);
    }
}

void Vault::startWatching()
{
    if (m_watch) {
        return;
    }
    // An own instance rather than KDirWatch::self(): the shared one carries
    // signals for paths other KDE code in this process registered.
    m_watch = new KDirWatch(this);
    m_watch->addDir(m_root, KDirWatch::WatchFiles | KDirWatch::WatchSubDirs);

    connect(m_watch, &KDirWatch::dirty, this, &Vault::onDirty);
    connect(m_watch, &KDirWatch::created, this, &Vault::onDirty);
    connect(m_watch, &KDirWatch::deleted, this, [this](const QString &path) {
        const QString rel = relativeFor(path);
        // The same filter onDirty() applies: our own folder is not part of the
        // tree the sidebar shows.
        if (rel.isEmpty() || rel.startsWith(u'.') || rel.contains("/."_L1)) {
            return;
        }
        // QSaveFile writes through "<name>.md.XXXXXX" beside the note and
        // renames it away on every save. A vanished temporary is not a page
        // being deleted, and reporting it rebuilt the sidebar every 400 ms
        // while someone typed, collapsing whatever they had expanded.
        if (rel.section(u'/', -1).contains(QString(NoteExt) + u'.')) {
            return;
        }
        Q_EMIT noteRemoved(rel);
        Q_EMIT treeChanged();
    });
}

void Vault::onDirty(const QString &absolutePath)
{
    const QString rel = relativeFor(absolutePath);
    if (rel.isEmpty() || rel.startsWith(u'.') || rel.contains("/."_L1)) {
        return;
    }

    if (QFileInfo(absolutePath).isDir()) {
        Q_EMIT treeChanged();
        return;
    }
    if (!rel.endsWith(NoteExt, Qt::CaseInsensitive)) {
        return;
    }
    // Every write this class makes arms the guard, so the app is never told
    // about its own saves.
    if (m_ignoreOnce.removeOne(rel)) {
        return;
    }
    Q_EMIT noteChanged(rel);
}

#include "moc_vault.cpp"
