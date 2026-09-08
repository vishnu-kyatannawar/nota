// SPDX-FileCopyrightText: 2026 Vishnu Kyatannawar
// SPDX-License-Identifier: MIT

import QtQuick.Controls as QQC2
import org.kde.kirigami as Kirigami

// A header button in the sidebar: icon only, with the label as its tooltip.
QQC2.ToolButton {
    display: QQC2.AbstractButton.IconOnly

    QQC2.ToolTip.text: text
    QQC2.ToolTip.visible: hovered
    QQC2.ToolTip.delay: Kirigami.Units.toolTipDelay
}
