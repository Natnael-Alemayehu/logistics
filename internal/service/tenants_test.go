package service

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Abay Logistics PLC":   "abay-logistics-plc",
		"  Selam  & Sons  ":    "selam-sons",
		"ሰላም ትራንስፖርት":          "company",
		"Tana Freight (Bahir)": "tana-freight-bahir",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
