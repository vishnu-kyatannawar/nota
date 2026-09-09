/*
 * SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
 * SPDX-License-Identifier: MIT
 *
 * The rules an update offer rests on: is that version newer, and are we even
 * allowed to replace this binary. Neither needs the network to be wrong.
 */

#include "update.h"

#include <QTest>

using namespace Qt::StringLiterals;

class UpdateTest : public QObject
{
    Q_OBJECT

private Q_SLOTS:
    void versionsCompareNumericallyNotAsText_data();
    void versionsCompareNumericallyNotAsText();
    void aLeadingVAndMissingPartsAreTheSameVersion();
    void aReleasePayloadYieldsTheTagAndThePage();
    void rubbishInThePayloadIsNotAnUpdate();
    void aBinaryUnderTheHomeDirectoryIsOursToReplace();
    void aBinaryUnderASystemPrefixBelongsToThePackageManager();
    void thereIsAUserAgent();
};

void UpdateTest::versionsCompareNumericallyNotAsText_data()
{
    QTest::addColumn<QString>("a");
    QTest::addColumn<QString>("b");
    QTest::addColumn<int>("sign");

    QTest::newRow("equal") << u"5.0.0"_s << u"5.0.0"_s << 0;
    QTest::newRow("patch") << u"5.0.0"_s << u"5.0.1"_s << -1;
    QTest::newRow("minor") << u"5.1.0"_s << u"5.0.9"_s << 1;
    QTest::newRow("major") << u"4.6.1"_s << u"5.0.0"_s << -1;
    // The reason this is not a string compare: "10" is after "9", not before.
    QTest::newRow("two digits") << u"5.9.0"_s << u"5.10.0"_s << -1;
    // The version the rewrite carried before it was released.
    QTest::newRow("the old branch version") << u"0.2.0"_s << u"5.0.0"_s << -1;
}

void UpdateTest::versionsCompareNumericallyNotAsText()
{
    QFETCH(QString, a);
    QFETCH(QString, b);
    QFETCH(int, sign);

    const int got = Update::compare(a, b);
    QCOMPARE(got < 0, sign < 0);
    QCOMPARE(got > 0, sign > 0);
    QCOMPARE(got == 0, sign == 0);
}

void UpdateTest::aLeadingVAndMissingPartsAreTheSameVersion()
{
    QCOMPARE(Update::compare(u"v5.1.0"_s, u"5.1.0"_s), 0);
    QCOMPARE(Update::compare(u"5.1"_s, u"5.1.0"_s), 0);
    QCOMPARE(Update::compare(u"5"_s, u"5.0.0"_s), 0);
}

void UpdateTest::aReleasePayloadYieldsTheTagAndThePage()
{
    const QByteArray body = R"({
      "tag_name": "v5.1.0",
      "html_url": "https://github.com/vishnu-kyatannawar/nota/releases/tag/v5.1.0",
      "name": "Nota 5.1.0"
    })";

    const Update::Release release = Update::parseLatest(body);
    QVERIFY(release.isValid());
    // Stored without the v, so it compares against what the binary reports.
    QCOMPARE(release.version, u"5.1.0"_s);
    QCOMPARE(release.url, u"https://github.com/vishnu-kyatannawar/nota/releases/tag/v5.1.0"_s);
}

void UpdateTest::rubbishInThePayloadIsNotAnUpdate()
{
    // Being offline, rate limited or behind a captive portal all end up here,
    // and none of them should be reported as a new version.
    QVERIFY(!Update::parseLatest(QByteArray()).isValid());
    QVERIFY(!Update::parseLatest("<html>not json</html>").isValid());
    QVERIFY(!Update::parseLatest(R"({"message": "API rate limit exceeded"})").isValid());
    QVERIFY(!Update::parseLatest(R"({"tag_name": ""})").isValid());
}

void UpdateTest::aBinaryUnderTheHomeDirectoryIsOursToReplace()
{
    QCOMPARE(Update::kindFor(u"/home/someone/.local/bin/nota"_s, u"/home/someone"_s), Update::Kind::UserManaged);
}

void UpdateTest::aBinaryUnderASystemPrefixBelongsToThePackageManager()
{
    // pacman, dnf or apt owns these. Writing over them leaves the package
    // database describing files that are no longer the ones on disk.
    QCOMPARE(Update::kindFor(u"/usr/bin/nota"_s, u"/home/someone"_s), Update::Kind::SystemManaged);
    QCOMPARE(Update::kindFor(u"/usr/local/bin/nota"_s, u"/home/someone"_s), Update::Kind::SystemManaged);
    QCOMPARE(Update::kindFor(u"/opt/nota/bin/nota"_s, u"/home/someone"_s), Update::Kind::SystemManaged);
    // A home directory that is a prefix of the path by text alone is not a
    // home directory: /home/someone-else is not inside /home/someone.
    QCOMPARE(Update::kindFor(u"/home/someone-else/.local/bin/nota"_s, u"/home/someone"_s), Update::Kind::SystemManaged);
}

void UpdateTest::thereIsAUserAgent()
{
    // Not decoration: GitHub's API answers 403 to a request without one, and
    // Qt sets none. Losing this header makes the update check fail silently on
    // every machine, which is the worst way for it to fail.
    const QByteArray agent = Update::userAgent();
    QVERIFY(!agent.isEmpty());
    QVERIFY(agent.startsWith(QByteArrayLiteral("Nota/")));
    QVERIFY(agent.size() > 5);
}

QTEST_GUILESS_MAIN(UpdateTest)

#include "updatetest.moc"
