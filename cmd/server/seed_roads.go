package main

import (
	"math"
	"math/rand/v2"

	"github.com/natnael-alemayehu/kiflet/internal/geo"
)

// The demo's GPS trails follow the main roads, so a trip on the map looks
// like one: the towns each demo route passes through, in order, between its
// two ends. Real trails need none of this; they are the phone's positions.
var demoRoads = map[demoRoute][][2]float64{
	{"Addis Ababa", "Adama"}: {
		{8.80, 38.90}, {8.75, 38.98}, {8.58, 39.12}, // Dukem, Bishoftu, Mojo
	},
	{"Addis Ababa", "Dire Dawa"}: {
		{8.80, 38.90}, {8.75, 38.98}, {8.58, 39.12}, {8.54, 39.27}, // Dukem, Bishoftu, Mojo, Adama
		{8.90, 39.92}, {8.98, 40.17}, {9.23, 40.75}, {9.08, 40.87}, // Metehara, Awash, Mieso, Asebe Teferi
		{9.22, 41.10}, {9.40, 41.55}, // Hirna, Kulubi
	},
	{"Addis Ababa", "Djibouti"}: {
		{8.80, 38.90}, {8.75, 38.98}, {8.58, 39.12}, {8.54, 39.27}, // Dukem, Bishoftu, Mojo, Adama
		{8.90, 39.92}, {8.98, 40.17}, {9.60, 40.36}, {10.17, 40.65}, // Metehara, Awash, Gewane road, Gewane
		{11.00, 40.75}, {11.42, 40.77}, {11.73, 40.98}, // Mille, Logia
		{11.69, 41.45}, {11.72, 41.83}, {11.51, 42.10}, // Dichoto, Galafi, Yoboki
		{11.40, 42.55}, {11.52, 42.85}, // Grand Bara, Arta
	},
	{"Addis Ababa", "Hawassa"}: {
		{8.80, 38.90}, {8.58, 39.12}, {8.15, 38.82}, {7.93, 38.72}, // Dukem, Mojo, Meki, Batu
		{7.35, 38.66}, {7.20, 38.60}, // Arsi Negele, Shashamane
	},
	{"Addis Ababa", "Bahir Dar"}: {
		{9.18, 38.75}, {9.31, 38.76}, {9.80, 38.73}, {10.00, 38.24}, // Sululta, Chancho, Fiche, Gohatsion
		{10.17, 38.13}, {10.33, 37.72}, {10.70, 37.27}, {10.70, 37.07}, // Dejen, Debre Markos, Finote Selam, Bure
		{10.95, 36.93}, {11.27, 36.83}, // Injibara, Dangila
	},
	{"Addis Ababa", "Gondar"}: {
		{9.18, 38.75}, {9.31, 38.76}, {9.80, 38.73}, {10.00, 38.24}, // Sululta, Chancho, Fiche, Gohatsion
		{10.17, 38.13}, {10.33, 37.72}, {10.70, 37.27}, {10.70, 37.07}, // Dejen, Debre Markos, Finote Selam, Bure
		{10.95, 36.93}, {11.27, 36.83}, {11.59, 37.39}, // Injibara, Dangila, Bahir Dar
		{11.92, 37.70}, {12.12, 37.78}, // Woreta, Addis Zemen
	},
	{"Addis Ababa", "Mekelle"}: {
		{9.32, 39.10}, {9.68, 39.53}, {9.85, 39.77}, {10.00, 39.90}, // Chacha, Debre Birhan, Debre Sina, Shewa Robit
		{10.72, 39.87}, {11.08, 39.74}, {11.13, 39.63}, {11.83, 39.60}, // Kemise, Kombolcha, Dessie, Woldia
		{12.15, 39.63}, {12.42, 39.56}, {12.78, 39.54}, // Kobo, Alamata, Maychew
	},
	{"Addis Ababa", "Jimma"}: {
		{8.92, 38.62}, {8.66, 38.21}, {8.53, 37.98}, {8.18, 37.58}, // Sebeta, Tullu Bollo, Woliso, Abelti
		{7.75, 37.23}, // Asendabo
	},
	{"Dire Dawa", "Jijiga"}: {
		{9.45, 41.95}, {9.31, 42.12}, {9.22, 42.33}, // Dengego, Harar, Babile
	},
}

// roadTrail is the demo's GPS trail for a trip from the pickup to the
// drop-off, along the route's road: n+1 positions evenly spaced over the
// first `progress` of it (1 for the whole way), wobbling a little as a
// phone's fixes do.
func roadTrail(rng *rand.Rand, r demoRoute, fromLat, fromLng, toLat, toLng, progress float64, n int) [][2]float64 {
	via, ok := demoRoads[r]
	if !ok {
		if back, found := demoRoads[demoRoute{r.to, r.from}]; found {
			via = make([][2]float64, len(back))
			for i, p := range back {
				via[len(back)-1-i] = p
			}
		}
	}
	line := append([][2]float64{{fromLat, fromLng}}, via...)
	line = append(line, [2]float64{toLat, toLng})

	legs := make([]float64, len(line)-1)
	total := 0.0
	for i := range legs {
		legs[i] = geo.DistanceMeters(line[i][0], line[i][1], line[i+1][0], line[i+1][1])
		total += legs[i]
	}
	out := make([][2]float64, 0, n+1)
	for k := 0; k <= n; k++ {
		at := total * progress * float64(k) / float64(n)
		i := 0
		for i < len(legs)-1 && at > legs[i] {
			at -= legs[i]
			i++
		}
		f := 0.0
		if legs[i] > 0 {
			f = math.Min(at/legs[i], 1)
		}
		lat := line[i][0] + (line[i+1][0]-line[i][0])*f
		lng := line[i][1] + (line[i+1][1]-line[i][1])*f
		if k > 0 && k < n {
			lat, lng = jitter(rng, lat, lng, 0.003)
		}
		out = append(out, [2]float64{lat, lng})
	}
	return out
}
