package scheduler

import (
	"fmt"
	"strings"
	"time"
)

// ParseRRULEAndGetNext calcula la siguiente fecha/hora a partir del tiempo actual (now) y del time.Time inicial del recordatorio (startAt),
// basándose en la cadena RRULE dada.
func ParseRRULEAndGetNext(rrule string, startAt, now time.Time) (time.Time, error) {
	if rrule == "" {
		return time.Time{}, fmt.Errorf("regla RRULE vacía")
	}

	parts := strings.Split(rrule, ";")
	params := make(map[string]string)
	for _, p := range parts {
		kv := strings.Split(p, "=")
		if len(kv) == 2 {
			params[strings.ToUpper(kv[0])] = strings.ToUpper(kv[1])
		}
	}

	freq := params["FREQ"]
	interval := 1
	if intervalStr, ok := params["INTERVAL"]; ok {
		var val int
		if _, err := fmt.Sscanf(intervalStr, "%d", &val); err == nil && val > 0 {
			interval = val
		}
	}

	target := startAt
	if target.After(now) {
		return target, nil
	}

	switch freq {
	case "MINUTELY":
		diff := now.Sub(target)
		minutes := int(diff.Minutes())
		if minutes < 0 {
			minutes = 0
		}
		ticks := (minutes / interval) + 1
		target = target.Add(time.Duration(ticks*interval) * time.Minute)

	case "HOURLY":
		diff := now.Sub(target)
		hours := int(diff.Hours())
		if hours < 0 {
			hours = 0
		}
		ticks := (hours / interval) + 1
		target = target.Add(time.Duration(ticks*interval) * time.Hour)

	case "DAILY":
		days := int(now.Sub(target).Hours() / 24)
		if days < 0 {
			days = 0
		}
		ticks := (days / interval) + 1
		target = target.AddDate(0, 0, ticks*interval)
		for !target.After(now) {
			target = target.AddDate(0, 0, interval)
		}

	case "WEEKLY":
		byday, hasByday := params["BYDAY"]
		if !hasByday {
			days := int(now.Sub(target).Hours() / 24)
			if days < 0 {
				days = 0
			}
			ticks := (days / (7 * interval)) + 1
			target = target.AddDate(0, 0, ticks*7*interval)
			for !target.After(now) {
				target = target.AddDate(0, 0, 7*interval)
			}
			return target, nil
		}

		bydays := strings.Split(byday, ",")
		dayMap := map[string]time.Weekday{
			"MO": time.Monday,
			"TU": time.Tuesday,
			"WE": time.Wednesday,
			"TH": time.Thursday,
			"FR": time.Friday,
			"SA": time.Saturday,
			"SU": time.Sunday,
		}

		var targetDays []time.Weekday
		for _, bd := range bydays {
			if wd, ok := dayMap[bd]; ok {
				targetDays = append(targetDays, wd)
			}
		}

		if len(targetDays) == 0 {
			return time.Time{}, fmt.Errorf("regla BYDAY no válida en RRULE: %s", rrule)
		}

		curr := now.Add(1 * time.Second)
		found := false
		for i := 0; i < 365; i++ {
			for _, wd := range targetDays {
				if curr.Weekday() == wd {
					target = time.Date(curr.Year(), curr.Month(), curr.Day(), startAt.Hour(), startAt.Minute(), startAt.Second(), 0, startAt.Location())
					if target.After(now) {
						found = true
						break
					}
				}
			}
			if found {
				break
			}
			curr = curr.AddDate(0, 0, 1)
		}
		if !found {
			return time.Time{}, fmt.Errorf("no se encontró siguiente fecha para RRULE: %s", rrule)
		}

	default:
		return time.Time{}, fmt.Errorf("frecuencia RRULE no soportada: %s", freq)
	}

	return target, nil
}
