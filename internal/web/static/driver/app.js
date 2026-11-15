// Kiflet driver app. Offline-first: everything the driver does is written to
// IndexedDB first and sent to the server by a background sync loop, so the
// app behaves the same with or without signal.
(() => {
  "use strict";

  const VERSION = "1.2.0";
  const API = "/api/v1";
  const ON_ROAD = ["in_transit", "delayed", "arrived", "issue"];
  const ACTIVE = ["assigned", ...ON_ROAD];
  const MAX_PHOTOS = 3;
  const DEFAULT_SETTINGS = { language: "", battery_saver: false, vibrate: true };

  // ---------------------------------------------------------------------------
  // Text. Amharic is the default for drivers whose phone is set to it.
  // ---------------------------------------------------------------------------
  const TEXT = {
    en: {
      signIn: "Sign in", phone: "Phone number", pin: "PIN", forgotPin: "Forgot your PIN?",
      signInHelp: "Use the phone number your company registered and the PIN you received by SMS.",
      sendCode: "Send me a code", code: "Code from SMS", newPin: "New PIN (4–6 digits)", newPinAgain: "New PIN again",
      setPin: "Save PIN", choosePin: "Choose your PIN", choosePinHelp: "Pick a PIN only you know. You'll use it to sign in.",
      currentPin: "Current PIN", pinsDiffer: "The PINs don't match.", back: "Back", cancel: "Cancel", save: "Save", send: "Send",
      myShipments: "My shipments", active: "Active", done: "Finished this week", none: "No shipments assigned to you right now.",
      noneHelp: "New work appears here automatically when the office assigns it.",
      offline: "No signal", syncing: "Sending…", waiting: "{n} waiting to send", allSent: "All sent", lastSync: "Updated {t}",
      neverSynced: "Not synced yet", syncNow: "Sync now", from: "From", to: "To", due: "Due", pickup: "Pickup", customer: "Customer",
      cargo: "Cargo", instructions: "Instructions from the office", call: "Call", navigate: "Directions", vehicle: "Vehicle",
      startTrip: "Start trip", backOnRoad: "Back on the road", arrivedBtn: "I've arrived", delayBtn: "Report a delay",
      problemBtn: "Report a problem", deliverBtn: "Record delivery", messageBtn: "Message the office",
      updateDelay: "Update the delay", updateProblem: "Update the problem",
      pendingSend: "Saved on the phone — will send when there's signal", reason: "Reason", note: "Note (optional)",
      recipient: "Received by (name)", recipientPhone: "Their phone (optional)", signature: "Signature",
      signHere: "Ask the recipient to sign here", clear: "Clear", photos: "Photos (up to 3)", addPhoto: "Take photo",
      remove: "Remove", deliveryNotes: "Notes (optional)", confirmDelivery: "Confirm delivery",
      needName: "Enter who received the cargo.", needSignature: "The recipient must sign.",
      locating: "Getting your location…", noFix: "Couldn't get a GPS fix. The delivery is saved without a location.",
      delivered: "Delivery saved.", statusSaved: "Saved.", messageSaved: "Message saved. It goes with the next sync.",
      messages: "From the office", messagePlaceholder: "e.g. Road closed at Awash, taking the long route",
      menu: "Menu", language: "Language", location: "Share my location while working", locationHelp: "The office sees where you are while you have active shipments. Your location is never shown to customers.",
      changePin: "Change PIN", signOut: "Sign out", signOutPending: "{n} updates haven't been sent yet. If you sign out now they are lost.",
      signOutAnyway: "Sign out anyway", smsHelp: "No data? Text {ref} and a code to {num}: 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem.",
      smsHelpGeneric: "No data? Text the shipment's last 4 digits and a code to {num}: 1 departed, 2 delayed, 3 arrived, 4 delivered, 5 problem.",
      rejected: "Not applied", dismiss: "Dismiss", priority_high: "High priority", priority_urgent: "Urgent",
      signedOut: "You've been signed out. Sign in again.", networkError: "No connection. Try again when you have signal.",
      codeSent: "If this number belongs to a driver, a code is on its way.", version: "Version", deliveredAt: "Delivered",
      shipmentGone: "This shipment is no longer assigned to you.",
      msgCancelled: "The office cancelled this shipment.", msgReopened: "The office reopened this shipment.",
      blocked: "Your company's account is paused. Call your office. What you record is kept on this phone and sent when the account is back.",
      draftRestored: "Your unfinished delivery is back as you left it.", install: "Install the app", installHelp: "Add it to your home screen so it opens without the browser.",
      locationOff: "Location is off", battery: "Battery", appUpdated: "App updated.", pinChanged: "PIN changed.",
      kg: "kg", m: "m", km: "km", weak: "Weak signal", settings: "Settings", photoUnreadable: "The photo couldn't be read. Take it again.",
      batterySaver: "Battery saver", batterySaverHelp: "Records your position every 15 minutes and syncs less often. It turns on by itself when the battery is below 20%.",
      savingNow: "Saving battery now: the battery is low.", vibrate: "Vibrate when the office writes",
      keepAwake: "Keep the screen on while charging", keepAwakeHelp: "Your position keeps updating on long drives. Only while the phone is plugged in.",
      storage: "Storage on this phone", storageUse: "{used} used, room for {free} more",
      storageSafe: "Saved work is protected from automatic clean-up.",
      storageUnsafe: "The phone may delete saved work when space runs low. Install the app to protect it.",
      storageLow: "Your phone is almost full. Deliveries not sent yet are kept, but free up space so new photos can be saved.",
      clearData: "Clear saved data", clearDataHelp: "Removes shipments and messages saved on this phone, then downloads your current work again. Updates not sent yet are kept.",
      clearDataConfirm: "Clear saved data now?", clearDataDone: "Saved data cleared.", clearDataOffline: "Needs signal, so your current work can download again.",
      help: "Help", callOffice: "Call the office",
      awayFrom: "{d} {dir} of you, in a straight line", fixFrom: "From your position {t}",
      dir_n: "north", dir_ne: "north-east", dir_e: "east", dir_se: "south-east", dir_s: "south", dir_sw: "south-west", dir_w: "west", dir_nw: "north-west",
      farTitle: "You're {d} from the drop-off", farHelp: "Deliveries should be recorded within {tol} of it. Go closer and try again, or say why you're delivering here.",
      whyHere: "Why here?", deliverAnyway: "Deliver here anyway", goBack: "Go back", needWhy: "Choose why you're delivering here.", needWhyNote: "Say why in the notes.",
      suggested: "Suggestions", quickNotes: "Quick notes", quickMessages: "Quick messages", you: "You", dropOff: "Drop-off",
      locationOffHere: "Location sharing is off, so the distance isn't shown.",
      note_security: "Left with security guard", note_reception: "Left at reception", note_partial: "Partial delivery", note_damaged: "Package damaged upon arrival",
      quick_onway: "On my way", quick_unloading: "Waiting to unload", quick_callme: "Please call me", quick_roadclosed: "The road ahead is closed",
    },
    am: {
      signIn: "ግባ", phone: "ስልክ ቁጥር", pin: "ፒን", forgotPin: "ፒንዎን ረሱ?",
      signInHelp: "ድርጅትዎ የመዘገበውን ስልክ ቁጥር እና በኤስኤምኤስ የደረሰዎትን ፒን ያስገቡ።",
      sendCode: "ኮድ ላክልኝ", code: "በኤስኤምኤስ የመጣው ኮድ", newPin: "አዲስ ፒን (4–6 አሃዝ)", newPinAgain: "አዲሱን ፒን ይድገሙ",
      setPin: "ፒን አስቀምጥ", choosePin: "ፒንዎን ይምረጡ", choosePinHelp: "እርስዎ ብቻ የሚያውቁትን ፒን ይምረጡ።",
      currentPin: "የአሁኑ ፒን", pinsDiffer: "ፒኖቹ አይመሳሰሉም።", back: "ተመለስ", cancel: "ተው", save: "አስቀምጥ", send: "ላክ",
      myShipments: "ጭነቶቼ", active: "በሂደት ላይ", done: "በዚህ ሳምንት የተጠናቀቁ", none: "አሁን የተመደበልዎት ጭነት የለም።",
      noneHelp: "ቢሮው ሥራ ሲመድብልዎ እዚህ ይታያል።",
      offline: "ኔትወርክ የለም", syncing: "በመላክ ላይ…", waiting: "{n} ለመላክ በመጠባበቅ ላይ", allSent: "ሁሉም ተልኳል", lastSync: "የተዘመነው {t}",
      neverSynced: "ገና አልተመሳሰለም", syncNow: "አሁን አመሳስል", from: "ከ", to: "ወደ", due: "የሚደርስበት", pickup: "የሚጫንበት", customer: "ደንበኛ",
      cargo: "ጭነት", instructions: "ከቢሮ የተሰጠ መመሪያ", call: "ደውል", navigate: "አቅጣጫ", vehicle: "ተሽከርካሪ",
      startTrip: "ጉዞ ጀምር", backOnRoad: "ጉዞ ቀጥል", arrivedBtn: "ደርሻለሁ", delayBtn: "መዘግየት አሳውቅ",
      problemBtn: "ችግር አሳውቅ", deliverBtn: "ርክክብ መዝግብ", messageBtn: "ለቢሮ መልዕክት",
      updateDelay: "መዘግየቱን አዘምን", updateProblem: "ችግሩን አዘምን",
      pendingSend: "በስልኩ ተቀምጧል — ኔትወርክ ሲገኝ ይላካል", reason: "ምክንያት", note: "ማስታወሻ (አማራጭ)",
      recipient: "የተረከበው ሰው ስም", recipientPhone: "ስልኩ (አማራጭ)", signature: "ፊርማ",
      signHere: "ተረካቢው እዚህ ይፈርሙ", clear: "አጥፋ", photos: "ፎቶዎች (እስከ 3)", addPhoto: "ፎቶ አንሳ",
      remove: "አስወግድ", deliveryNotes: "ማስታወሻ (አማራጭ)", confirmDelivery: "ርክክቡን አረጋግጥ",
      needName: "ጭነቱን የተረከበውን ሰው ስም ያስገቡ።", needSignature: "ተረካቢው መፈረም አለበት።",
      locating: "አካባቢዎን በማግኘት ላይ…", noFix: "ጂፒኤስ አልተገኘም። ርክክቡ ያለ አካባቢ ተቀምጧል።",
      delivered: "ርክክቡ ተቀምጧል።", statusSaved: "ተቀምጧል።", messageSaved: "መልዕክቱ ተቀምጧል። በሚቀጥለው ማመሳሰል ይላካል።",
      messages: "ከቢሮ", messagePlaceholder: "ለምሳሌ፦ አዋሽ ላይ መንገድ ተዘግቷል",
      menu: "ምናሌ", language: "ቋንቋ", location: "በሥራ ላይ ሳለሁ አካባቢዬን አጋራ", locationHelp: "ንቁ ጭነት ሲኖርዎት ቢሮው የት እንዳሉ ያያል። አካባቢዎ ለደንበኞች አይታይም።",
      changePin: "ፒን ቀይር", signOut: "ውጣ", signOutPending: "{n} ያልተላኩ መረጃዎች አሉ። አሁን ከወጡ ይጠፋሉ።",
      signOutAnyway: "ቢሆንም ውጣ", smsHelp: "ዳታ የለም? {ref} እና ኮድ ወደ {num} ይላኩ፦ 1 ተነሳ፣ 2 ዘገየ፣ 3 ደረሰ፣ 4 ተረከበ፣ 5 ችግር።",
      smsHelpGeneric: "ዳታ የለም? የጭነቱን የመጨረሻ 4 አሃዝ እና ኮድ ወደ {num} ይላኩ፦ 1 ተነሳ፣ 2 ዘገየ፣ 3 ደረሰ፣ 4 ተረከበ፣ 5 ችግር።",
      rejected: "አልተተገበረም", dismiss: "ዝጋ", priority_high: "ከፍተኛ ቅድሚያ", priority_urgent: "አስቸኳይ",
      signedOut: "ከመለያዎ ወጥተዋል። እንደገና ይግቡ።", networkError: "ግንኙነት የለም። ኔትወርክ ሲኖር እንደገና ይሞክሩ።",
      codeSent: "ቁጥሩ የሹፌር ከሆነ ኮድ ይላካል።", version: "ስሪት", deliveredAt: "የተረከበው",
      shipmentGone: "ይህ ጭነት ከእንግዲህ ለእርስዎ አልተመደበም።",
      msgCancelled: "ቢሮው ይህን ጭነት ሰርዞታል።", msgReopened: "ቢሮው ይህን ጭነት እንደገና ከፍቶታል።",
      blocked: "የድርጅትዎ መለያ ለጊዜው ቆሟል። ቢሮዎን ይደውሉ። የሚመዘግቡት በዚህ ስልክ ተቀምጦ መለያው ሲመለስ ይላካል።",
      draftRestored: "ያልጨረሱት የማስረከቢያ ቅጽ እንደተዉት ተመልሷል።", install: "መተግበሪያውን ጫን", installHelp: "ያለ ብሮውዘር እንዲከፈት ወደ መነሻ ገጽ ያክሉት።",
      locationOff: "አካባቢ ጠፍቷል", battery: "ባትሪ", appUpdated: "መተግበሪያው ተዘምኗል።", pinChanged: "ፒን ተቀይሯል።",
      kg: "ኪ.ግ", m: "ሜ", km: "ኪ.ሜ", weak: "ደካማ ኔትወርክ", settings: "ቅንብሮች", photoUnreadable: "ፎቶው ሊነበብ አልቻለም። እንደገና ያንሱ።",
      batterySaver: "ባትሪ ቆጣቢ", batterySaverHelp: "አካባቢዎን በየ15 ደቂቃው ይመዘግባል፣ ብዙ ጊዜም አያመሳስልም። ባትሪው ከ20% በታች ሲሆን በራሱ ይበራል።",
      savingNow: "አሁን ባትሪ እየቆጠበ ነው፦ ባትሪው ዝቅተኛ ነው።", vibrate: "ቢሮው ሲጽፍ ይንዘር",
      keepAwake: "ቻርጅ ላይ ሲሆን ስክሪኑ አይጥፋ", keepAwakeHelp: "በረጅም ጉዞ አካባቢዎ መዘመኑን ይቀጥላል። ስልኩ ቻርጅ ላይ ሲሆን ብቻ።",
      storage: "በዚህ ስልክ ያለ ማከማቻ", storageUse: "{used} ጥቅም ላይ ውሏል፣ ተጨማሪ {free} ቦታ አለ",
      storageSafe: "የተቀመጠው ሥራ በራሱ እንዳይጠፋ ተጠብቋል።",
      storageUnsafe: "ቦታ ሲያንስ ስልኩ የተቀመጠ ሥራ ሊያጠፋ ይችላል። ለመጠበቅ መተግበሪያውን ይጫኑ።",
      storageLow: "ስልክዎ ሊሞላ ነው። ያልተላኩ ርክክቦች ይቀመጣሉ፣ ነገር ግን አዲስ ፎቶ እንዲቀመጥ ቦታ ያስለቅቁ።",
      clearData: "የተቀመጠ መረጃ አጥፋ", clearDataHelp: "በዚህ ስልክ የተቀመጡ ጭነቶችንና መልዕክቶችን ያጠፋል፣ ከዚያ የአሁኑን ሥራዎን እንደገና ያወርዳል። ያልተላኩ መረጃዎች ይቀመጣሉ።",
      clearDataConfirm: "የተቀመጠውን መረጃ አሁን ላጥፋ?", clearDataDone: "የተቀመጠው መረጃ ጠፍቷል።", clearDataOffline: "የአሁኑ ሥራዎ እንደገና እንዲወርድ ኔትወርክ ያስፈልጋል።",
      help: "እርዳታ", callOffice: "ለቢሮ ደውል",
      awayFrom: "ከእርስዎ በስተ{dir} {d}፣ በቀጥታ መስመር", fixFrom: "ከ{t} አካባቢዎ",
      dir_n: "ሰሜን", dir_ne: "ሰሜን ምሥራቅ", dir_e: "ምሥራቅ", dir_se: "ደቡብ ምሥራቅ", dir_s: "ደቡብ", dir_sw: "ደቡብ ምዕራብ", dir_w: "ምዕራብ", dir_nw: "ሰሜን ምዕራብ",
      farTitle: "ከመድረሻው {d} ርቀዋል", farHelp: "ርክክብ ከመድረሻው በ{tol} ውስጥ መመዝገብ አለበት። ቀረብ ብለው እንደገና ይሞክሩ፣ ወይም ለምን እዚህ እንደሚያስረክቡ ይግለጹ።",
      whyHere: "ለምን እዚህ?", deliverAnyway: "ቢሆንም እዚህ አስረክብ", goBack: "ተመለስ", needWhy: "ለምን እዚህ እንደሚያስረክቡ ይምረጡ።", needWhyNote: "ምክንያቱን በማስታወሻው ይጻፉ።",
      suggested: "ጥቆማዎች", quickNotes: "ፈጣን ማስታወሻ", quickMessages: "ፈጣን መልዕክቶች", you: "እርስዎ", dropOff: "መድረሻ",
      locationOffHere: "አካባቢ ማጋራት ስለጠፋ ርቀቱ አይታይም።",
      note_security: "ለጥበቃ ተሰጥቷል", note_reception: "ለእንግዳ መቀበያ ተሰጥቷል", note_partial: "ከፊል ርክክብ", note_damaged: "ጭነቱ ሲደርስ ተጎድቶ ነበር",
      quick_onway: "በመንገድ ላይ ነኝ", quick_unloading: "ለማውረድ እየጠበቅሁ ነው", quick_callme: "እባክዎ ይደውሉልኝ", quick_roadclosed: "ከፊት ያለው መንገድ ተዘግቷል",
    },
  };
  const STATUS_AM = {
    pending: "በመጠባበቅ ላይ", assigned: "ተመድቧል", in_transit: "በጉዞ ላይ", delayed: "ዘግይቷል",
    arrived: "መድረሻ ደርሷል", delivered: "ተረክቧል", issue: "ችግር አለ", cancelled: "ተሰርዟል",
  };
  const REASON_AM = {
    road_conditions: "የመንገድ ሁኔታ", weather: "የአየር ሁኔታ", security_checkpoint: "የፍተሻ ኬላ", mechanical_issue: "የመኪና ብልሽት",
    traffic: "የትራፊክ መጨናነቅ", accident: "አደጋ", breakdown: "መኪናው ተበላሽቷል", recipient_unavailable: "ተቀባዩ የለም",
    wrong_address: "የተሳሳተ አድራሻ", cargo_damaged: "ጭነቱ ተጎድቷል", recipient_refused: "ተቀባዩ አልተቀበለም", other: "ሌላ",
    met_elsewhere: "ተቀባዩ በሌላ ቦታ ተቀበለ", address_wrong: "በጭነቱ ላይ ያለው መድረሻ ተሳስቷል", gps_inaccurate: "እዚህ የስልኩ አካባቢ ትክክል አይደለም",
  };

  // ---------------------------------------------------------------------------
  // Storage
  // ---------------------------------------------------------------------------
  const DB_NAME = "kiflet-driver";
  let dbp = null;
  function idb() {
    if (dbp) return dbp;
    dbp = new Promise((resolve, reject) => {
      const req = indexedDB.open(DB_NAME, 1);
      req.onupgradeneeded = () => {
        const d = req.result;
        d.createObjectStore("kv");
        d.createObjectStore("shipments", { keyPath: "id" });
        d.createObjectStore("outbox", { keyPath: "client_id" });
        d.createObjectStore("locations", { autoIncrement: true });
      };
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
    return dbp;
  }
  async function tx(store, mode, fn) {
    const d = await idb();
    return new Promise((resolve, reject) => {
      const t = d.transaction(store, mode);
      const s = t.objectStore(store);
      let out;
      Promise.resolve(fn(s)).then((v) => (out = v));
      t.oncomplete = () => resolve(out);
      t.onerror = () => reject(t.error);
      t.onabort = () => reject(t.error);
    });
  }
  const req2p = (r) => new Promise((res, rej) => ((r.onsuccess = () => res(r.result)), (r.onerror = () => rej(r.error))));
  const kvGet = (k) => tx("kv", "readonly", (s) => req2p(s.get(k)));
  const kvSet = (k, v) => tx("kv", "readwrite", (s) => s.put(v, k));
  const all = (store) => tx(store, "readonly", (s) => req2p(s.getAll()));
  const put = (store, v) => tx(store, "readwrite", (s) => s.put(v));
  const del = (store, k) => tx(store, "readwrite", (s) => s.delete(k));
  const clear = (store) => tx(store, "readwrite", (s) => s.clear());
  // Delivery forms being filled in, kept under draft:<shipment id>.
  const dropDrafts = () => tx("kv", "readwrite", (s) => req2p(s.getAllKeys()).then((keys) =>
    keys.filter((k) => String(k).startsWith("draft:")).forEach((k) => s.delete(k))));

  // ---------------------------------------------------------------------------
  // State
  // ---------------------------------------------------------------------------
  const S = {
    token: null, driver: null, config: null, deviceId: null, cursor: null, lang: "en",
    shipments: new Map(), outbox: [], messages: [], seen: new Set(), notices: [],
    tracking: true, lastFix: null, lastRecorded: null, battery: null, charging: false, pendingLocations: 0,
    online: navigator.onLine, weak: false, syncing: false, lastSync: null, failures: 0, installPrompt: null,
    // The driver's choices, kept on the server too. changed: what was
    // changed here since the server last heard; rev tells a change made
    // during a sync.
    settings: { ...DEFAULT_SETTINGS }, changed: {}, settingsRev: 0,
    keepAwake: false, storage: null, persisted: false, parked: false, anchor: null,
    // blocked: the company's account is suspended, so syncs are refused.
    blocked: false,
  };

  function t(key, vars) {
    let s = (TEXT[S.lang] && TEXT[S.lang][key]) || TEXT.en[key] || key;
    for (const [k, v] of Object.entries(vars || {})) s = s.replace("{" + k + "}", v);
    return s;
  }
  // A message as shown: what the office did to a shipment is said in the
  // driver's language, with the office's note after it.
  function msgText(m) {
    if (m.kind === "cancelled" || m.kind === "reopened") {
      const said = t(m.kind === "cancelled" ? "msgCancelled" : "msgReopened");
      return m.note ? said + " " + m.note : said;
    }
    return m.text;
  }
  function statusLabel(st) {
    if (S.lang === "am" && STATUS_AM[st]) return STATUS_AM[st];
    const found = S.config && S.config.statuses.find((x) => x.code === st);
    return found ? found.label : st;
  }
  function reasonLabel(code) {
    if (S.lang === "am" && REASON_AM[code]) return REASON_AM[code];
    const list = S.config ? [...S.config.delay_reasons, ...S.config.issue_reasons, ...(S.config.location_reasons || [])] : [];
    const found = list.find((x) => x.code === code);
    return found ? found.label : code;
  }

  function uuid() {
    if (crypto.randomUUID) return crypto.randomUUID();
    const b = crypto.getRandomValues(new Uint8Array(16));
    b[6] = (b[6] & 0x0f) | 0x40;
    b[8] = (b[8] & 0x3f) | 0x80;
    const h = [...b].map((x) => x.toString(16).padStart(2, "0")).join("");
    return `${h.slice(0, 8)}-${h.slice(8, 12)}-${h.slice(12, 16)}-${h.slice(16, 20)}-${h.slice(20)}`;
  }

  // ---------------------------------------------------------------------------
  // DOM
  // ---------------------------------------------------------------------------
  function h(tag, props, ...children) {
    const n = document.createElement(tag);
    for (const [k, v] of Object.entries(props || {})) {
      if (v == null || v === false) continue;
      if (k === "class") n.className = v;
      else if (k === "text") n.textContent = v;
      else if (k.startsWith("on")) n.addEventListener(k.slice(2), v);
      else if (v === true) n.setAttribute(k, "");
      else n.setAttribute(k, v);
    }
    for (const c of children.flat()) {
      if (c == null || c === false) continue;
      n.append(c instanceof Node ? c : document.createTextNode(String(c)));
    }
    return n;
  }
  const root = () => document.getElementById("app");

  function toast(msg, kind) {
    let box = document.querySelector(".toasts");
    if (!box) document.body.append((box = h("div", { class: "toasts", "aria-live": "polite" })));
    const n = h("div", { class: "toast " + (kind || ""), role: "status", text: msg });
    box.append(n);
    setTimeout(() => n.remove(), 4500);
  }

  function fmtTime(iso) {
    if (!iso) return "";
    const d = new Date(iso);
    const now = new Date();
    const opts = { hour: "2-digit", minute: "2-digit" };
    if (d.toDateString() !== now.toDateString()) Object.assign(opts, { weekday: "short", day: "numeric", month: "short" });
    return d.toLocaleString(S.lang === "am" ? "am-ET" : "en-GB", opts);
  }
  function fmtDist(m) {
    if (m < 1000) return Math.max(10, Math.round(m / 10) * 10) + " " + t("m");
    return (m < 10000 ? (m / 1000).toFixed(1) : Math.round(m / 1000)) + " " + t("km");
  }
  function fmtBytes(b) {
    if (b < 1048576) return Math.max(1, Math.round(b / 1024)) + " KB";
    if (b < 1073741824) return (b / 1048576).toFixed(1) + " MB";
    return (b / 1073741824).toFixed(1) + " GB";
  }
  function ago(ms) {
    const s = (Date.now() - ms) / 1000;
    if (s < 60) return S.lang === "am" ? "አሁን" : "just now";
    if (s < 3600) return Math.floor(s / 60) + (S.lang === "am" ? " ደቂቃ በፊት" : " min ago");
    return Math.floor(s / 3600) + (S.lang === "am" ? " ሰዓት በፊት" : " h ago");
  }

  // ---------------------------------------------------------------------------
  // Server
  // ---------------------------------------------------------------------------
  class ApiError extends Error {
    constructor(status, body) {
      super((body && body.error && body.error.message) || "Request failed");
      this.status = status;
      this.code = body && body.error && body.error.code;
      this.field = body && body.error && body.error.field;
    }
  }

  async function gzip(text) {
    if (!("CompressionStream" in window)) return null;
    const stream = new Blob([text]).stream().pipeThrough(new CompressionStream("gzip"));
    return new Response(stream).arrayBuffer();
  }

  async function api(path, { method = "GET", body, auth = true, compress = false, timeout = 30000 } = {}) {
    const headers = { Accept: "application/json" };
    let payload;
    if (body !== undefined) {
      headers["Content-Type"] = "application/json";
      payload = JSON.stringify(body);
      if (compress && payload.length > 2048) {
        const gz = await gzip(payload);
        if (gz) {
          payload = gz;
          headers["Content-Encoding"] = "gzip";
        }
      }
    }
    if (auth && S.token) headers.Authorization = "Bearer " + S.token;
    const ctl = new AbortController();
    const timer = setTimeout(() => ctl.abort(), timeout);
    try {
      const res = await fetch(API + path, { method, headers, body: payload, cache: "no-store", signal: ctl.signal });
      let data = null;
      try {
        data = await res.json();
      } catch (e) {
        if (ctl.signal.aborted) throw e; // empty bodies are fine, cut-off ones aren't
      }
      if (!res.ok) throw new ApiError(res.status, data);
      return data;
    } finally {
      clearTimeout(timer);
    }
  }

  // ---------------------------------------------------------------------------
  // Outbox: every change the driver makes, until the server has it.
  // ---------------------------------------------------------------------------
  async function enqueue(kind, payload) {
    const item = { client_id: uuid(), kind, payload: { ...payload }, created: Date.now() };
    item.payload.client_id = item.client_id;
    await put("outbox", item);
    S.outbox.push(item);
    renderPill();
    scheduleSync(800);
    return item;
  }

  // effective returns the shipment as the driver sees it: the server's copy
  // with any changes still waiting in the outbox applied on top.
  function effective(sh) {
    const out = { ...sh, pending: false };
    const mine = S.outbox
      .filter((o) => o.payload.shipment_id === sh.id && (o.kind === "status" || o.kind === "pod"))
      .sort((a, b) => a.payload.recorded_at.localeCompare(b.payload.recorded_at));
    for (const o of mine) {
      out.pending = true;
      if (o.kind === "status") {
        out.status = o.payload.status;
        out.status_reason = o.payload.reason || "";
      } else {
        out.status = "delivered";
        out.delivered_at = o.payload.recorded_at;
      }
    }
    return out;
  }

  function fixForEvent(maxAgeMs = 10 * 60 * 1000) {
    const f = S.lastFix;
    if (f && Date.now() - f.time < maxAgeMs) return { lat: f.lat, lng: f.lng };
    return { lat: null, lng: null };
  }

  function freshFix(timeout = 10000) {
    return new Promise((resolve) => {
      if (!navigator.geolocation) return resolve(null);
      navigator.geolocation.getCurrentPosition(
        (p) => {
          S.lastFix = { lat: p.coords.latitude, lng: p.coords.longitude, acc: p.coords.accuracy, time: Date.now() };
          resolve(S.lastFix);
        },
        () => resolve(null),
        { enableHighAccuracy: true, timeout, maximumAge: 60000 },
      );
    });
  }

  // ---------------------------------------------------------------------------
  // Settings. They are kept on the server too, so a new phone starts the way
  // the driver left the last one (AC-026.6). A change made here goes with the
  // next sync, and until then the server's copy doesn't replace it.
  // ---------------------------------------------------------------------------
  async function changeSetting(key, value) {
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
  async function adoptSettings(server) {
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

  // ---------------------------------------------------------------------------
  // Storage. The browser is asked to keep this app's data when space runs
  // low, so unsent deliveries survive (AC-014.7), and the office sees how much
  // the app holds (AC-014.6).
  // ---------------------------------------------------------------------------
  async function storageEstimate() {
    if (!navigator.storage || !navigator.storage.estimate) return null;
    try {
      const e = await navigator.storage.estimate();
      return { usage: e.usage || 0, quota: e.quota || 0 };
    } catch {
      return null;
    }
  }
  // Short of space: under 50 MB left, or more than 80% of the allowance used.
  function storageLow() {
    const st = S.storage;
    return !!(st && st.quota && (st.quota - st.usage < 50 * 1048576 || st.usage / st.quota > 0.8));
  }
  async function protectStorage() {
    if (!navigator.storage || !navigator.storage.persist) return;
    try {
      S.persisted = (await navigator.storage.persisted()) || (await navigator.storage.persist());
    } catch {
      S.persisted = false;
    }
  }

  // ---------------------------------------------------------------------------
  // Sync
  // ---------------------------------------------------------------------------
  let syncTimer = null;
  function scheduleSync(ms) {
    clearTimeout(syncTimer);
    syncTimer = setTimeout(sync, ms);
  }

  async function sync() {
    clearTimeout(syncTimer);
    if (!S.token || S.syncing) return;
    if (!navigator.onLine) {
      S.online = false;
      renderPill();
      scheduleSync(30000);
      return;
    }
    S.syncing = true;
    renderPill();
    try {
      const statuses = S.outbox.filter((o) => o.kind === "status").map((o) => o.payload);
      const pods = S.outbox.filter((o) => o.kind === "pod").slice(0, S.weak ? 1 : 2).map((o) => o.payload);
      const messages = S.outbox.filter((o) => o.kind === "message").map((o) => o.payload);
      const locKeys = [];
      const locations = [];
      await tx("locations", "readonly", (s) => {
        const r = s.openCursor();
        return new Promise((resolve) => {
          r.onsuccess = () => {
            const c = r.result;
            if (!c || locations.length >= 1000) return resolve();
            locKeys.push(c.key);
            locations.push(c.value);
            c.continue();
          };
        });
      });
      S.storage = await storageEstimate();
      const rev = S.settingsRev;
      const changed = Object.keys(S.changed).length ? S.changed : undefined;
      const body = {
        device_id: S.deviceId, app_version: VERSION, device_time: new Date().toISOString(),
        battery: S.battery, pending: S.outbox.length, tracking_paused: !S.tracking, power_saving: saving(),
        storage_kb: S.storage ? Math.round(S.storage.usage / 1024) : null,
        settings: changed,
        since: S.cursor, statuses, pods, locations, messages,
      };
      const started = Date.now();
      // Long enough for photos over a slow connection; a stalled one ends.
      const resp = await api("/driver/sync", { method: "POST", body, compress: true, timeout: pods.length ? 180000 : 60000 });
      if (changed && rev === S.settingsRev) {
        S.changed = {};
        await kvSet("settingsChanged", {});
      }
      await applySync(resp, locKeys);
      S.online = true;
      S.failures = 0;
      // A round trip this slow means a weak signal (AC-015.1). Photos take
      // longer anyway.
      S.weak = Date.now() - started > (pods.length ? 60000 : 10000);
      S.lastSync = Date.now();
      await kvSet("lastSync", S.lastSync);
      // More waiting (PODs go two at a time, or more messages than one
      // response carries): carry on straight away. Saving battery, the app
      // syncs every five minutes unless the driver acts.
      const more = resp.more || S.outbox.some((o) => o.kind === "pod");
      scheduleSync(more ? 500 : saving() ? 300000 : 60000);
      if (S.blocked) {
        S.blocked = false;
        rerender();
      }
    } catch (err) {
      S.failures++;
      if (err instanceof ApiError && err.status === 401) {
        await signedOut();
        return;
      }
      // The company's account is suspended: say so, and ask again now and
      // then rather than every few seconds. Nothing recorded is dropped.
      if (err instanceof ApiError && err.status === 403) {
        if (!S.blocked) {
          S.blocked = true;
          rerender();
        }
        scheduleSync(600000);
        return;
      }
      // The phone says it is online, but the server can't be reached.
      if (!(err instanceof ApiError)) {
        if (!navigator.onLine) S.online = false;
        else if (S.failures >= 2) S.weak = true;
      }
      scheduleSync(Math.min(300000, 15000 * 2 ** Math.min(S.failures, 4)));
    } finally {
      S.syncing = false;
      renderPill();
    }
  }

  async function applySync(resp, locKeys) {
    // Outbox: drop everything the server accepted or refused for good. What
    // was refused stays on screen until the driver dismisses it, even if
    // the app restarts.
    let noticesChanged = false;
    const byId = new Map(S.outbox.map((o) => [o.client_id, o]));
    for (const r of resp.results || []) {
      const item = byId.get(r.client_id);
      if (!item) continue;
      if (r.ok || !r.retry) {
        await del("outbox", r.client_id);
        byId.delete(r.client_id);
        if (!r.ok && !S.notices.some((n) => n.id === r.client_id)) {
          const sh = S.shipments.get(item.payload.shipment_id);
          S.notices.push({ id: r.client_id, text: (sh ? sh.ref + ": " : "") + r.message });
          noticesChanged = true;
        }
      }
    }
    S.outbox = [...byId.values()];
    if (noticesChanged) await kvSet("notices", S.notices);
    if (locKeys.length) {
      await tx("locations", "readwrite", (s) => locKeys.forEach((k) => s.delete(k)));
      S.pendingLocations = Math.max(0, S.pendingLocations - locKeys.length);
    }

    // Shipments: the server sends what the device should hold.
    for (const sh of resp.shipments || []) {
      S.shipments.set(sh.id, sh);
      await put("shipments", sh);
    }
    for (const id of resp.removed_shipment_ids || []) {
      if (S.shipments.has(id)) {
        S.shipments.delete(id);
        await del("shipments", id);
        await del("kv", "draft:" + id);
        if (location.hash.includes(id)) toast(t("shipmentGone"));
      }
    }
    const weekAgo = Date.now() - 7 * 864e5;
    for (const sh of S.shipments.values()) {
      if (!ACTIVE.includes(sh.status) && new Date(sh.updated_at).getTime() < weekAgo) {
        S.shipments.delete(sh.id);
        await del("shipments", sh.id);
      }
    }

    // The driver's settings first: they say whether a new message vibrates.
    if (resp.driver) {
      S.driver = { ...S.driver, ...resp.driver };
      await kvSet("driver", S.driver);
      await adoptSettings(resp.driver.settings);
    }

    // Messages from the office. The toast shows the newest that is new to
    // the phone, not whatever happens to be newest.
    let newest = null;
    const known = new Set(S.messages.map((m) => m.id));
    for (const m of resp.messages || []) {
      if (!known.has(m.id)) {
        known.add(m.id);
        S.messages.push(m);
        if (!S.seen.has(m.id) && (!newest || m.at > newest.at)) newest = m;
      }
    }
    S.messages.sort((a, b) => b.at.localeCompare(a.at));
    S.messages = S.messages.slice(0, 50);
    await kvSet("messages", S.messages);
    if (newest) {
      toast(t("messages") + ": " + msgText(newest));
      if (S.settings.vibrate && navigator.vibrate) navigator.vibrate([120, 80, 120]);
    }
    S.cursor = resp.cursor;
    await kvSet("cursor", S.cursor);
    updateTracking();
    rerender();
  }

  // The session was ended elsewhere: the office signed this phone out, or
  // the PIN was reset. Customers' names and numbers go with it. What the
  // driver recorded and hasn't sent stays for their next sign-in; another
  // driver signing in clears it (signedIn).
  async function signedOut() {
    S.token = null;
    await kvSet("token", null);
    await forgetShipments();
    stopTracking();
    toast(t("signedOut"));
    go("#/");
    render();
  }

  async function forgetShipments() {
    await dropDrafts();
    await clear("shipments");
    S.shipments.clear();
    S.messages = [];
    S.cursor = null;
    await kvSet("messages", []);
    await kvSet("cursor", null);
  }

  // ---------------------------------------------------------------------------
  // GPS (FR-019). A position a minute while moving, one every five minutes
  // when parked. Battery saver, chosen or on by itself below 20% when not
  // charging, uses the coarse location and keeps one every 15 minutes. After
  // 10 minutes parked the GPS goes off, and a check every 5 minutes (15 when
  // saving) notices when the truck moves again.
  // ---------------------------------------------------------------------------
  const MOVED_M = 75;
  const PARKED_MS = 10 * 60000;
  let watchId = null, watchSaving = null, parkTimer = null;
  function saving() {
    return !!(S.settings.battery_saver || (S.battery != null && S.battery < 20 && !S.charging));
  }
  function onRoadShipment() {
    const list = [...S.shipments.values()].map(effective);
    return list.find((s) => ON_ROAD.includes(s.status)) || list.find((s) => s.status === "assigned");
  }
  function wantTracking() {
    return !!(S.token && S.tracking && navigator.geolocation && onRoadShipment());
  }
  function updateTracking() {
    if (!wantTracking()) return stopTracking();
    updateWakeLock();
    if (S.parked) return; // the parked checks hold the GPS until the truck moves
    const sv = saving();
    if (watchId != null && watchSaving === sv) return;
    if (watchId != null) navigator.geolocation.clearWatch(watchId);
    watchSaving = sv;
    watchId = navigator.geolocation.watchPosition(onFix, () => {}, sv
      ? { enableHighAccuracy: false, maximumAge: 300000, timeout: 120000 }
      : { enableHighAccuracy: true, maximumAge: 30000, timeout: 60000 });
  }
  function stopTracking() {
    if (watchId != null) navigator.geolocation.clearWatch(watchId);
    watchId = null;
    clearTimeout(parkTimer);
    S.parked = false;
    updateWakeLock();
  }
  const checkEvery = () => (saving() ? 15 : 5) * 60000;
  function park() {
    if (watchId != null) navigator.geolocation.clearWatch(watchId);
    watchId = null;
    S.parked = true;
    clearTimeout(parkTimer);
    parkTimer = setTimeout(parkedCheck, checkEvery());
  }
  // The driver doing something, or the truck moving, brings the GPS back.
  function unpark() {
    if (!S.parked) return;
    clearTimeout(parkTimer);
    S.parked = false;
    S.anchor = null;
    updateTracking();
  }
  function parkedCheck() {
    if (!wantTracking()) return stopTracking();
    const again = () => (parkTimer = setTimeout(parkedCheck, checkEvery()));
    navigator.geolocation.getCurrentPosition(async (p) => {
      const fix = fixOf(p);
      S.lastFix = fix;
      if (fix.acc <= maxAccuracy()) await record(p, fix);
      if (S.anchor && metres(S.anchor, fix) > Math.max(150, fix.acc)) return unpark();
      again();
    }, again, { enableHighAccuracy: !saving(), maximumAge: 60000, timeout: 60000 });
  }
  // Coarse positions are all battery saver gets on the open road.
  const maxAccuracy = () => (saving() ? 3000 : 500);
  const fixOf = (p) => ({ lat: p.coords.latitude, lng: p.coords.longitude, acc: p.coords.accuracy, time: Date.now() });

  // Keep the screen on while charging, if the driver asked: the browser
  // only tracks while the app is open.
  let wakeLock = null, wakeLockBusy = false;
  async function updateWakeLock() {
    const want = S.keepAwake && S.charging && !document.hidden && wantTracking() && "wakeLock" in navigator;
    if (wakeLockBusy || want === !!wakeLock) return;
    wakeLockBusy = true;
    try {
      if (want) {
        wakeLock = await navigator.wakeLock.request("screen");
        wakeLock.addEventListener("release", () => (wakeLock = null));
      } else {
        const w = wakeLock;
        wakeLock = null;
        await w.release();
      }
    } catch {
      wakeLock = null;
    } finally {
      wakeLockBusy = false;
    }
  }
  function metres(a, b) {
    const R = 6371000, rad = Math.PI / 180;
    const dLat = (b.lat - a.lat) * rad, dLng = (b.lng - a.lng) * rad;
    const x = Math.sin(dLat / 2) ** 2 + Math.cos(a.lat * rad) * Math.cos(b.lat * rad) * Math.sin(dLng / 2) ** 2;
    return 2 * R * Math.asin(Math.sqrt(x));
  }
  async function onFix(p) {
    const fix = fixOf(p);
    S.lastFix = fix;
    if (fix.acc > maxAccuracy()) return; // too vague to be useful
    // Parked: no further than 75 m from where the truck stopped, for 10 minutes.
    if (!S.anchor || metres(S.anchor, fix) > Math.max(MOVED_M, fix.acc)) S.anchor = fix;
    else if (fix.time - S.anchor.time > PARKED_MS) {
      await record(p, fix);
      return park();
    }
    const last = S.lastRecorded;
    const elapsed = last ? fix.time - last.time : Infinity;
    if (last) {
      if (saving() && elapsed < 15 * 60000) return;
      if (!saving() && !(elapsed >= 60000 && metres(last, fix) > MOVED_M) && elapsed < 300000) return;
    }
    await record(p, fix);
  }
  async function record(p, fix) {
    S.lastRecorded = fix;
    const sh = onRoadShipment();
    await tx("locations", "readwrite", (s) =>
      s.add({
        client_id: uuid(), shipment_id: sh ? sh.id : "", lat: fix.lat, lng: fix.lng,
        accuracy: fix.acc, speed: p.coords.speed != null ? p.coords.speed * 3.6 : null,
        heading: p.coords.heading, battery: S.battery, recorded_at: new Date(p.timestamp || fix.time).toISOString(),
      }),
    );
    S.pendingLocations++;
  }

  // ---------------------------------------------------------------------------
  // Router and screens
  // ---------------------------------------------------------------------------
  function go(hash) {
    if (location.hash !== hash) location.hash = hash;
  }
  let formOpen = false; // screens with forms are not redrawn by background syncs
  function rerender() {
    if (!formOpen) render();
    else renderPill();
  }

  function render() {
    const app = root();
    formOpen = false;
    if (!S.config) return app.replaceChildren(h("div", { class: "boot" }, h("p", { text: t("networkError") })));
    if (!S.token) return app.replaceChildren(screenLogin());
    if (S.driver && S.driver.must_change_pin) return app.replaceChildren(screenPin(true));
    const parts = location.hash.replace(/^#\/?/, "").split("/");
    let view;
    if (parts[0] === "s" && parts[1]) {
      const sh = S.shipments.get(parts[1]);
      if (!sh) view = screenList();
      else if (parts[2] === "deliver") view = screenDeliver(effective(sh));
      else if (parts[2] === "report") view = screenReport(effective(sh), parts[3]);
      else if (parts[2] === "message") view = screenMessage(effective(sh));
      else view = screenDetail(effective(sh));
    } else if (parts[0] === "menu") view = screenMenu();
    else if (parts[0] === "pin") view = screenPin(false);
    else view = screenList();
    app.replaceChildren(view);
    renderPill();
    window.scrollTo(0, 0);
  }

  function header(title, backHref) {
    return h("header", { class: "bar" },
      backHref ? h("a", { class: "bar-btn", href: backHref, "aria-label": t("back"), text: "‹" }) : h("span", { class: "bar-logo", "aria-hidden": "true" }),
      h("div", { class: "bar-title" }, h("strong", { text: title }), S.driver ? h("span", { class: "bar-sub", text: S.driver.company }) : null),
      h("button", { class: "pill-sync", type: "button", "data-pill": true, onclick: () => sync() }),
      h("a", { class: "bar-btn", href: "#/menu", "aria-label": t("menu"), text: "☰" }),
    );
  }

  function renderPill() {
    const pill = document.querySelector("[data-pill]");
    if (!pill) return;
    const waiting = S.outbox.length;
    let cls = "ok", txt = t("allSent");
    if (S.syncing) (cls = "busy"), (txt = t("syncing"));
    else if (!S.online || !navigator.onLine) (cls = "off"), (txt = waiting ? t("waiting", { n: waiting }) : t("offline"));
    else if (S.weak) (cls = "weak"), (txt = waiting ? t("waiting", { n: waiting }) : t("weak"));
    else if (waiting) (cls = "wait"), (txt = t("waiting", { n: waiting }));
    pill.className = "pill-sync " + cls;
    pill.textContent = txt;
    pill.title = (S.weak ? t("weak") + " · " : "") + (S.lastSync ? t("lastSync", { t: ago(S.lastSync) }) : t("neverSynced"));
  }

  // ---- sign in ------------------------------------------------------------
  function field(label, input, error) {
    return h("label", { class: "field" }, h("span", { text: label }), input, error ? h("span", { class: "err", text: error }) : null);
  }

  function screenLogin(mode = "login", state = {}) {
    formOpen = true;
    const wrap = h("main", { class: "auth" });
    const err = h("p", { class: "err", role: "alert" });
    const draw = () => wrap.replaceChildren(...content().filter(Boolean));
    const busy = (form, on) => form.querySelectorAll("button").forEach((b) => (b.disabled = on));
    const fail = (e) => (err.textContent = e instanceof ApiError ? e.message : t("networkError"));

    const langSwitch = () =>
      h("div", { class: "lang" },
        ["en", "am"].map((l) => h("button", { type: "button", class: S.lang === l ? "on" : "", onclick: async () => { pickedAtLogin = true; await changeSetting("language", l); draw(); } }, l === "en" ? "English" : "አማርኛ")));

    function content() {
      err.textContent = "";
      if (mode === "reset") {
        const phone = h("input", { name: "phone", type: "tel", inputmode: "tel", autocomplete: "tel", required: true, value: state.phone || "" });
        const form = h("form", { class: "stack", onsubmit: async (e) => {
          e.preventDefault();
          busy(form, true);
          try {
            await api("/driver/pin-reset/request", { method: "POST", auth: false, body: { phone: phone.value } });
            mode = "confirm";
            state.phone = phone.value;
            draw();
            toast(t("codeSent"));
          } catch (x) { fail(x); } finally { busy(form, false); }
        } }, field(t("phone"), phone), err, h("button", { class: "btn primary big", type: "submit", text: t("sendCode") }));
        return [h("h1", { text: t("forgotPin") }), form, h("button", { class: "btn link", type: "button", text: t("back"), onclick: () => { mode = "login"; draw(); } })];
      }
      if (mode === "confirm") {
        const code = h("input", { inputmode: "numeric", autocomplete: "one-time-code", maxlength: "6", required: true, class: "code" });
        const pin = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true });
        const pin2 = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true });
        const form = h("form", { class: "stack", onsubmit: async (e) => {
          e.preventDefault();
          if (pin.value !== pin2.value) return (err.textContent = t("pinsDiffer"));
          busy(form, true);
          try {
            const res = await api("/driver/pin-reset/confirm", { method: "POST", auth: false, body: { phone: state.phone, code: code.value, pin: pin.value, device_id: S.deviceId } });
            await signedIn(res);
          } catch (x) { fail(x); } finally { busy(form, false); }
        } }, field(t("code"), code), field(t("newPin"), pin), field(t("newPinAgain"), pin2), err, h("button", { class: "btn primary big", type: "submit", text: t("setPin") }));
        return [h("h1", { text: t("forgotPin") }), form, h("button", { class: "btn link", type: "button", text: t("back"), onclick: () => { mode = "reset"; draw(); } })];
      }
      const phone = h("input", { name: "phone", type: "tel", inputmode: "tel", autocomplete: "tel", required: true, value: state.phone || "", placeholder: "09…" });
      const pin = h("input", { name: "pin", type: "password", inputmode: "numeric", autocomplete: "current-password", maxlength: "6", required: true });
      const form = h("form", { class: "stack", onsubmit: async (e) => {
        e.preventDefault();
        busy(form, true);
        try {
          const res = await api("/driver/login", { method: "POST", auth: false, body: { phone: phone.value, pin: pin.value, device_id: S.deviceId } });
          await signedIn(res);
        } catch (x) { fail(x); } finally { busy(form, false); }
      } }, field(t("phone"), phone), field(t("pin"), pin), err, h("button", { class: "btn primary big", type: "submit", text: t("signIn") }));
      return [
        h("div", { class: "brand" }, h("span", { class: "bar-logo big", "aria-hidden": "true" }), h("strong", { text: (S.config && S.config.app_name) || "Kiflet" })),
        langSwitch(),
        h("p", { class: "muted", text: t("signInHelp") }),
        form,
        h("button", { class: "btn link", type: "button", text: t("forgotPin"), onclick: () => { mode = "reset"; state.phone = phone.value; draw(); } }),
        S.config && S.config.sms_number ? h("p", { class: "muted small", text: t("smsHelpGeneric", { num: S.config.sms_number }) }) : null,
      ];
    }
    draw();
    return wrap;
  }

  let pickedAtLogin = false; // the language was chosen on the sign-in screen
  async function signedIn(res) {
    const prev = await kvGet("driver");
    if (prev && prev.id !== res.driver.id) {
      // A different driver on this phone: nothing of the last one carries
      // over, except the language just chosen to sign in.
      await Promise.all([clear("shipments"), clear("outbox"), clear("locations")]);
      S.shipments.clear();
      S.outbox = [];
      S.messages = [];
      S.cursor = null;
      S.settings = { ...DEFAULT_SETTINGS, language: S.lang };
      S.changed = pickedAtLogin ? { language: S.lang } : {};
      await kvSet("cursor", null);
      await kvSet("messages", []);
      await kvSet("settings", S.settings);
      await kvSet("settingsChanged", S.changed);
    }
    pickedAtLogin = false;
    protectStorage();
    S.token = res.token;
    S.driver = { ...res.driver, must_change_pin: res.must_change_pin };
    await kvSet("token", S.token);
    await kvSet("driver", S.driver);
    go("#/");
    render();
    sync();
  }

  function screenPin(forced) {
    formOpen = true;
    const cur = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "current-password" });
    const pin = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "new-password" });
    const pin2 = h("input", { type: "password", inputmode: "numeric", maxlength: "6", required: true, autocomplete: "new-password" });
    const err = h("p", { class: "err", role: "alert" });
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      err.textContent = "";
      if (pin.value !== pin2.value) return (err.textContent = t("pinsDiffer"));
      form.querySelector("button").disabled = true;
      try {
        await api("/driver/pin", { method: "POST", body: { current_pin: cur.value, new_pin: pin.value } });
        S.driver.must_change_pin = false;
        await kvSet("driver", S.driver);
        toast(t("pinChanged"));
        go("#/");
        render();
      } catch (x) {
        if (x instanceof ApiError && x.status === 401) return signedOut();
        err.textContent = x instanceof ApiError ? x.message : t("networkError");
      } finally {
        form.querySelector("button").disabled = false;
      }
    } }, field(t("currentPin"), cur), field(t("newPin"), pin), field(t("newPinAgain"), pin2), err,
      h("button", { class: "btn primary big", type: "submit", text: t("setPin") }));
    return h("div", {},
      forced ? h("header", { class: "bar" }, h("span", { class: "bar-logo" }), h("div", { class: "bar-title" }, h("strong", { text: t("choosePin") }))) : header(t("changePin"), "#/menu"),
      h("main", { class: "page" }, forced ? h("p", { class: "muted", text: t("choosePinHelp") }) : null, form));
  }

  // ---- shipment list ----------------------------------------------------------
  function card(sh) {
    return h("a", { class: "ship " + (sh.priority !== "normal" ? "prio-" + sh.priority : ""), href: "#/s/" + sh.id },
      h("div", { class: "ship-top" },
        h("span", { class: "ref", text: "#" + sh.ref }),
        h("span", { class: "st st-" + sh.status, text: statusLabel(sh.status) }),
        sh.pending ? h("span", { class: "pending-dot", title: t("pendingSend") }) : null),
      h("div", { class: "ship-route" }, h("strong", { text: sh.origin_city || sh.origin_address }), " → ", h("strong", { text: sh.destination_city || sh.destination_address })),
      h("div", { class: "muted small" }, sh.customer_name, sh.eta && ACTIVE.includes(sh.status) ? " · " + t("due") + " " + fmtTime(sh.eta) : "",
        sh.delivered_at && sh.status === "delivered" ? " · " + t("deliveredAt") + " " + fmtTime(sh.delivered_at) : ""),
      sh.priority !== "normal" ? h("div", { class: "prio", text: t("priority_" + sh.priority) }) : null,
    );
  }

  function screenList() {
    const list = [...S.shipments.values()].map(effective);
    const rank = { urgent: 0, high: 1, normal: 2 };
    const active = list.filter((s) => ACTIVE.includes(s.status))
      .sort((a, b) => (rank[a.priority] ?? 2) - (rank[b.priority] ?? 2) || (a.eta || "~").localeCompare(b.eta || "~"));
    const done = list.filter((s) => !ACTIVE.includes(s.status)).sort((a, b) => (b.updated_at || "").localeCompare(a.updated_at || ""));
    const unseen = S.messages.filter((m) => !S.seen.has(m.id));
    return h("div", {},
      header(t("myShipments")),
      h("main", { class: "page" },
        S.blocked ? h("div", { class: "notice caution", role: "alert" }, h("p", { text: t("blocked") })) : null,
        storageLow() ? h("div", { class: "notice caution" }, h("p", { text: t("storageLow") }), h("a", { class: "btn link", href: "#/menu", text: t("storage") })) : null,
        S.notices.map((n) => h("div", { class: "notice" }, h("span", {}, h("strong", { text: t("rejected") + ": " }), n.text),
          h("button", { type: "button", class: "btn link", text: t("dismiss"), onclick: async () => { S.notices = S.notices.filter((x) => x !== n); await kvSet("notices", S.notices); render(); } }))),
        unseen.length ? h("section", { class: "msgs" }, h("h2", { text: t("messages") }),
          unseen.slice(0, 3).map((m) => h("div", { class: "msg" }, h("p", { text: msgText(m) }), h("p", { class: "muted small", text: (m.tracking_number ? "#" + m.tracking_number.slice(-4) + " · " : "") + m.from + " · " + fmtTime(m.at) }))),
          h("button", { class: "btn link", type: "button", text: t("dismiss"), onclick: async () => { unseen.forEach((m) => S.seen.add(m.id)); await kvSet("seen", [...S.seen]); render(); } })) : null,
        h("h2", { class: "section", text: t("active") + " (" + active.length + ")" }),
        active.length ? active.map(card) : h("div", { class: "empty" }, h("p", { text: t("none") }), h("p", { class: "muted small", text: t("noneHelp") })),
        done.length ? h("details", { class: "done" }, h("summary", { text: t("done") + " (" + done.length + ")" }), done.map(card)) : null,
        S.installPrompt ? h("div", { class: "install" }, h("p", {}, h("strong", { text: t("install") }), h("br"), h("span", { class: "muted small", text: t("installHelp") })),
          h("button", { class: "btn", type: "button", text: t("install"), onclick: async () => { S.installPrompt.prompt(); S.installPrompt = null; render(); } })) : null,
      ));
  }

  // ---- one shipment ------------------------------------------------------------
  function directions(sh) {
    if (sh.destination_lat != null) return `https://www.google.com/maps/dir/?api=1&destination=${sh.destination_lat},${sh.destination_lng}`;
    return "https://www.google.com/maps/search/?api=1&query=" + encodeURIComponent(sh.destination_address + ", " + sh.destination_city + ", Ethiopia");
  }

  async function setStatus(sh, status, reason = "", note = "") {
    const fix = fixForEvent();
    await enqueue("status", { shipment_id: sh.id, status, reason, note, lat: fix.lat, lng: fix.lng, recorded_at: new Date().toISOString() });
    if (navigator.vibrate) navigator.vibrate(40);
    toast(t("statusSaved"));
    unpark();
    updateTracking();
    go("#/s/" + sh.id);
    render();
  }

  // Straight-line distance and compass direction to the drop-off from the
  // phone's last position, so it works without signal (FR-017).
  const DIRS = ["n", "ne", "e", "se", "s", "sw", "w", "nw"];
  function bearing(a, b) {
    const rad = Math.PI / 180;
    const dLng = (b.lng - a.lng) * rad;
    const y = Math.sin(dLng) * Math.cos(b.lat * rad);
    const x = Math.cos(a.lat * rad) * Math.sin(b.lat * rad) - Math.sin(a.lat * rad) * Math.cos(b.lat * rad) * Math.cos(dLng);
    return (Math.atan2(y, x) / rad + 360) % 360;
  }
  const dropOff = (sh) => (sh.destination_lat != null ? { lat: sh.destination_lat, lng: sh.destination_lng } : null);
  function awayLine(sh) {
    const line = h("p", { class: "away" });
    const draw = () => {
      const f = S.lastFix, to = dropOff(sh);
      if (!f || !to) return line.replaceChildren();
      const dir = t("dir_" + DIRS[Math.round(bearing(f, to) / 45) % 8]);
      line.replaceChildren(h("strong", { text: t("awayFrom", { d: fmtDist(metres(f, to)), dir }) }), h("br"),
        h("span", { class: "muted small", text: t("fixFrom", { t: ago(f.time) }) }));
    };
    draw();
    if (!S.tracking) line.replaceChildren(h("span", { class: "muted small", text: t("locationOffHere") }));
    else if (!S.lastFix || Date.now() - S.lastFix.time > 5 * 60000) freshFix(15000).then(() => line.isConnected && draw());
    return line;
  }

  // Leaflet is loaded the first time a map is shown, and only with signal.
  let leafletP = null;
  function leaflet() {
    if (window.L) return Promise.resolve(window.L);
    if (!leafletP) {
      leafletP = new Promise((resolve, reject) => {
        document.head.append(h("link", { rel: "stylesheet", href: "/static/vendor/leaflet/leaflet.css" }));
        document.head.append(h("script", { src: "/static/vendor/leaflet/leaflet.js", onload: () => resolve(window.L), onerror: reject }));
      }).catch((e) => {
        leafletP = null;
        throw e;
      });
    }
    return leafletP;
  }
  function miniMap(sh) {
    const to = dropOff(sh);
    if (!to || !navigator.onLine || !S.config.tile_url) return null;
    const box = h("div", { class: "minimap" });
    const wrap = h("section", { class: "card map-card" }, box,
      h("p", { class: "map-key small muted" }, h("span", { class: "dot you" }), t("you"), h("span", { class: "dot drop" }), t("dropOff")));
    leaflet().then((L) => {
      if (!box.isConnected) return;
      const map = L.map(box, { zoomControl: false, scrollWheelZoom: false });
      L.tileLayer(S.config.tile_url, { attribution: S.config.tile_credit, maxZoom: 18 }).addTo(map);
      const dot = (ll, color) => L.circleMarker(ll, { radius: 8, weight: 3, color: "#fff", fillColor: color, fillOpacity: 1 }).addTo(map);
      dot([to.lat, to.lng], "#b42b2b");
      const f = S.lastFix;
      if (f) {
        dot([f.lat, f.lng], "#256abf");
        map.fitBounds([[to.lat, to.lng], [f.lat, f.lng]], { padding: [24, 24], maxZoom: 15 });
      } else map.setView([to.lat, to.lng], 13);
    }).catch(() => wrap.remove());
    return wrap;
  }

  function screenDetail(sh) {
    const moves = (S.config.transitions[sh.status] || []).filter((m) => m !== "delivered");
    const canDeliver = S.config.pod_from.includes(sh.status);
    const actions = [];
    if (moves.includes("in_transit")) actions.push(h("button", { class: "btn primary big", type: "button", text: sh.status === "assigned" ? t("startTrip") : t("backOnRoad"), onclick: () => setStatus(sh, "in_transit") }));
    if (moves.includes("arrived")) actions.push(h("button", { class: "btn big st-btn-arrived", type: "button", text: t("arrivedBtn"), onclick: () => setStatus(sh, "arrived") }));
    if (canDeliver) actions.push(h("a", { class: "btn big " + (sh.status === "arrived" ? "primary" : "good"), href: "#/s/" + sh.id + "/deliver", text: t("deliverBtn") }));
    if (moves.includes("delayed")) actions.push(h("a", { class: "btn", href: "#/s/" + sh.id + "/report/delayed", text: t("delayBtn") }));
    if (moves.includes("issue")) actions.push(h("a", { class: "btn warn", href: "#/s/" + sh.id + "/report/issue", text: t("problemBtn") }));
    // A new cause for the delay or problem the shipment is already in.
    if (sh.status === "delayed") actions.push(h("a", { class: "btn", href: "#/s/" + sh.id + "/report/delayed", text: t("updateDelay") }));
    if (sh.status === "issue") actions.push(h("a", { class: "btn warn", href: "#/s/" + sh.id + "/report/issue", text: t("updateProblem") }));
    const msgs = S.messages.filter((m) => m.shipment_id === sh.id);
    const smsNum = S.driver && S.driver.sms_number;
    return h("div", {},
      header("#" + sh.ref, "#/"),
      h("main", { class: "page" },
        h("div", { class: "detail-head" },
          h("span", { class: "st st-" + sh.status + " big", text: statusLabel(sh.status) + (sh.status_reason ? " · " + reasonLabel(sh.status_reason) : "") }),
          sh.priority !== "normal" ? h("span", { class: "prio", text: t("priority_" + sh.priority) }) : null,
          h("p", { class: "mono small muted", text: sh.tracking_number })),
        sh.pending ? h("p", { class: "pending", text: t("pendingSend") }) : null,
        actions.length ? h("div", { class: "actions" }, actions) : null,
        h("section", { class: "card" },
          h("div", { class: "leg" }, h("span", { class: "leg-label", text: t("from") }), h("strong", { text: sh.origin_city }), h("span", { text: sh.origin_address })),
          h("div", { class: "leg" }, h("span", { class: "leg-label", text: t("to") }), h("strong", { text: sh.destination_city }), h("span", { text: sh.destination_address }),
            ACTIVE.includes(sh.status) ? awayLine(sh) : null,
            ACTIVE.includes(sh.status) ? h("a", { class: "btn small", href: directions(sh), target: "_blank", rel: "noopener", text: "➜ " + t("navigate") }) : null),
          sh.planned_pickup && sh.status === "assigned" ? h("p", {}, h("span", { class: "muted", text: t("pickup") + ": " }), fmtTime(sh.planned_pickup)) : null,
          sh.eta && ACTIVE.includes(sh.status) ? h("p", {}, h("span", { class: "muted", text: t("due") + ": " }), fmtTime(sh.eta)) : null),
        ACTIVE.includes(sh.status) ? miniMap(sh) : null,
        h("section", { class: "card" },
          h("p", { class: "muted small", text: t("customer") }),
          h("p", {}, h("strong", { text: sh.customer_name })),
          h("a", { class: "btn big call", href: "tel:" + sh.customer_phone, text: "📞 " + t("call") + " " + sh.customer_phone })),
        sh.cargo || sh.weight_kg || sh.vehicle ? h("section", { class: "card" },
          sh.cargo ? h("p", {}, h("span", { class: "muted", text: t("cargo") + ": " }), sh.cargo, sh.weight_kg ? " · " + sh.weight_kg + " " + t("kg") : "") : null,
          sh.vehicle ? h("p", {}, h("span", { class: "muted", text: t("vehicle") + ": " }), sh.vehicle) : null) : null,
        sh.instructions ? h("section", { class: "card instructions" }, h("p", { class: "muted small", text: t("instructions") }), h("p", { class: "pre", text: sh.instructions })) : null,
        msgs.length ? h("section", { class: "card" }, h("p", { class: "muted small", text: t("messages") }),
          msgs.map((m) => h("div", { class: "msg" }, h("p", { text: msgText(m) }), h("p", { class: "muted small", text: m.from + " · " + fmtTime(m.at) })))) : null,
        ACTIVE.includes(sh.status) ? h("a", { class: "btn block", href: "#/s/" + sh.id + "/message", text: "✉ " + t("messageBtn") }) : null,
        smsNum && ACTIVE.includes(sh.status) ? h("p", { class: "muted small sms-help", text: t("smsHelp", { ref: "ET " + sh.ref, num: smsNum }) }) : null,
      ));
  }

  // ---- delay / problem -------------------------------------------------------------
  function screenReport(sh, status) {
    formOpen = true;
    const reasons = status === "delayed" ? S.config.delay_reasons : S.config.issue_reasons;
    let chosen = "";
    const err = h("p", { class: "err", role: "alert" });
    const note = h("textarea", { rows: "3", maxlength: "500" });
    const opts = h("div", { class: "choices" }, reasons.map((r) =>
      h("label", { class: "choice" }, h("input", { type: "radio", name: "reason", value: r.code, onchange: () => (chosen = r.code) }), h("span", { text: reasonLabel(r.code) }))));
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      if (!chosen) return (err.textContent = t("reason"));
      await setStatus(sh, status, chosen, note.value.trim());
    } }, h("p", { class: "muted", text: t("reason") }), opts, field(t("note"), note), err,
      h("button", { class: "btn primary big", type: "submit", text: t("save") }),
      h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
    const title = status === "delayed" ? (sh.status === "delayed" ? t("updateDelay") : t("delayBtn")) : (sh.status === "issue" ? t("updateProblem") : t("problemBtn"));
    return h("div", {}, header(title, "#/s/" + sh.id), h("main", { class: "page" }, form));
  }

  function screenMessage(sh) {
    formOpen = true;
    const text = h("textarea", { rows: "4", maxlength: "1000", required: true, placeholder: t("messagePlaceholder") });
    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      if (!text.value.trim()) return;
      await enqueue("message", { shipment_id: sh.id, text: text.value.trim(), recorded_at: new Date().toISOString() });
      toast(t("messageSaved"));
      go("#/s/" + sh.id);
      render();
    } }, text,
      h("div", { class: "chips", role: "group", "aria-label": t("quickMessages") },
        ["onway", "unloading", "callme", "roadclosed"].map((k) => h("button", { type: "button", class: "chip", text: t("quick_" + k), onclick: () => { text.value = t("quick_" + k); text.focus(); } }))),
      h("button", { class: "btn primary big", type: "submit", text: t("send") }), h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel") }));
    return h("div", {}, header(t("messageBtn"), "#/s/" + sh.id), h("main", { class: "page" }, form));
  }

  // ---- proof of delivery --------------------------------------------------------------
  // The signature box. The strokes are kept as points, so the signature is
  // drawn again unchanged when the phone turns, and saved with the draft.
  // onChange is told when a stroke ends or the box is cleared.
  function signaturePad(onChange) {
    const canvas = h("canvas", { class: "sig", "aria-label": t("signHere") });
    const ctx = canvas.getContext("2d");
    // lines: each stroke's points, with the size of the box it was drawn in.
    let lines = [], drawing = null, box = null;
    function scaleOf(l) {
      return Math.min(box.width / l.w, box.height / l.h);
    }
    function drawLine(l) {
      const k = scaleOf(l);
      const [x0, y0] = l.pts[0];
      ctx.beginPath();
      if (l.pts.length === 1) {
        ctx.arc(x0 * k, y0 * k, 1.1, 0, Math.PI * 2);
        ctx.fill();
        return;
      }
      ctx.moveTo(x0 * k, y0 * k);
      for (const [x, y] of l.pts.slice(1)) ctx.lineTo(x * k, y * k);
      ctx.stroke();
    }
    // size fits the canvas to the box it is shown in and draws the strokes
    // again.
    function size() {
      const r = canvas.getBoundingClientRect();
      if (!r.width) return;
      if (box && box.width === r.width && box.height === r.height) return;
      const ratio = Math.max(1, window.devicePixelRatio || 1);
      canvas.width = Math.round(r.width * ratio);
      canvas.height = Math.round(r.height * ratio);
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
      box = { width: r.width, height: r.height };
      redraw();
    }
    function redraw() {
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, box.width, box.height);
      ctx.lineWidth = 2.4;
      ctx.lineCap = ctx.lineJoin = "round";
      ctx.strokeStyle = ctx.fillStyle = "#111";
      lines.forEach(drawLine);
    }
    function clearPad() {
      lines = [];
      if (box) redraw();
      if (onChange) onChange();
    }
    // restore draws saved strokes back.
    function restore(saved) {
      if (!Array.isArray(saved)) return;
      lines = saved.filter((l) => l && l.w > 0 && l.h > 0 && Array.isArray(l.pts) && l.pts.length);
      if (!box) size();
      else redraw();
    }
    const pos = (e) => {
      const r = canvas.getBoundingClientRect();
      return [Math.round((e.clientX - r.left) * 10) / 10, Math.round((e.clientY - r.top) * 10) / 10];
    };
    canvas.addEventListener("pointerdown", (e) => {
      e.preventDefault();
      if (!box) return;
      canvas.setPointerCapture(e.pointerId);
      drawing = { w: box.width, h: box.height, pts: [pos(e)] };
      lines.push(drawing);
      drawLine(drawing);
    });
    canvas.addEventListener("pointermove", (e) => {
      if (!drawing) return;
      const last = drawing.pts[drawing.pts.length - 1];
      const p = pos(e);
      drawing.pts.push(p);
      ctx.beginPath();
      ctx.moveTo(last[0], last[1]);
      ctx.lineTo(p[0], p[1]);
      ctx.stroke();
    });
    const end = () => {
      if (!drawing) return;
      drawing = null;
      if (onChange) onChange();
    };
    canvas.addEventListener("pointerup", end);
    canvas.addEventListener("pointercancel", end);
    // The phone turned or the window changed size: fit the box again. The
    // listener goes once the form has.
    let resizeTimer = null;
    const onResize = () => {
      if (!canvas.isConnected) {
        window.removeEventListener("resize", onResize);
        window.removeEventListener("orientationchange", onResize);
        return;
      }
      clearTimeout(resizeTimer);
      resizeTimer = setTimeout(size, 150);
    };
    window.addEventListener("resize", onResize);
    window.addEventListener("orientationchange", onResize);
    requestAnimationFrame(() => box || size());
    return {
      canvas, clear: clearPad, restore, empty: () => lines.length === 0,
      lines: () => lines, data: () => canvas.toDataURL("image/png"),
    };
  }

  function compress(file) {
    return new Promise((resolve, reject) => {
      const img = new Image();
      const url = URL.createObjectURL(file);
      img.onload = () => {
        // Fit inside 800×600 (600×800 for portrait shots): plenty to read a
        // label or see damage, small enough to send over 2G.
        const landscape = img.width >= img.height;
        const ratio = Math.min(1, (landscape ? 800 : 600) / img.width, (landscape ? 600 : 800) / img.height);
        const cw = Math.round(img.width * ratio), ch = Math.round(img.height * ratio);
        const c = document.createElement("canvas");
        c.width = cw;
        c.height = ch;
        c.getContext("2d").drawImage(img, 0, 0, cw, ch);
        URL.revokeObjectURL(url);
        resolve(c.toDataURL("image/jpeg", 0.72));
      };
      img.onerror = () => {
        URL.revokeObjectURL(url);
        reject(new Error("unreadable image"));
      };
      img.src = url;
    });
  }

  function screenDeliver(sh) {
    formOpen = true;
    const photos = [];
    const err = h("p", { class: "err", role: "alert" });
    // The form is saved as it is filled in: a phone short of memory often
    // reloads the page when the camera opens, and the driver would otherwise
    // start again.
    const draftKey = "draft:" + sh.id;
    let draftTimer = null, restoring = true;
    const saveDraft = () => {
      if (restoring) return;
      clearTimeout(draftTimer);
      draftTimer = setTimeout(() => {
        kvSet(draftKey, {
          name: name.value, phone: phone.value, notes: notes.value, photos: photos.slice(),
          strokes: pad.lines(), why, saved: Date.now(),
        }).catch(() => {});
      }, 400);
    };
    const dropDraft = () => {
      clearTimeout(draftTimer);
      restoring = true; // nothing more to save
      return del("kv", draftKey).catch(() => {});
    };
    const name = h("input", { required: true, autocomplete: "off", maxlength: "120", oninput: saveDraft });
    const phone = h("input", { type: "tel", inputmode: "tel", maxlength: "20", oninput: saveDraft });
    const notes = h("textarea", { rows: "2", maxlength: "500", oninput: saveDraft });
    const pad = signaturePad(saveDraft);
    const thumbs = h("div", { class: "thumbs" });
    const fileInput = h("input", { type: "file", accept: "image/*", capture: "environment", class: "visually-hidden", onchange: async () => {
      const f = fileInput.files[0];
      fileInput.value = "";
      if (!f || photos.length >= MAX_PHOTOS) return;
      try {
        photos.push(await compress(f));
        drawThumbs();
        saveDraft();
      } catch {
        toast(t("photoUnreadable"));
      }
    } });
    const addBtn = h("button", { type: "button", class: "btn", onclick: () => fileInput.click() }, "📷 " + t("addPhoto"));
    function drawThumbs() {
      thumbs.replaceChildren(...photos.map((p, i) => h("figure", {}, h("img", { src: p, alt: "" }),
        h("button", { type: "button", class: "btn link small", text: t("remove"), onclick: () => { photos.splice(i, 1); drawThumbs(); saveDraft(); } }))));
      addBtn.hidden = photos.length >= MAX_PHOTOS;
    }
    const submit = h("button", { class: "btn primary big", type: "submit", text: t("confirmDelivery") });

    // Who usually signs for this customer, to save typing (AC-020.4).
    const taken = new Set();
    const names = [sh.customer_name, ...(sh.recent_recipients || [])].filter((n) => {
      const k = (n || "").trim().toLowerCase();
      return k && !taken.has(k) && taken.add(k);
    }).slice(0, 6);
    const suggestions = names.length ? h("div", { class: "chips", role: "group", "aria-label": t("suggested") },
      names.map((n) => h("button", { type: "button", class: "chip", text: n, onclick: () => { name.value = n; saveDraft(); phone.focus(); } }))) : null;
    const quickNotes = h("div", { class: "chips", role: "group", "aria-label": t("quickNotes") },
      ["security", "reception", "partial", "damaged"].map((k) => h("button", { type: "button", class: "chip", text: t("note_" + k), onclick: () => {
        const cur = notes.value.trim(), add = t("note_" + k);
        if (!cur.split("\n").includes(add)) notes.value = (cur ? cur + "\n" + add : add).slice(0, 500);
        err.textContent = "";
        saveDraft();
      } })));

    // Too far from the drop-off (AC-022.2): the driver says why, or goes
    // closer. The phone checks as the screen opens, before anyone signs,
    // and again with the position the delivery is saved with.
    let why = "";
    const far = h("div", { class: "far", hidden: true });
    const distance = (fix) => (fix && dropOff(sh) && sh.tolerance_m ? metres(fix, dropOff(sh)) : 0);
    function drawFar(d) {
      far.hidden = false;
      submit.textContent = t("deliverAnyway");
      far.replaceChildren(
        h("p", {}, h("strong", { text: t("farTitle", { d: fmtDist(d) }) })),
        h("p", { class: "small", text: t("farHelp", { tol: fmtDist(sh.tolerance_m) }) }),
        h("p", { class: "small" }, h("strong", { text: t("whyHere") })),
        h("div", { class: "choices" }, (S.config.location_reasons || []).map((r) =>
          h("label", { class: "choice" }, h("input", { type: "radio", name: "why", value: r.code, checked: why === r.code, onchange: () => ((why = r.code), (err.textContent = ""), saveDraft()) }), h("span", { text: reasonLabel(r.code) })))),
        h("a", { class: "btn", href: "#/s/" + sh.id, text: t("goBack") }));
    }
    freshFix(15000).then((fix) => {
      const d = distance(fix);
      if (far.isConnected && d > sh.tolerance_m) drawFar(d);
    });

    const form = h("form", { class: "stack", onsubmit: async (e) => {
      e.preventDefault();
      err.textContent = "";
      if (!name.value.trim()) return (err.textContent = t("needName"));
      if (pad.empty()) return (err.textContent = t("needSignature"));
      submit.disabled = true;
      submit.textContent = t("locating");
      const fix = (await freshFix(10000)) || null;
      const d = distance(fix);
      const stop = (msg) => {
        submit.disabled = false;
        submit.textContent = t("deliverAnyway");
        err.textContent = msg;
        far.scrollIntoView({ block: "start" });
      };
      if (d > sh.tolerance_m) {
        if (far.hidden) {
          drawFar(d);
          return stop(t("needWhy"));
        }
        if (!why) return stop(t("needWhy"));
        if (why === "other" && !notes.value.trim()) return stop(t("needWhyNote"));
      } else why = ""; // close enough after all
      await enqueue("pod", {
        shipment_id: sh.id, recipient_name: name.value.trim(), recipient_phone: phone.value.trim(),
        signature: pad.data(), photos: photos.slice(), notes: notes.value.trim(), location_reason: why,
        lat: fix ? fix.lat : null, lng: fix ? fix.lng : null, recorded_at: new Date().toISOString(),
      });
      await dropDraft();
      if (navigator.vibrate) navigator.vibrate([60, 40, 60]);
      toast(fix ? t("delivered") : t("noFix"));
      updateTracking();
      go("#/s/" + sh.id);
      render();
    } },
      far, field(t("recipient"), name), suggestions, field(t("recipientPhone"), phone),
      h("div", { class: "field" }, h("span", { text: t("signature") }), h("div", { class: "sig-wrap" }, pad.canvas, h("span", { class: "sig-hint", text: t("signHere") })),
        h("button", { type: "button", class: "btn link small", text: t("clear"), onclick: () => pad.clear() })),
      h("div", { class: "field" }, h("span", { text: t("photos") }), thumbs, addBtn, fileInput),
      field(t("deliveryNotes"), notes), quickNotes, err, submit,
      // Cancel throws the form away; going back keeps it for later.
      h("a", { class: "btn link", href: "#/s/" + sh.id, text: t("cancel"), onclick: () => { dropDraft(); } }));

    kvGet(draftKey).then((d) => {
      if (d && form.isConnected !== false) {
        name.value = d.name || "";
        phone.value = d.phone || "";
        notes.value = d.notes || "";
        photos.push(...(d.photos || []).slice(0, MAX_PHOTOS));
        drawThumbs();
        if (d.strokes) pad.restore(d.strokes);
        if (d.why) why = d.why;
        if (d.name || (d.strokes || []).length || photos.length) toast(t("draftRestored"));
      }
    }).catch(() => {}).finally(() => { restoring = false; });

    return h("div", {}, header(t("deliverBtn") + " · #" + sh.ref, "#/s/" + sh.id),
      h("main", { class: "page" }, h("p", { class: "muted small" }, sh.customer_name + " · " + sh.destination_address), form));
  }

  // ---- menu -------------------------------------------------------------------
  function toggle(label, checked, onchange, help) {
    return h("div", { class: "setting" },
      h("label", { class: "toggle" }, h("input", { type: "checkbox", checked, onchange: (e) => onchange(e.target.checked) }), h("span", { text: label })),
      help ? h("p", { class: "muted small", text: help }) : null);
  }

  function screenMenu() {
    const pending = S.outbox.length;
    let confirmOut = false, confirmClear = false;
    const outBox = h("div", {});
    function drawOut() {
      outBox.replaceChildren(confirmOut
        ? h("div", { class: "notice" }, h("p", { text: t("signOutPending", { n: pending }) }),
            h("button", { class: "btn warn", type: "button", text: t("signOutAnyway"), onclick: signOut }),
            h("button", { class: "btn link", type: "button", text: t("cancel"), onclick: () => { confirmOut = false; drawOut(); } }))
        : h("button", { class: "btn block", type: "button", text: t("signOut"), onclick: () => (pending ? ((confirmOut = true), drawOut()) : signOut()) }));
    }
    drawOut();

    const storage = h("section", { class: "card" });
    function drawStorage() {
      const st = S.storage;
      storage.replaceChildren(...[
        h("h2", { text: t("storage") }),
        st && st.quota ? h("p", { text: t("storageUse", { used: fmtBytes(st.usage), free: fmtBytes(Math.max(0, st.quota - st.usage)) }) }) : null,
        storageLow() ? h("p", { class: "caution-text", text: t("storageLow") }) : null,
        navigator.storage && navigator.storage.persist ? h("p", { class: "muted small", text: S.persisted ? t("storageSafe") : t("storageUnsafe") }) : null,
        confirmClear
          ? h("div", { class: "notice caution" }, h("p", { text: t("clearDataConfirm") }), h("p", { class: "small", text: t("clearDataHelp") }),
              h("button", { class: "btn warn", type: "button", text: t("clearData"), onclick: clearSaved }),
              h("button", { class: "btn link", type: "button", text: t("cancel"), onclick: () => { confirmClear = false; drawStorage(); } }))
          : h("button", { class: "btn block", type: "button", text: t("clearData"), onclick: () => {
              if (!navigator.onLine) return toast(t("clearDataOffline"));
              confirmClear = true;
              drawStorage();
            } }),
      ].filter(Boolean));
    }
    drawStorage();
    storageEstimate().then((st) => {
      S.storage = st;
      if (storage.isConnected) drawStorage();
    });

    const d = S.driver || {};
    const lowBattery = saving() && !S.settings.battery_saver;
    return h("div", {},
      header(t("menu"), "#/"),
      h("main", { class: "page" },
        h("section", { class: "card" }, h("p", {}, h("strong", { text: d.name })), h("p", { class: "muted", text: d.phone }), h("p", { class: "muted", text: d.company + (d.vehicle ? " · " + d.vehicle : "") })),
        h("section", { class: "card stack" },
          h("h2", { text: t("settings") }),
          h("div", { class: "setting" },
            h("p", { class: "muted small", text: t("language") }),
            h("div", { class: "lang" }, ["en", "am"].map((l) => h("button", { type: "button", class: S.lang === l ? "on" : "", onclick: async () => { await changeSetting("language", l); render(); } }, l === "en" ? "English" : "አማርኛ")))),
          toggle(t("batterySaver"), S.settings.battery_saver, async (on) => { await changeSetting("battery_saver", on); render(); }, t("batterySaverHelp")),
          lowBattery ? h("p", { class: "caution-text small", text: t("savingNow") }) : null,
          toggle(t("vibrate"), S.settings.vibrate, (on) => changeSetting("vibrate", on)),
          "wakeLock" in navigator ? toggle(t("keepAwake"), S.keepAwake, async (on) => { S.keepAwake = on; await kvSet("keepAwake", on); updateWakeLock(); }, t("keepAwakeHelp")) : null),
        h("section", { class: "card" },
          toggle(t("location"), S.tracking, async (on) => { S.tracking = on; await kvSet("tracking", on); updateTracking(); scheduleSync(500); }, t("locationHelp"))),
        h("section", { class: "card" },
          h("button", { class: "btn block", type: "button", text: t("syncNow"), onclick: () => sync() }),
          h("p", { class: "muted small", text: [!navigator.onLine ? t("offline") : S.weak ? t("weak") : "", S.lastSync ? t("lastSync", { t: ago(S.lastSync) }) : t("neverSynced"), pending ? t("waiting", { n: pending }) : ""].filter(Boolean).join(" · ") })),
        storage,
        h("a", { class: "btn block", href: "#/pin", text: t("changePin") }),
        outBox,
        h("section", { class: "card" },
          h("h2", { text: t("help") }),
          d.office_phone ? h("a", { class: "btn big call", href: "tel:" + d.office_phone, text: "📞 " + t("callOffice") + " " + d.office_phone }) : null,
          d.sms_number ? h("p", { class: "muted small", text: t("smsHelpGeneric", { num: d.sms_number }) }) : null),
        h("p", { class: "muted small center", text: t("version") + " " + VERSION + (S.config ? " · " + S.config.version : "") }),
      ));
  }

  // Clear saved data: the shipments and messages on the phone go, and the
  // current work downloads again. Nothing the driver recorded is lost.
  async function clearSaved() {
    if (!navigator.onLine) return toast(t("clearDataOffline"));
    await forgetShipments();
    S.notices = [];
    await kvSet("notices", []);
    toast(t("clearDataDone"));
    go("#/");
    render();
    await sync();
    S.storage = await storageEstimate();
  }

  async function signOut() {
    try {
      if (navigator.onLine) await api("/driver/logout", { method: "POST" });
    } catch { /* signing out locally is what matters */ }
    S.token = null;
    await kvSet("token", null);
    await Promise.all([clear("outbox"), clear("locations"), forgetShipments(), kvSet("notices", [])]);
    S.outbox = [];
    S.notices = [];
    S.blocked = false;
    S.pendingLocations = 0;
    stopTracking();
    go("#/");
    render();
  }

  // ---------------------------------------------------------------------------
  // Start
  // ---------------------------------------------------------------------------
  // The config the phone has is used at once, so the app opens without
  // waiting on a weak signal, and the server's is fetched behind it. Only the
  // very first start waits for the network.
  async function loadConfig() {
    const cached = await kvGet("config");
    const fresh = api("/driver/config", { auth: false, timeout: cached ? 30000 : 15000 }).then(async (cfg) => {
      await kvSet("config", cfg);
      return cfg;
    });
    if (!cached) return fresh.catch(() => null);
    fresh.then((cfg) => {
      if (JSON.stringify(cfg) === JSON.stringify(S.config)) return;
      S.config = cfg;
      rerender();
    }).catch(() => {});
    return cached;
  }
  // A first start without signal has nothing to draw: try again when the
  // signal or the driver comes back.
  async function retryConfig() {
    if (S.config) return;
    const cfg = await loadConfig();
    if (cfg && !S.config) {
      S.config = cfg;
      render();
    }
  }

  async function boot() {
    if ("serviceWorker" in navigator) {
      navigator.serviceWorker.register("/driver/sw.js", { scope: "/driver/" }).catch(() => {});
    }
    S.deviceId = await kvGet("deviceId");
    if (!S.deviceId) {
      S.deviceId = uuid();
      await kvSet("deviceId", S.deviceId);
    }
    S.settings = { ...DEFAULT_SETTINGS, ...((await kvGet("settings")) || {}) };
    S.changed = (await kvGet("settingsChanged")) || {};
    S.lang = S.settings.language || ((navigator.language || "").startsWith("am") ? "am" : "en");
    document.documentElement.lang = S.lang;
    S.keepAwake = (await kvGet("keepAwake")) === true;
    S.token = await kvGet("token");
    S.driver = await kvGet("driver");
    S.cursor = await kvGet("cursor");
    S.lastSync = await kvGet("lastSync");
    S.messages = (await kvGet("messages")) || [];
    S.seen = new Set((await kvGet("seen")) || []);
    S.notices = (await kvGet("notices")) || [];
    const tracking = await kvGet("tracking");
    S.tracking = tracking !== false;
    for (const sh of await all("shipments")) S.shipments.set(sh.id, sh);
    S.outbox = await all("outbox");
    S.pendingLocations = await tx("locations", "readonly", (s) => req2p(s.count()));
    S.config = await loadConfig();
    // A new version may send fields the stored shipments don't have yet:
    // download them again.
    const was = await kvGet("version");
    if (was !== VERSION) {
      if (was && S.token) toast(t("appUpdated"));
      S.cursor = null;
      await kvSet("cursor", null);
      await kvSet("version", VERSION);
    }
    S.storage = await storageEstimate();
    if (S.token) protectStorage();

    if (navigator.getBattery) {
      navigator.getBattery().then((b) => {
        const upd = () => {
          S.battery = Math.round(b.level * 100);
          S.charging = b.charging;
          updateTracking(); // battery saver may turn on or off
        };
        upd();
        b.addEventListener("levelchange", upd);
        b.addEventListener("chargingchange", upd);
      }).catch(() => {});
    }
    window.addEventListener("hashchange", render);
    window.addEventListener("online", () => { S.online = true; renderPill(); retryConfig(); scheduleSync(500); });
    window.addEventListener("offline", () => { S.online = false; renderPill(); });
    document.addEventListener("visibilitychange", () => {
      if (!document.hidden) {
        retryConfig();
        scheduleSync(300);
      }
      updateWakeLock(); // the browser lets go of it while the app is hidden
    });
    window.addEventListener("beforeinstallprompt", (e) => { e.preventDefault(); S.installPrompt = e; rerender(); });
    setInterval(renderPill, 30000);

    render();
    if (S.token) {
      updateTracking();
      sync();
    }
  }

  boot().catch((e) => {
    root().replaceChildren(h("div", { class: "boot" }, h("p", { text: "The app couldn't start: " + e.message })));
  });
})();
