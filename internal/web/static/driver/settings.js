// The driver's settings. They are kept on the server too, so a new phone
// starts the way the driver left the last one (AC-026.6). A change made
// here goes with the next sync, and until then the server's copy doesn't
// replace it.

import { updateTracking } from "./gps.js";
import { S } from "./state.js";
import { kvSet } from "./store.js";
import { scheduleSync } from "./sync.js";

export async function changeSetting(key, value) {
  S.settings = { ...S.settings, [key]: value };
  S.changed = { ...S.changed, [key]: value };
  S.settingsRev++;
  await kvSet("settings", S.settings);
  await kvSet("settingsChanged", S.changed);
  applySettings();
  scheduleSync(800);
}
function applySettings() {
  if (S.settings.language) S.lang = S.settings.language;
  document.documentElement.lang = S.lang;
  updateTracking();
}
// The server's copy, for each setting not changed here since.
export async function adoptSettings(server) {
  if (!server) return;
  const next = { ...S.settings };
  if (server.language && !("language" in S.changed)) next.language = server.language;
  if (!("battery_saver" in S.changed)) next.battery_saver = !!server.battery_saver;
  if (!("vibrate" in S.changed)) next.vibrate = server.vibrate !== false;
  if (JSON.stringify(next) === JSON.stringify(S.settings)) return;
  S.settings = next;
  await kvSet("settings", S.settings);
  applySettings();
}
