// Package geo holds the place reference data, distance maths and transit-time
// estimates the platform uses. It is pure and has no database dependency.
package geo

import (
	"sort"
	"strings"
)

// Place is a known town or city. Coordinates are the approximate town centre:
// good enough to place a shipment on a map, estimate a transit time, and say
// "last seen near Adama", but not to verify a street-level delivery.
type Place struct {
	Name   string
	Region string
	Lat    float64
	Lng    float64
	// Border marks crossing points and foreign termini on the export corridors.
	Border bool
}

// places covers the regional capitals, the towns on the main freight
// corridors (Djibouti, Moyale, Metema, Togochale) and the corridor termini.
var places = []Place{
	{"Addis Ababa", "Addis Ababa", 9.0300, 38.7400, false},
	{"Adama", "Oromia", 8.5400, 39.2700, false},
	{"Bishoftu", "Oromia", 8.7500, 38.9833, false},
	{"Dukem", "Oromia", 8.8000, 38.9000, false},
	{"Mojo", "Oromia", 8.5833, 39.1167, false},
	{"Sebeta", "Oromia", 8.9167, 38.6167, false},
	{"Sululta", "Oromia", 9.1833, 38.7500, false},
	{"Burayu", "Oromia", 9.0833, 38.6667, false},
	{"Ambo", "Oromia", 8.9833, 37.8500, false},
	{"Woliso", "Oromia", 8.5333, 37.9833, false},
	{"Fiche", "Oromia", 9.8000, 38.7333, false},
	{"Nekemte", "Oromia", 9.0833, 36.5500, false},
	{"Gimbi", "Oromia", 9.1667, 35.8333, false},
	{"Dembi Dolo", "Oromia", 8.5333, 34.8000, false},
	{"Jimma", "Oromia", 7.6667, 36.8333, false},
	{"Agaro", "Oromia", 7.8500, 36.6500, false},
	{"Metu", "Oromia", 8.3000, 35.5833, false},
	{"Assela", "Oromia", 7.9500, 39.1333, false},
	{"Batu", "Oromia", 7.9333, 38.7167, false},
	{"Shashamane", "Oromia", 7.2000, 38.6000, false},
	{"Robe", "Oromia", 7.1167, 40.0000, false},
	{"Goba", "Oromia", 7.0167, 39.9833, false},
	{"Negele Borana", "Oromia", 5.3333, 39.5833, false},
	{"Yabelo", "Oromia", 4.8833, 38.0833, false},
	{"Moyale", "Oromia", 3.5167, 39.0500, true},
	{"Awash", "Afar", 8.9833, 40.1667, false},
	{"Semera", "Afar", 11.7922, 41.0086, false},
	{"Mille", "Afar", 11.4167, 40.7667, false},
	{"Galafi", "Afar", 11.7167, 41.8333, true},
	{"Dire Dawa", "Dire Dawa", 9.6000, 41.8667, false},
	{"Dewele", "Somali", 11.0500, 42.6500, true},
	{"Harar", "Harari", 9.3100, 42.1200, false},
	{"Jijiga", "Somali", 9.3500, 42.8000, false},
	{"Togochale", "Somali", 9.6000, 43.3333, true},
	{"Degehabur", "Somali", 8.2167, 43.5667, false},
	{"Kebri Dahar", "Somali", 6.7333, 44.2667, false},
	{"Gode", "Somali", 5.9500, 43.4500, false},
	{"Debre Birhan", "Amhara", 9.6833, 39.5333, false},
	{"Debre Sina", "Amhara", 9.8500, 39.7667, false},
	{"Kemise", "Amhara", 10.7167, 39.8667, false},
	{"Kombolcha", "Amhara", 11.0833, 39.7333, false},
	{"Dessie", "Amhara", 11.1333, 39.6333, false},
	{"Woldia", "Amhara", 11.8333, 39.6000, false},
	{"Lalibela", "Amhara", 12.0333, 39.0333, false},
	{"Sekota", "Amhara", 12.6333, 39.0333, false},
	{"Debre Markos", "Amhara", 10.3333, 37.7167, false},
	{"Finote Selam", "Amhara", 10.7000, 37.2667, false},
	{"Bure", "Amhara", 10.7000, 37.0667, false},
	{"Injibara", "Amhara", 10.9500, 36.9333, false},
	{"Dangila", "Amhara", 11.2667, 36.8333, false},
	{"Bahir Dar", "Amhara", 11.5936, 37.3908, false},
	{"Debre Tabor", "Amhara", 11.8500, 38.0167, false},
	{"Gondar", "Amhara", 12.6000, 37.4667, false},
	{"Metema", "Amhara", 12.9667, 36.2000, true},
	{"Alamata", "Tigray", 12.4167, 39.5500, false},
	{"Maychew", "Tigray", 12.7833, 39.5333, false},
	{"Mekelle", "Tigray", 13.4967, 39.4753, false},
	{"Adigrat", "Tigray", 14.2667, 39.4500, false},
	{"Aksum", "Tigray", 14.1211, 38.7267, false},
	{"Shire", "Tigray", 14.1000, 38.2833, false},
	{"Humera", "Tigray", 14.3000, 36.6167, false},
	{"Hawassa", "Sidama", 7.0500, 38.4667, false},
	{"Dilla", "South Ethiopia", 6.4167, 38.3167, false},
	{"Wolaita Sodo", "South Ethiopia", 6.8500, 37.7500, false},
	{"Arba Minch", "South Ethiopia", 6.0333, 37.5500, false},
	{"Hosaena", "Central Ethiopia", 7.5500, 37.8500, false},
	{"Butajira", "Central Ethiopia", 8.1167, 38.3667, false},
	{"Welkite", "Central Ethiopia", 8.2833, 37.7833, false},
	{"Durame", "Central Ethiopia", 7.2333, 37.8833, false},
	{"Bonga", "South West Ethiopia", 7.2667, 36.2333, false},
	{"Mizan Teferi", "South West Ethiopia", 6.9833, 35.5833, false},
	{"Gambela", "Gambela", 8.2500, 34.5833, false},
	{"Asosa", "Benishangul-Gumuz", 10.0667, 34.5333, false},
	{"Djibouti", "Djibouti", 11.5886, 43.1456, true},
	{"Berbera", "Somaliland", 10.4396, 45.0143, true},
}

var byName = func() map[string]Place {
	m := make(map[string]Place, len(places))
	for _, p := range places {
		m[normalizeName(p.Name)] = p
	}
	// Common alternative names, so "Nazret" or "Debre Zeyit" still resolve.
	aliases := map[string]string{
		"addis":         "Addis Ababa",
		"finfinne":      "Addis Ababa",
		"nazret":        "Adama",
		"nazareth":      "Adama",
		"debre zeyit":   "Bishoftu",
		"modjo":         "Mojo",
		"ziway":         "Batu",
		"sodo":          "Wolaita Sodo",
		"awassa":        "Hawassa",
		"gonder":        "Gondar",
		"mekele":        "Mekelle",
		"axum":          "Aksum",
		"inda selassie": "Shire",
		"weldiya":       "Woldia",
		"jigjiga":       "Jijiga",
		"djibouti city": "Djibouti",
		"assosa":        "Asosa",
		"gambella":      "Gambela",
		"hossana":       "Hosaena",
		"kebridehar":    "Kebri Dahar",
	}
	for alias, canonical := range aliases {
		m[alias] = m[normalizeName(canonical)]
	}
	return m
}()

func normalizeName(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// Lookup finds a place by name or common alternative name, ignoring case and
// spacing.
func Lookup(name string) (Place, bool) {
	p, ok := byName[normalizeName(name)]
	return p, ok
}

// Places returns every known place, sorted by name, for pickers.
func Places() []Place {
	out := make([]Place, len(places))
	copy(out, places)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Region is an administrative region, or a corridor terminus abroad, framed
// by the known places in it: enough to centre a map on, not a border.
type Region struct {
	Name                     string
	South, West, North, East float64
}

// regionPad widens a region's frame beyond its outermost towns, about 30 km.
const regionPad = 0.3

// Regions returns the regions of the known places, sorted by name.
func Regions() []Region {
	byRegion := map[string]*Region{}
	for _, p := range places {
		r, ok := byRegion[p.Region]
		if !ok {
			r = &Region{Name: p.Region, South: p.Lat, North: p.Lat, West: p.Lng, East: p.Lng}
			byRegion[p.Region] = r
		}
		r.South = min(r.South, p.Lat)
		r.North = max(r.North, p.Lat)
		r.West = min(r.West, p.Lng)
		r.East = max(r.East, p.Lng)
	}
	out := make([]Region, 0, len(byRegion))
	for _, r := range byRegion {
		r.South, r.West, r.North, r.East = r.South-regionPad, r.West-regionPad, r.North+regionPad, r.East+regionPad
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Nearest returns the known place closest to a coordinate and its distance in
// metres.
func Nearest(lat, lng float64) (Place, float64) {
	best := places[0]
	bestDist := DistanceMeters(lat, lng, best.Lat, best.Lng)
	for _, p := range places[1:] {
		if d := DistanceMeters(lat, lng, p.Lat, p.Lng); d < bestDist {
			best, bestDist = p, d
		}
	}
	return best, bestDist
}

// InServiceArea reports whether a coordinate is plausibly within Ethiopia or
// its export corridors. It rejects obviously wrong input — swapped lat/lng, a
// pin dropped in the wrong hemisphere — without pretending to be a border map.
func InServiceArea(lat, lng float64) bool {
	return lat >= 2.5 && lat <= 15.5 && lng >= 32.5 && lng <= 48.5
}
