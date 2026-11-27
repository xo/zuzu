package expr

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// zoneNamesJSON holds the names of the time zones that the Date of
// JavaScript writes at the end of toString(), which
// test/oracle/zonenames.mjs writes from Node.js. It holds each name once,
// and for each time zone the index of the name of each offset from UTC, in
// seconds, that the zone has in 2024.
//
//go:embed zonenames.json
var zoneNamesJSON []byte

// zoneNames reads the names of the time zones once.
var zoneNames = sync.OnceValue(func() map[string]map[int]string {
	var raw struct {
		Names []string                  `json:"names"`
		Zones map[string]map[string]int `json:"zones"`
	}
	if err := json.Unmarshal(zoneNamesJSON, &raw); err != nil {
		panic("expr: the names of the time zones do not read: " + err.Error())
	}
	out := make(map[string]map[int]string, len(raw.Zones))
	for zone, offsets := range raw.Zones {
		m := make(map[int]string, len(offsets))
		for off, i := range offsets {
			n, err := strconv.Atoi(off)
			if err != nil || i < 0 || i >= len(raw.Names) {
				panic("expr: the names of the time zones do not read: the zone " + zone)
			}
			m[n] = raw.Names[i]
		}
		out[zone] = m
	}
	return out
})

// zoneName returns the name of the time zone of a date that toString of
// JavaScript writes, such as "Eastern Standard Time", from the name of its
// location. JavaScript names a zone by its name of today, whatever the
// year of the date. So a date at an offset that the zone does not have in
// 2024 takes the name of the standard time of the zone, or of its daylight
// time when the date is in daylight time. A location that Node.js does not
// know, such as time.Local or a fixed zone, takes the name GMT+hh:mm, as
// JavaScript names a zone that has no name.
func zoneName(t time.Time) string {
	_, off := t.Zone()
	names := zoneNames()[t.Location().String()]
	if n, ok := names[off]; ok {
		return n
	}
	if len(names) > 0 {
		low, high := 0, 0
		first := true
		for o := range names {
			if first || o < low {
				low = o
			}
			if first || o > high {
				high = o
			}
			first = false
		}
		if t.IsDST() {
			return names[high]
		}
		return names[low]
	}
	sign := '+'
	if off < 0 {
		sign, off = '-', -off
	}
	s := fmt.Sprintf("GMT%c%02d:%02d", sign, off/3600, off/60%60)
	if sec := off % 60; sec != 0 {
		s += fmt.Sprintf(":%02d", sec)
	}
	return s
}
