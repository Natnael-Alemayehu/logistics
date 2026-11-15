package service

import "testing"

func TestPODLocationReasonIsKnownOrOther(t *testing.T) {
	for in, want := range map[string]string{"met_elsewhere": "met_elsewhere", " gps_inaccurate ": "gps_inaccurate", "teleported": "other", "": ""} {
		p := PODInput{RecipientName: "Almaz", LocationReason: in}
		if err := p.Check(); err != nil {
			t.Fatal(err)
		}
		if p.LocationReason != want {
			t.Errorf("reason %q became %q, want %q", in, p.LocationReason, want)
		}
	}
}
