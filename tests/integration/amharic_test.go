//go:build integration

package integration

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/natnael-alemayehu/kiflet/internal/service"
)

// TestCustomersInAmharic: a company that writes to its customers in Amharic
// sends them Amharic texts, and their tracking and confirmation pages open
// in Amharic until they switch (audit W1).
func TestCustomersInAmharic(t *testing.T) {
	e := newEnv(t)
	office := e.company("Amharic Freight")
	settings := url.Values{
		"name": {"Amharic Freight"}, "region_city": {"Addis Ababa"}, "max_active_per_driver": {"5"},
		"contact_phone": {office.phone}, "contact_email": {office.email},
		"sms_on_created": {"1"}, "sms_on_dispatched": {"1"}, "sms_on_delivered": {"1"}, "sms_driver_on_assign": {"1"},
	}
	settings.Set("customer_language", "fr")
	if p := office.post("/app/settings", settings); p.Status != http.StatusBadRequest {
		t.Fatalf("a language that isn't offered: status %d, want 400", p.Status)
	}
	settings.Set("customer_language", "am")
	office.mustPost("/app/settings", settings)
	if p := office.get("/app/settings"); !strings.Contains(p.Body, `value="am" lang="am" selected`) {
		t.Error("the settings page should show Amharic chosen")
	}
	// A form without the field, as an older page would send, changes nothing.
	settings.Del("customer_language")
	office.mustPost("/app/settings", settings)

	driverID, driverPhone, pin := office.addDriver("Selam Tadesse")
	customer := phoneNumber()
	shipmentID, tracking := office.bookFor("Addis Ababa", "Adama", driverID, customer)

	booked := e.received("sms", customer, tracking).Body
	if !strings.Contains(booked, "ተመዝግቧል") || !strings.Contains(booked, "/track/"+tracking) || strings.Contains(booked, "booked") {
		t.Errorf("the booking text should be in Amharic with the link: %q", booked)
	}
	if n := scalar[int32](t, `SELECT segments FROM sms_messages WHERE shipment_id = $1 AND phone = $2`, shipmentID, customer); n < 2 {
		t.Errorf("an Amharic text of %d characters was counted as %d part", len([]rune(booked)), n)
	}
	// The driver's text stays as it was: only customers are written to in
	// the company's language.
	if got := e.received("sms", driverPhone, "new shipment").Body; !strings.Contains(got, "Customer Abyssinia Traders") {
		t.Errorf("the driver's text: %q", got)
	}

	// The tracking page opens in Amharic for someone who has chosen nothing.
	visitor := e.browser()
	p := visitor.get("/track/" + tracking)
	for _, want := range []string{`<html lang="am">`, "አዳዲስ መረጃዎች", "ጭነቱ ተመዝግቧል", `href="/track/` + tracking + `?lang=en"`, "ሌላ ጭነት ይከታተሉ"} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("the Amharic tracking page lacks %q", want)
		}
	}
	for _, english := range []string{"Updates", "Shipment booked", "Track another shipment", "Preparing"} {
		if strings.Contains(p.Body, english) {
			t.Errorf("the Amharic tracking page still says %q", english)
		}
	}
	// It remembers, so the pages it links to are in Amharic too.
	if p := visitor.get("/track"); !strings.Contains(p.Body, "የክትትል ቁጥር") {
		t.Error("the form reached from an Amharic page should be in Amharic")
	}
	if p := visitor.get("/track/ET-20260101-ZZZZZZZZZZ"); p.Status != http.StatusNotFound || !strings.Contains(p.Body, "ይህን የክትትል ቁጥር ማግኘት አልቻልንም") {
		t.Errorf("an unknown number, in Amharic: status %d", p.Status)
	}
	if p := visitor.post("/track/phone", url.Values{"phone": {"12"}}); !strings.Contains(p.Body, "የኢትዮጵያ የሞባይል ቁጥር ያስገቡ") {
		t.Error("a bad phone number should be refused in Amharic")
	}
	// The switch turns it to English, and that is remembered as well.
	if p := visitor.get("/track/" + tracking + "?lang=en"); !strings.Contains(p.Body, `<html lang="en">`) || !strings.Contains(p.Body, "Shipment booked") || !strings.Contains(p.Body, "?lang=am") {
		t.Error("?lang=en should show the page in English and offer Amharic")
	}
	if p := visitor.get("/track/" + tracking); !strings.Contains(p.Body, "Updates") {
		t.Error("the choice of English should be remembered")
	}
	// A browser that asks for Amharic gets the pages that know no company
	// in Amharic.
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+"/track", nil)
	req.Header.Set("Accept-Language", "am-ET,am;q=0.9,en;q=0.8")
	if p := e.browser().do(req); !strings.Contains(p.Body, "የክትትል ቁጥር") {
		t.Error("Accept-Language: am should show the form in Amharic")
	}
	// The pages for the office are not translated, so they stay English
	// whatever was chosen.
	if p := visitor.get("/login?lang=am"); !strings.Contains(p.Body, `<html lang="en">`) || !strings.Contains(p.Body, "Sign in") {
		t.Error("the sign-in page should stay in English")
	}

	// Finding shipments by phone: the code comes in the page's language.
	amharic := e.browser()
	amharic.get("/track?lang=am")
	if p := amharic.post("/track/phone", url.Values{"phone": {customer}}); !strings.Contains(p.Body, "ባለ 6 አሃዝ ኮድ") {
		t.Fatalf("the code step should be in Amharic: status %d", p.Status)
	}
	sms := e.received("sms", customer, "ኮድዎ").Body
	if p := amharic.post("/track/phone/verify", url.Values{"phone": {customer}, "code": {"000000"}}); !strings.Contains(p.Body, "ኮዱ ተሳስቷል") {
		t.Error("a wrong code should be refused in Amharic")
	}
	amharic.mustPost("/track/phone/verify", url.Values{"phone": {customer}, "code": {codeRe.FindString(sms)}})
	if p := amharic.get("/track/mine"); !strings.Contains(p.Body, tracking) || !strings.Contains(p.Body, "ጭነቶችዎ") || !strings.Contains(p.Body, "በዝግጅት ላይ") {
		t.Error("the list of shipments should be in Amharic")
	}

	// On the road and delivered.
	d := e.signIn(driverPhone, pin)
	if r := d.status(clientID(), shipmentID, "in_transit"); !r.OK {
		t.Fatalf("depart: %+v", r)
	}
	if got := e.received("sms", customer, "በጉዞ ላይ ነው").Body; !strings.Contains(got, "Adama") {
		t.Errorf("the departure text: %q", got)
	}
	if r := d.pod(service.SyncPOD{ShipmentID: shipmentID, RecipientName: "ሩት ዓለሙ"}); !r.OK {
		t.Fatalf("proof: %+v", r)
	}
	delivered := e.received("sms", customer, "ርክክቡ ተፈጽሟል").Body
	if !strings.Contains(delivered, "ተረካቢ፦ ሩት ዓለሙ") || !strings.Contains(delivered, "/c/") {
		t.Errorf("the delivery text should name who received it and carry the link: %q", delivered)
	}

	// The confirmation page, and its answers, in Amharic.
	link := delivered[strings.Index(delivered, "/c/"):]
	customerBrowser := e.browser()
	p = customerBrowser.get(link)
	for _, want := range []string{`<html lang="am">`, "የእርስዎ ጭነት", "አዎ፣ በጥሩ ሁኔታ ተረክቤያለሁ", "ችግር አለ"} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("the Amharic confirmation page lacks %q", want)
		}
	}
	customerBrowser.mustPost(link, url.Values{"answer": {"problem"}, "feedback": {""}})
	if p := customerBrowser.get(link); !strings.Contains(p.Body, "ችግሩ ምን እንደሆነ ይንገሩን") {
		t.Error("a problem without words should be refused in Amharic")
	}
	customerBrowser.mustPost(link, url.Values{"answer": {"ok"}})
	if p := customerBrowser.get(link); !strings.Contains(p.Body, "ስላረጋገጡ እናመሰግናለን") || !strings.Contains(p.Body, "መረከብዎን አረጋግጠዋል") {
		t.Error("the thanks should be in Amharic")
	}
	if p := e.browser().get("/c/this-link-was-never-issued-to-anyone?lang=am"); p.Status != http.StatusNotFound || !strings.Contains(p.Body, "ይህ ሊንክ ጊዜው አልፎበታል") {
		t.Errorf("a dead link, in Amharic: status %d", p.Status)
	}
}

// TestCustomersInEnglishByDefault: nothing changes for a company that has
// not chosen Amharic.
func TestCustomersInEnglishByDefault(t *testing.T) {
	e := newEnv(t)
	office := e.company("English Freight")
	customer := phoneNumber()
	_, tracking := office.bookFor("Addis Ababa", "Adama", "", customer)
	if got := e.received("sms", customer, tracking).Body; !strings.Contains(got, "from Addis Ababa to Adama is booked. Track it: ") {
		t.Errorf("the booking text: %q", got)
	}
	visitor := e.browser()
	p := visitor.get("/track/" + tracking)
	for _, want := range []string{`<html lang="en">`, "Updates", "Shipment booked", `?lang=am" lang="am"`, "አማርኛ"} {
		if !strings.Contains(p.Body, want) {
			t.Errorf("the tracking page lacks %q", want)
		}
	}
	// English is nobody's choice until they make it, so it isn't
	// remembered: an Amharic company's page still opens in Amharic.
	exec(t, `UPDATE tenants SET customer_language = 'am' WHERE id = (SELECT tenant_id FROM shipments WHERE tracking_number = $1)`, tracking)
	if p := visitor.get("/track/" + tracking); !strings.Contains(p.Body, `<html lang="am">`) {
		t.Error("the page should follow the company's language while the visitor has chosen none")
	}
}
