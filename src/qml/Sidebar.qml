// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick
import QtQuick.Layouts
import QtQuick.Controls as QQC2

import org.kde.kirigami as Kirigami
import org.kde.kitemmodels as KItemModels
import org.kde.nota

// The sidebar tree is the directory tree. The model is a real tree; flattening
// it here rather than in C++ is what buys the expand and collapse for free.
Item {
    id: sidebar

    Kirigami.Theme.colorSet: Kirigami.Theme.Window
    Kirigami.Theme.inherit: false

    Rectangle {
        anchors.fill: parent
        color: Kirigami.Theme.backgroundColor
    }

    KItemModels.KDescendantsProxyModel {
        id: flatTree
        model: Nota.folderTree
        expandsByDefault: false
    }

    ColumnLayout {
        anchors.fill: parent
        spacing: 0

        RowLayout {
            Layout.fillWidth: true
            Layout.margins: Kirigami.Units.smallSpacing
            spacing: Kirigami.Units.smallSpacing

            Kirigami.Icon {
                source: "io.github.vishnu_kyatannawar.Nota"
                fallback: "text-markdown"
                implicitWidth: Kirigami.Units.iconSizes.smallMedium
                implicitHeight: Kirigami.Units.iconSizes.smallMedium
            }

            Kirigami.Heading {
                text: i18nc("@title The application name", "Nota")
                level: 4
                Layout.fillWidth: true
            }

            QQC2.ToolButton {
                icon.name: "go-jump-today"
                display: QQC2.AbstractButton.IconOnly
                text: i18nc("@action:button", "Today's workplan")
                onClicked: Nota.openToday()

                QQC2.ToolTip.text: text
                QQC2.ToolTip.visible: hovered
                QQC2.ToolTip.delay: Kirigami.Units.toolTipDelay
            }
        }

        Kirigami.Separator {
            Layout.fillWidth: true
        }

        QQC2.ScrollView {
            Layout.fillWidth: true
            Layout.fillHeight: true
            clip: true

            ListView {
                id: treeView
                model: flatTree
                currentIndex: -1

                delegate: QQC2.ItemDelegate {
                    id: treeDelegate

                    required property int index
                    required property string name
                    required property string path
                    required property bool isFolder
                    required property string iconName
                    required property int kDescendantLevel
                    required property bool kDescendantExpandable
                    required property bool kDescendantExpanded

                    width: ListView.view.width
                    highlighted: !isFolder && path === Nota.currentPath

                    onClicked: {
                        if (kDescendantExpandable) {
                            flatTree.toggleChildren(index);
                        } else {
                            Nota.open(path);
                        }
                    }

                    contentItem: RowLayout {
                        spacing: Kirigami.Units.smallSpacing

                        Item {
                            // One grid unit of indent per level of nesting.
                            implicitWidth: Kirigami.Units.gridUnit * (treeDelegate.kDescendantLevel - 1)
                            implicitHeight: 1
                        }

                        Kirigami.Icon {
                            source: treeDelegate.kDescendantExpandable
                                ? (treeDelegate.kDescendantExpanded ? "collapse-symbolic" : "expand-symbolic")
                                : treeDelegate.iconName
                            implicitWidth: Kirigami.Units.iconSizes.small
                            implicitHeight: Kirigami.Units.iconSizes.small
                        }

                        QQC2.Label {
                            text: treeDelegate.name
                            elide: Text.ElideRight
                            Layout.fillWidth: true
                        }
                    }
                }

                Kirigami.PlaceholderMessage {
                    anchors.centerIn: parent
                    width: parent.width - Kirigami.Units.gridUnit * 2
                    visible: treeView.count === 0
                    text: i18n("This vault is empty")
                    explanation: Nota.vaultPath
                }
            }
        }

        Kirigami.Separator {
            Layout.fillWidth: true
        }

        QQC2.Label {
            Layout.fillWidth: true
            Layout.margins: Kirigami.Units.smallSpacing
            text: Nota.vaultPath
            elide: Text.ElideMiddle
            font: Kirigami.Theme.smallFont
            opacity: 0.7
        }
    }
}
