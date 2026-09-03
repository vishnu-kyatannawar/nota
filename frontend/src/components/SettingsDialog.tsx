import { useEffect, useState } from "react";
import { Dialogs } from "@wailsio/runtime";
import type { Settings } from "../lib/api";
import { api } from "../lib/api";
import { THEMES, type Theme } from "../lib/theme";
import { FACES, SIZES, type Fonts, type FontSlot } from "../lib/fonts";
import { Dialog } from "./Dialog";

type Props = {
  open: boolean;
  onClose: () => void;
  theme: Theme;
  onTheme: (t: Theme) => void;
  fonts: Fonts;
  onFonts: (f: Fonts) => void;
  onError: (m: string) => void;
  onVaultChanged: () => void;
};

export function SettingsDialog({ open, onClose, theme, onTheme, fonts, onFonts, onError, onVaultChanged }: Props) {
  const [settings, setSettings] = useState<Settings | null>(null);
  const [note, setNote] = useState<string | null>(null);

  // Each opening starts with a clean status line.
  const [wasOpen, setWasOpen] = useState(open);
  if (open !== wasOpen) {
    setWasOpen(open);
    if (open) setNote(null);
  }

  useEffect(() => {
    if (!open) return;
    api.settings().then(setSettings).catch((e) => onError(String(e)));
  }, [open, onError]);

  const saveWeekends = async (createOnWeekends: boolean) => {
    if (!settings) return;
    const next = { ...settings, createOnWeekends };
    try {
      await api.saveSettings(next);
      setSettings(next);
    } catch (e) {
      onError(String(e));
    }
  };

  const exportVault = async () => {
    try {
      const picked = await Dialogs.OpenFile({ Title: "Choose where to save the backup", CanChooseDirectories: true, CanChooseFiles: false });
      const dir = Array.isArray(picked) ? picked[0] : picked;
      if (!dir) return;
      const bundle = await api.exportTo(dir);
      setNote(`Exported to ${bundle}`);
    } catch (e) {
      onError(String(e));
    }
  };

  const restoreVault = async () => {
    try {
      const picked = await Dialogs.OpenFile({
        Title: "Choose a Nota backup to restore",
        Filters: [{ DisplayName: "Nota backup", Pattern: "*.zip" }],
      });
      const bundle = Array.isArray(picked) ? picked[0] : picked;
      if (!bundle) return;
      await api.restoreBackup(bundle);
      setNote("Restored. Notes and settings from the backup are now in the vault.");
      onVaultChanged();
    } catch (e) {
      onError(String(e));
    }
  };

  return (
    <Dialog open={open} onClose={onClose} title="Settings" width="30rem">
      <div className="space-y-5 text-sm">
        <Field label="Appearance">
          <div className="flex gap-1 rounded-md border border-border p-0.5">
            {THEMES.map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => onTheme(t)}
                className={`flex-1 rounded px-2 py-1 text-[13px] capitalize ${theme === t ? "bg-accent text-accent-ink" : "text-ink-muted hover:bg-surface-sunken"}`}
              >
                {t}
              </button>
            ))}
          </div>
        </Field>

        <Field label="Fonts">
          <div className="grid gap-2">
            {(["ui", "notes", "code"] as FontSlot[]).map((slot) => (
              <label key={slot} className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
                <span className="text-xs text-ink-muted">{{ ui: "Interface", notes: "Notes", code: "Code" }[slot]}</span>
                <select
                  value={fonts[slot]}
                  onChange={(e) => onFonts({ ...fonts, [slot]: e.target.value })}
                  style={{ fontFamily: FACES[slot].find((f) => f.id === fonts[slot])?.stack }}
                  className="rounded-md border border-border bg-surface px-2 py-1 text-[13px]"
                >
                  {FACES[slot].map((f) => (
                    <option key={f.id} value={f.id} style={{ fontFamily: f.stack }}>{f.label} — {f.sample}</option>
                  ))}
                </select>
              </label>
            ))}
            <div className="grid grid-cols-[4.5rem_minmax(0,1fr)] items-center gap-2">
              <span className="text-xs text-ink-muted">Size</span>
              <div className="flex gap-1 rounded-md border border-border p-0.5">
                {SIZES.map((sz) => (
                  <button
                    key={sz.id}
                    type="button"
                    onClick={() => onFonts({ ...fonts, size: sz.id })}
                    className={`flex-1 rounded px-2 py-1 text-[13px] ${fonts.size === sz.id ? "bg-accent text-accent-ink" : "text-ink-muted hover:bg-surface-sunken"}`}
                  >
                    {sz.label}
                  </button>
                ))}
              </div>
            </div>
          </div>
          <p className="mt-1 text-xs text-ink-muted">All faces ship with the app; nothing is fetched from the network.</p>
        </Field>

        <Field label="Workplans">
          <label className="flex items-center gap-2">
            <input
              type="checkbox"
              checked={settings?.createOnWeekends ?? true}
              onChange={(e) => void saveWeekends(e.target.checked)}
              className="accent-accent"
            />
            Create a workplan on weekends too
          </label>
          <p className="mt-1 text-xs text-ink-muted">Rollover carries unfinished items across gaps either way; this only decides whether Saturday and Sunday get a note.</p>
        </Field>

        <Field label="Vault">
          <div className="rounded-md border border-border bg-surface px-2.5 py-1.5 font-mono text-xs text-ink-muted">{settings?.vaultPath ?? "…"}</div>
          <p className="mt-1 text-xs text-ink-muted">Notes are plain markdown files in this folder. Settings live inside it at .nota/settings.json.</p>
        </Field>

        <Field label="Backup">
          <div className="flex gap-2">
            <button type="button" onClick={() => void exportVault()} className="rounded-md border border-border px-3 py-1.5 hover:bg-surface-sunken">Export…</button>
            <button type="button" onClick={() => void restoreVault()} className="rounded-md border border-border px-3 py-1.5 hover:bg-surface-sunken">Restore…</button>
          </div>
          <p className="mt-1 text-xs text-ink-muted">Export writes one zip of everything. Restore puts a zip back and rebuilds the index.</p>
          {note && <p className="mt-2 text-xs text-success">{note}</p>}
        </Field>
      </div>
    </Dialog>
  );
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="mb-1.5 text-[11px] font-semibold uppercase tracking-[0.1em] text-ink-muted">{label}</div>
      {children}
    </div>
  );
}
