// The words the app shows, in English and Amharic. Amharic is the default
// for drivers whose phone is set to it.

import { S } from "./state.js";

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

export function t(key, vars) {
  let s = (TEXT[S.lang] && TEXT[S.lang][key]) || TEXT.en[key] || key;
  for (const [k, v] of Object.entries(vars || {})) s = s.replace("{" + k + "}", v);
  return s;
}
// A message as shown: what the office did to a shipment is said in the
// driver's language, with the office's note after it.
export function msgText(m) {
  if (m.kind === "cancelled" || m.kind === "reopened") {
    const said = t(m.kind === "cancelled" ? "msgCancelled" : "msgReopened");
    return m.note ? said + " " + m.note : said;
  }
  return m.text;
}
export function statusLabel(st) {
  if (S.lang === "am" && STATUS_AM[st]) return STATUS_AM[st];
  const found = S.config && S.config.statuses.find((x) => x.code === st);
  return found ? found.label : st;
}
export function reasonLabel(code) {
  if (S.lang === "am" && REASON_AM[code]) return REASON_AM[code];
  const list = S.config ? [...S.config.delay_reasons, ...S.config.issue_reasons, ...(S.config.location_reasons || [])] : [];
  const found = list.find((x) => x.code === code);
  return found ? found.label : code;
}
