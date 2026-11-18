package service

import "testing"

func TestParsePieces(t *testing.T) {
	for in, want := range map[string]struct {
		n    int
		unit string
	}{
		"":             {0, ""},
		"  ":           {0, ""},
		"152 bales":    {152, "bales"},
		"152":          {152, ""},
		"1,200 bags":   {1200, "bags"},
		" 40  pallets": {40, "pallets"},
		"12ctn":        {12, "ctn"},
	} {
		n, unit, err := ParsePieces(in)
		if err != nil || n != want.n || unit != want.unit {
			t.Errorf("ParsePieces(%q) = %d, %q, %v; want %d, %q", in, n, unit, err, want.n, want.unit)
		}
	}
	for _, in := range []string{"bales", "0 bags", "-3 bags", ",", "x12"} {
		if _, _, err := ParsePieces(in); err == nil {
			t.Errorf("ParsePieces(%q) should fail", in)
		}
	}
	n, unit := int32(152), "bales"
	if got := FormatPieces(&n, &unit); got != "152 bales" {
		t.Errorf("FormatPieces = %q", got)
	}
	if got := FormatPieces(&n, nil); got != "152" {
		t.Errorf("FormatPieces without a unit = %q", got)
	}
	if got := FormatPieces(nil, &unit); got != "" {
		t.Errorf("FormatPieces without a count = %q", got)
	}
}

// The import template's example row names only columns that exist, so a
// renamed column can't leave it silently empty.
func TestImportExampleColumns(t *testing.T) {
	names := map[string]bool{}
	for _, c := range ImportColumns {
		names[c.Name] = true
	}
	for name := range importExample {
		if !names[name] {
			t.Errorf("the import example fills %q, which isn't a column", name)
		}
	}
}
