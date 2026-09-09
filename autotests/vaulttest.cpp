/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 */

#include "vault.h"

#include <QDir>
#include <QSignalSpy>
#include <QFile>
#include <QTemporaryDir>
#include <QTest>

using namespace Qt::StringLiterals;

class VaultTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void init();
    void cleanup();

    void pathsThatEscapeTheVaultAreRefused_data();
    void pathsThatEscapeTheVaultAreRefused();
    void writingCreatesMissingFolders();
    void aNoteRoundTripsThroughTheVault();
    void renameRefusesToOverwrite();
    void deleteMovesToTheTrashRatherThanRemoving();
    void theApplicationFolderCannotBeDeleted();
    void theTreeHidesDotFoldersAndSortsFoldersFirst();
    void listNotesSkipsHiddenFoldersAndNonNotes();
    void savingANoteIsNotReportedAsSomethingDisappearing();

private:
    void touch(const QString &rel, const QString &content = u"x"_s) const;

    std::unique_ptr<QTemporaryDir> m_dir;
    std::unique_ptr<Vault> m_vault;
};

void VaultTest::init()
{
    m_dir = std::make_unique<QTemporaryDir>();
    QVERIFY(m_dir->isValid());
    m_vault = std::make_unique<Vault>(m_dir->path());
    QVERIFY(m_vault->isOpen());
}

void VaultTest::cleanup()
{
    m_vault.reset();
    m_dir.reset();
}

void VaultTest::touch(const QString &rel, const QString &content) const
{
    QVERIFY(m_vault->writeRaw(rel, content));
}

void VaultTest::pathsThatEscapeTheVaultAreRefused_data()
{
    QTest::addColumn<QString>("path");
    QTest::newRow("empty") << QString();
    QTest::newRow("absolute") << u"/etc/passwd"_s;
    QTest::newRow("parent") << u".."_s;
    QTest::newRow("climbing") << u"../outside.md"_s;
    QTest::newRow("climbing through") << u"Projects/../../outside.md"_s;
    QTest::newRow("dot") << u"."_s;
    QTest::newRow("backslash climbing") << u"..\\outside.md"_s;
}

void VaultTest::pathsThatEscapeTheVaultAreRefused()
{
    QFETCH(QString, path);
    // A path that escapes the root would read or write arbitrary files.
    QVERIFY(!m_vault->resolve(path).has_value());
    QVERIFY(!m_vault->writeRaw(path, u"nope"_s));
    QVERIFY(!m_vault->readRaw(path).has_value());
}

void VaultTest::writingCreatesMissingFolders()
{
    QVERIFY(m_vault->writeRaw(u"Projects/Deep/Nested/note.md"_s, u"hello\n"_s));
    QVERIFY(QFile::exists(m_dir->path() + "/Projects/Deep/Nested/note.md"_L1));
    QCOMPARE(m_vault->readRaw(u"Projects/Deep/Nested/note.md"_s).value(), u"hello\n"_s);
}

void VaultTest::aNoteRoundTripsThroughTheVault()
{
    const QString source = uR"(---
type: workplan
date: 2026-09-02
hours: "01:20"
daytype: work
---

- [x] Fix auth #rv-api <!--n id:A t:09:34 done:11:02-->
- [ ] Review PR <!--n id:B t:09:40-->
)"_s;

    touch(u"Workplans/2026-09-02.md"_s, source);
    const auto note = m_vault->readNote(u"Workplans/2026-09-02.md"_s);
    QVERIFY(note.has_value());
    QCOMPARE(note->items.size(), 2);

    QVERIFY(m_vault->writeNote(u"Workplans/2026-09-02.md"_s, *note));
    QCOMPARE(m_vault->readRaw(u"Workplans/2026-09-02.md"_s).value(), source);
}

void VaultTest::renameRefusesToOverwrite()
{
    touch(u"one.md"_s, u"one\n"_s);
    touch(u"two.md"_s, u"two\n"_s);

    // Silently replacing a note the user cannot see would lose their work.
    QVERIFY(!m_vault->rename(u"one.md"_s, u"two.md"_s));
    QCOMPARE(m_vault->readRaw(u"two.md"_s).value(), u"two\n"_s);

    QVERIFY(m_vault->rename(u"one.md"_s, u"Moved/three.md"_s));
    QCOMPARE(m_vault->readRaw(u"Moved/three.md"_s).value(), u"one\n"_s);
    QVERIFY(!m_vault->exists(u"one.md"_s));
}

void VaultTest::deleteMovesToTheTrashRatherThanRemoving()
{
    touch(u"Projects/doomed.md"_s, u"still here\n"_s);

    QVERIFY(m_vault->remove(u"Projects/doomed.md"_s));
    QVERIFY(!m_vault->exists(u"Projects/doomed.md"_s));

    // Nothing is removed outright, so a mis-click in the sidebar costs nothing.
    const QDir trash(m_dir->path() + "/.nota/trash"_L1);
    const QStringList entries = trash.entryList(QDir::Dirs | QDir::NoDotAndDotDot);
    QCOMPARE(entries.size(), 1);

    const QString entry = trash.absoluteFilePath(entries.first());
    QVERIFY(QFile::exists(entry + "/Projects/doomed.md"_L1));
    QVERIFY(QFile::exists(entry + "/meta.json"_L1));
}

void VaultTest::theApplicationFolderCannotBeDeleted()
{
    touch(u".nota/templates/recurring.md"_s, u"- [ ] Stand-up\n"_s);

    QVERIFY(!m_vault->remove(u".nota"_s));
    QVERIFY(!m_vault->remove(u".nota/templates/recurring.md"_s));
    QVERIFY(m_vault->exists(u".nota/templates/recurring.md"_s));
}

void VaultTest::theTreeHidesDotFoldersAndSortsFoldersFirst()
{
    touch(u"zebra.md"_s);
    touch(u"apple.md"_s);
    touch(u"Projects/one.md"_s);
    touch(u"Archive/old.md"_s);
    touch(u".nota/settings.json"_s, u"{}"_s);
    touch(u"attachments/pasted.png"_s);

    const VaultNode root = m_vault->tree();
    QStringList names;
    for (const VaultNode &child : root.children) {
        names.append(child.name);
    }

    // Folders before notes, each group by name; .nota and attachments hidden.
    QCOMPARE(names, QStringList({u"Archive"_s, u"Projects"_s, u"apple"_s, u"zebra"_s}));
    QCOMPARE(root.children.at(1).children.size(), 1);
    QCOMPARE(root.children.at(1).children.first().path, u"Projects/one.md"_s);
    // A note's display name drops the extension; its path keeps it.
    QCOMPARE(root.children.at(2).name, u"apple"_s);
    QCOMPARE(root.children.at(2).path, u"apple.md"_s);
}

void VaultTest::listNotesSkipsHiddenFoldersAndNonNotes()
{
    touch(u"Workplans/2026-09-02.md"_s);
    touch(u"Workplans/2026-09-01.md"_s);
    touch(u"Projects/notes.md"_s);
    touch(u"Projects/notes.txt"_s);
    touch(u".nota/templates/recurring.md"_s);

    QCOMPARE(m_vault->listNotes(),
             QStringList({u"Projects/notes.md"_s, u"Workplans/2026-09-01.md"_s, u"Workplans/2026-09-02.md"_s}));
}

void VaultTest::savingANoteIsNotReportedAsSomethingDisappearing()
{
    // QSaveFile writes through a temporary beside the note and renames it away.
    // The watcher sees that temporary vanish on every save; treating it as a
    // deletion rebuilt the sidebar while the user was typing.
    touch(u"Projects/notes.md"_s, u"first"_s);
    m_vault->startWatching();

    QSignalSpy removed(m_vault.get(), &Vault::noteRemoved);
    QSignalSpy tree(m_vault.get(), &Vault::treeChanged);

    QVERIFY(m_vault->writeRaw(u"Projects/notes.md"_s, u"second"_s));

    // Long enough for inotify to deliver, and for KDirWatch's own poll.
    QTest::qWait(2000);
    QCOMPARE(removed.count(), 0);

    // The folder itself does go dirty on a save, so treeChanged is expected
    // here. FolderTreeModel is where that is kept from resetting the sidebar.
    QVERIFY(tree.count() <= 1);
}

QTEST_GUILESS_MAIN(VaultTest)

#include "vaulttest.moc"
