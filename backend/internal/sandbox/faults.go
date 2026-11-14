package sandbox

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
)

// Faults are what the sandbox can be told to get wrong, to see what Menged
// makes of it. They are set on the controls page, or with PUT or PATCH
// /api/faults, and last until they are turned off or the sandbox stops.
type Faults struct {
	// The SMS gateway answers 503 to every text.
	SMSDown bool `json:"sms_down"`
	// It answers after Timing.Slow, longer than Menged waits.
	SMSSlow bool `json:"sms_slow"`
	// It refuses every token, new ones too, with 401.
	SMSRefuseTokens bool `json:"sms_refuse_tokens"`
	// It keeps delivery reports back.
	DropReports bool `json:"drop_reports"`
	// It refuses texts to Barred numbers for good, with 400; it takes texts
	// to Unreachable ones but reports them undelivered.
	Barred      []string `json:"barred"`
	Unreachable []string `json:"unreachable"`

	// telebirr answers preOrder 500.
	PreOrderFails bool `json:"preorder_fails"`
	// It keeps notifications back, sends each twice, sends them Timing.Delay
	// late, or signs them with a key that isn't telebirr's.
	DropNotifications      bool `json:"drop_notifications"`
	DuplicateNotifications bool `json:"duplicate_notifications"`
	DelayNotifications     bool `json:"delay_notifications"`
	WrongKey               bool `json:"wrong_key"`

	// CBE Birr keeps confirmations back, or sends each twice.
	DropConfirmations      bool `json:"drop_confirmations"`
	DuplicateConfirmations bool `json:"duplicate_confirmations"`
}

func (f Faults) copy() Faults {
	f.Barred, f.Unreachable = slices.Clone(f.Barred), slices.Clone(f.Unreachable)
	return f
}

// on counts the faults set.
func (f Faults) on() int {
	n := 0
	for _, set := range []bool{
		f.SMSDown, f.SMSSlow, f.SMSRefuseTokens, f.DropReports, len(f.Barred) > 0, len(f.Unreachable) > 0,
		f.PreOrderFails, f.DropNotifications, f.DuplicateNotifications, f.DelayNotifications, f.WrongKey,
		f.DropConfirmations, f.DuplicateConfirmations,
	} {
		if set {
			n++
		}
	}
	return n
}

func (f Faults) isBarred(number string) bool      { return slices.Contains(f.Barred, number) }
func (f Faults) isUnreachable(number string) bool { return slices.Contains(f.Unreachable, number) }

// clean puts the numbers in their local form, refusing what isn't one.
func (f *Faults) clean() error {
	for _, list := range []*[]string{&f.Barred, &f.Unreachable} {
		var out, bad []string
		for _, n := range *list {
			if local, ok := localNumber(n); ok {
				if !slices.Contains(out, local) {
					out = append(out, local)
				}
			} else if strings.TrimSpace(n) != "" {
				bad = append(bad, n)
			}
		}
		if len(bad) > 0 {
			return fmt.Errorf("not Ethiopian mobile numbers: %s", strings.Join(bad, ", "))
		}
		*list = out
	}
	return nil
}

// setFaults replaces the faults.
func (s *Sandbox) setFaults(f Faults) error {
	if err := f.clean(); err != nil {
		return err
	}
	s.mu.Lock()
	s.faults = f
	s.mu.Unlock()
	return nil
}

// faultsFromForm reads the controls page's form.
func faultsFromForm(form url.Values) Faults {
	on := func(name string) bool { return form.Get(name) != "" }
	numbers := func(name string) []string {
		return strings.FieldsFunc(form.Get(name), func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' || r == '\r' || r == '\t' })
	}
	return Faults{
		SMSDown: on("sms_down"), SMSSlow: on("sms_slow"), SMSRefuseTokens: on("sms_refuse_tokens"), DropReports: on("drop_reports"),
		Barred: numbers("barred"), Unreachable: numbers("unreachable"),
		PreOrderFails: on("preorder_fails"), DropNotifications: on("drop_notifications"),
		DuplicateNotifications: on("duplicate_notifications"), DelayNotifications: on("delay_notifications"), WrongKey: on("wrong_key"),
		DropConfirmations: on("drop_confirmations"), DuplicateConfirmations: on("duplicate_confirmations"),
	}
}
