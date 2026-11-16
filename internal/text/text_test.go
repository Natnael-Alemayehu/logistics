package text

import (
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/natnael-alemayehu/kiflet/internal/model"
)

var verb = regexp.MustCompile(`%(\[\d+\])?[sd]`)

// verbs lists what a format string fills in, in a form that doesn't depend
// on the order: Amharic often says things the other way round.
func verbs(t *testing.T, key, f string) []string {
	t.Helper()
	found := verb.FindAllString(f, -1)
	if strings.Count(f, "%") != len(found) {
		t.Errorf("%s: %q has a %% that isn't %%s, %%d or a numbered one", key, f)
	}
	if len(found) > 1 {
		for _, v := range found {
			if !strings.Contains(v, "[") {
				t.Errorf("%s: %q fills in more than one value, so number each (%%[1]s)", key, f)
				break
			}
		}
	}
	for i, v := range found {
		if !strings.Contains(v, "[") {
			found[i] = "%[1]" + v[1:]
		}
	}
	slices.Sort(found)
	return found
}

// Every sentence exists in both languages and fills in the same values.
func TestCataloguesMatch(t *testing.T) {
	for key, e := range en {
		a, ok := am[key]
		if !ok {
			t.Errorf("%s has no Amharic", key)
			continue
		}
		if ev, av := verbs(t, key, e), verbs(t, key, a); !slices.Equal(ev, av) {
			t.Errorf("%s fills in %v in English and %v in Amharic", key, ev, av)
		}
		if strings.Contains(e, "<") != strings.HasSuffix(key, "_html") {
			t.Errorf("%s: markup belongs in keys ending _html, and only there", key)
		}
	}
	for key := range am {
		if _, ok := en[key]; !ok && !strings.HasPrefix(key, "reason.") {
			t.Errorf("%s has no English", key)
		}
	}
}

// A customer is told the reason for a delay or a problem, so each of those
// needs its Amharic. English comes from the model's own list.
func TestReasons(t *testing.T) {
	for _, code := range slices.Concat(model.DelayReasons, model.IssueReasons) {
		if _, ok := am["reason."+code]; !ok {
			t.Errorf("reason %s has no Amharic", code)
		}
		if got := Reason(English, code); got != model.ReasonLabel(code) {
			t.Errorf("Reason(en, %s) = %q", code, got)
		}
	}
	if got := Reason(Amharic, "flat tyre near Mojo"); got != "flat tyre near Mojo" {
		t.Errorf("prose should pass through, got %q", got)
	}
}

func TestStatus(t *testing.T) {
	for _, s := range model.AllStatuses {
		for _, l := range Languages {
			if got := Status(l.Code, s); got == "" || got == s {
				t.Errorf("Status(%s, %s) = %q", l.Code, s, got)
			}
		}
	}
	if got := Status(English, model.StatusIssue); got != "Delayed" {
		t.Errorf("a customer is told an issue is a delay, got %q", got)
	}
}

func TestT(t *testing.T) {
	if got := T(English, "place.km_from", 45, "Adama"); got != "45 km from Adama" {
		t.Errorf("got %q", got)
	}
	if got := T(Amharic, "place.km_from", 45, "Adama"); got != "ከAdama 45 ኪ.ሜ ርቀት ላይ" {
		t.Errorf("got %q", got)
	}
	if got := T("fr", "form.track"); got != "Track" {
		t.Errorf("an unknown language should read English, got %q", got)
	}
	if got := T(Amharic, "no.such.key"); got != "no.such.key" {
		t.Errorf("got %q", got)
	}
}

func TestHTMLEscapesArguments(t *testing.T) {
	got := string(HTML(English, "track.last_seen_html", "<b>near</b>", "now"))
	if got != "Last seen <strong>&lt;b&gt;near&lt;/b&gt;</strong>, now." {
		t.Errorf("got %q", got)
	}
}

func TestAccept(t *testing.T) {
	for header, want := range map[string]string{
		"am-ET,am;q=0.9,en;q=0.8": Amharic,
		"en-US,en;q=0.9":          English,
		"fr-FR, am;q=0.5":         Amharic,
		"fr":                      "",
		"":                        "",
	} {
		if got := Accept(header); got != want {
			t.Errorf("Accept(%q) = %q, want %q", header, got, want)
		}
	}
}

func TestWhen(t *testing.T) {
	at := time.Date(2026, 11, 16, 15, 4, 0, 0, time.UTC) // a Monday
	now := time.Date(2026, 11, 20, 0, 0, 0, 0, time.UTC)
	later := now.AddDate(1, 0, 0)
	for _, c := range []struct{ got, want string }{
		{When(English, at, now), "Mon 16 Nov, 15:04"},
		{When(English, at, later), "16 Nov 2026, 15:04"},
		{When(Amharic, at, now), "ሰኞ ኖቬምበር 16፣ 15:04"},
		{When(Amharic, at, later), "ኖቬምበር 16 2026፣ 15:04"},
		{ShortWhen(English, at), "Mon 16 Nov 15:04"},
		{ShortWhen(Amharic, at), "ሰኞ ኖቬምበር 16 15:04"},
		{Date(English, at), "16 Nov 2026"},
		{Date(Amharic, at), "ኖቬምበር 16 2026"},
		{Ago(English, 30*time.Second), "just now"},
		{Ago(English, 5*time.Minute), "5 min ago"},
		{Ago(English, 3*time.Hour), "3 h ago"},
		{Ago(English, 30*time.Hour), "yesterday"},
		{Ago(English, 72*time.Hour), "3 days ago"},
		{Ago(Amharic, 5*time.Minute), "ከ5 ደቂቃ በፊት"},
	} {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}
