package timeparser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Parse intenta resolver una frase de tiempo en español y retorna el ParsedTime correspondiente.
// defaultTimeStr especifica la hora por defecto (formato "HH:MM") si el usuario no proporciona la hora.
func Parse(input string, defaultTimeStr string) (ParsedTime, error) {
	now := timeNow()
	normalized := NormalizeSpanish(input)

	// Inicializar valores por defecto
	res := ParsedTime{
		StartAt:    now,
		Confidence: 0.0,
		Ambiguous:  false,
	}

	// Parsing de la hora por defecto
	defHour := 9
	defMin := 0
	if parts := strings.Split(defaultTimeStr, ":"); len(parts) == 2 {
		if h, err := strconv.Atoi(parts[0]); err == nil {
			defHour = h
		}
		if m, err := strconv.Atoi(parts[1]); err == nil {
			defMin = m
		}
	}

	// 1. EVALUAR RECURRENCIAS PRIMERO (Generan RRULE)
	
	// "cada (N) minutos" o "cada minuto"
	reRecurMin := regexp.MustCompile(`cada\s+(?:(\d+)\s+)?minutos?`)
	if match := reRecurMin.FindStringSubmatch(normalized); len(match) > 0 {
		interval := 1
		if match[1] != "" {
			if v, err := strconv.Atoi(match[1]); err == nil {
				interval = v
			}
		}
		res.RecurrenceRule = fmt.Sprintf("FREQ=MINUTELY;INTERVAL=%d", interval)
		res.StartAt = now.Add(time.Duration(interval) * time.Minute)
		res.Confidence = 0.90
		return res, nil
	}

	// "cada (N) horas" o "cada hora"
	reRecurHour := regexp.MustCompile(`cada\s+(?:(\d+)\s+)?horas?`)
	if match := reRecurHour.FindStringSubmatch(normalized); len(match) > 0 {
		interval := 1
		if match[1] != "" {
			if v, err := strconv.Atoi(match[1]); err == nil {
				interval = v
			}
		}
		res.RecurrenceRule = fmt.Sprintf("FREQ=HOURLY;INTERVAL=%d", interval)
		res.StartAt = now.Add(time.Duration(interval) * time.Hour)
		res.Confidence = 0.90
		return res, nil
	}

	// "todos los dias a las N" / "cada dia a las N"
	reRecurDaily := regexp.MustCompile(`(?:todos los dias|cada dia|diariamente)(?:\s+a las\s+(\d+)(?::(\d+))?)?`)
	if match := reRecurDaily.FindStringSubmatch(normalized); len(match) > 0 && match[0] != "" {
		res.RecurrenceRule = "FREQ=DAILY;INTERVAL=1"
		res.Confidence = 0.90
		
		hour := defHour
		min := defMin
		if match[1] != "" {
			if h, err := strconv.Atoi(match[1]); err == nil {
				hour = h
			}
			if match[2] != "" {
				if m, err := strconv.Atoi(match[2]); err == nil {
					min = m
				}
			}
		} else {
			res.Ambiguous = true
			res.Reason = "Falta la hora exacta para la recurrencia diaria"
		}
		
		res.StartAt = resolveTargetTime(now, hour, min, true) // Cambiado a true para evitar programar hoy en el pasado
		return res, nil
	}

	// "todos los (dia de la semana) a las N"
	reRecurWeekly := regexp.MustCompile(`todos los\s+(lunes|martes|miercoles|miercoles|jueves|viernes|sabado|sábado|domingo)(?:\s+a las\s+(\d+)(?::(\d+))?)?`)
	if match := reRecurWeekly.FindStringSubmatch(normalized); len(match) > 0 {
		dayStr := match[1]
		weekday, ok := SpanishDays[dayStr]
		if ok {
			var rruleDay string
			switch weekday {
			case time.Monday:
				rruleDay = "MO"
			case time.Tuesday:
				rruleDay = "TU"
			case time.Wednesday:
				rruleDay = "WE"
			case time.Thursday:
				rruleDay = "TH"
			case time.Friday:
				rruleDay = "FR"
			case time.Saturday:
				rruleDay = "SA"
			case time.Sunday:
				rruleDay = "SU"
			}
			res.RecurrenceRule = fmt.Sprintf("FREQ=WEEKLY;BYDAY=%s", rruleDay)
			res.Confidence = 0.90

			hour := defHour
			min := defMin
			if match[2] != "" {
				if h, err := strconv.Atoi(match[2]); err == nil {
					hour = h
				}
				if match[3] != "" {
					if m, err := strconv.Atoi(match[3]); err == nil {
						min = m
					}
				}
			} else {
				res.Ambiguous = true
				res.Reason = fmt.Sprintf("Falta la hora para la recurrencia semanal del %s", dayStr)
			}

			// Encontrar el siguiente día de la semana correspondiente
			targetDate := now
			daysDiff := (int(weekday) - int(now.Weekday()) + 7) % 7
			if daysDiff == 0 {
				// Si es hoy, verificar si la hora ya pasó
				testTime := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
				if testTime.Before(now) {
					daysDiff = 7
				}
			}
			targetDate = targetDate.AddDate(0, 0, daysDiff)
			res.StartAt = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), hour, min, 0, 0, now.Location())
			return res, nil
		}
	}

	// 2. PATRONES RELATIVOS DE INTERVALO INMEDIATO

	// "en un minuto" o "en N minutos"
	reMin := regexp.MustCompile(`en\s+(?:(\d+|un|una|1)\s+)?minutos?`)
	if match := reMin.FindStringSubmatch(normalized); len(match) > 0 {
		interval := 1
		if match[1] != "" {
			if match[1] == "un" || match[1] == "una" {
				interval = 1
			} else if v, err := strconv.Atoi(match[1]); err == nil {
				interval = v
			}
		}
		res.StartAt = now.Add(time.Duration(interval) * time.Minute)
		res.Confidence = 0.95
		return res, nil
	}

	// "en media hora" / "en 1/2 hora" / "en media"
	if strings.Contains(normalized, "en media hora") || strings.Contains(normalized, "en 1/2 hora") {
		res.StartAt = now.Add(30 * time.Minute)
		res.Confidence = 0.95
		return res, nil
	}

	// "en un cuarto de hora" / "en 15 minutos"
	if strings.Contains(normalized, "en un cuarto de hora") {
		res.StartAt = now.Add(15 * time.Minute)
		res.Confidence = 0.95
		return res, nil
	}

	// "en una hora" o "en N horas"
	reHour := regexp.MustCompile(`en\s+(?:(\d+|un|una|1)\s+)?horas?`)
	if match := reHour.FindStringSubmatch(normalized); len(match) > 0 {
		interval := 1
		if match[1] != "" {
			if match[1] == "un" || match[1] == "una" {
				interval = 1
			} else if v, err := strconv.Atoi(match[1]); err == nil {
				interval = v
			}
		}
		res.StartAt = now.Add(time.Duration(interval) * time.Hour)
		res.Confidence = 0.95
		return res, nil
	}

	// "en un dia" o "en N dias"
	reDay := regexp.MustCompile(`en\s+(?:(\d+|un|una|1)\s+)?dias?`)
	if match := reDay.FindStringSubmatch(normalized); len(match) > 0 {
		interval := 1
		if match[1] != "" {
			if match[1] == "un" || match[1] == "una" {
				interval = 1
			} else if v, err := strconv.Atoi(match[1]); err == nil {
				interval = v
			}
		}
		res.StartAt = now.AddDate(0, 0, interval)
		res.Confidence = 0.95
		return res, nil
	}

	// 3. PATRONES ABSOLUTOS / FECHA RELATIVA + HORA

	// Extraer hora y am/pm de la frase
	// "a las X" / "a la X" / "de las X"
	reTimeSpec := regexp.MustCompile(`(?:a la|a las|de la|de las)\s+(\d+)(?::(\d+))?\s*(pm|am|de la tarde|de la noche|de la manana)?`)
	timeMatch := reTimeSpec.FindStringSubmatch(normalized)

	hour := defHour
	min := defMin
	hasTime := false
	pmFlag := false
	amFlag := false

	if len(timeMatch) > 0 {
		hasTime = true
		if h, err := strconv.Atoi(timeMatch[1]); err == nil {
			hour = h
		}
		if timeMatch[2] != "" {
			if m, err := strconv.Atoi(timeMatch[2]); err == nil {
				min = m
			}
		} else {
			min = 0 // Si dice "a las 8" se asume "8:00"
		}
		// Evaluar indicador de tarde/mañana
		indicator := timeMatch[3]
		if indicator == "pm" || indicator == "de la tarde" || indicator == "de la noche" {
			pmFlag = true
		} else if indicator == "am" || indicator == "de la manana" {
			amFlag = true
		}
	}

	// Resolver la hora (12h a 24h)
	if hasTime {
		if hour <= 12 {
			if pmFlag && hour < 12 {
				hour += 12
			} else if amFlag && hour == 12 {
				hour = 0
			} else if !pmFlag && !amFlag {
				// Detección heurística de 12h si no hay indicador y el objetivo es para hoy
				isTodayTarget := strings.Contains(normalized, "hoy") ||
					(!strings.Contains(normalized, "manana") &&
						!strings.Contains(normalized, "pasado manana") &&
						!strings.Contains(normalized, "lunes") &&
						!strings.Contains(normalized, "martes") &&
						!strings.Contains(normalized, "miercoles") &&
						!strings.Contains(normalized, "jueves") &&
						!strings.Contains(normalized, "viernes") &&
						!strings.Contains(normalized, "sabado") &&
						!strings.Contains(normalized, "domingo"))

				if isTodayTarget {
					testTimeAM := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
					if testTimeAM.Before(now) {
						// Probar en PM (ej. las 8 pm -> 20:00)
						hourPM := hour + 12
						testTimePM := time.Date(now.Year(), now.Month(), now.Day(), hourPM, min, 0, 0, now.Location())
						if !testTimePM.Before(now) {
							hour = hourPM
						}
					}
				}
			}
		}
	}

	// 3.1. "pasado mañana (a las X)"
	if strings.Contains(normalized, "pasado manana") {
		res.Confidence = 0.90
		if !hasTime {
			res.Ambiguous = true
			res.Reason = "Falta la hora exacta para pasado mañana, se usará la hora por defecto"
		}
		res.StartAt = resolveTargetTime(now.AddDate(0, 0, 2), hour, min, false)
		return res, nil
	}

	// 3.2. "mañana (a las X)"
	if strings.Contains(normalized, "manana") {
		res.Confidence = 0.90
		if !hasTime {
			res.Ambiguous = true
			res.Reason = "Falta la hora exacta para mañana, se usará la hora por defecto"
		}
		res.StartAt = resolveTargetTime(now.AddDate(0, 0, 1), hour, min, false)
		return res, nil
	}

	// 3.3. "hoy (a las X)"
	if strings.Contains(normalized, "hoy") {
		res.Confidence = 0.90
		if !hasTime {
			res.Ambiguous = true
			res.Reason = "Falta la hora exacta para hoy, se usará la hora por defecto"
		}
		res.StartAt = resolveTargetTime(now, hour, min, true)
		return res, nil
	}

	// 3.4. "el (día de la semana) a las X"
	reWeekday := regexp.MustCompile(`el\s+(lunes|martes|miercoles|jueves|viernes|sabado|domingo)`)
	if match := reWeekday.FindStringSubmatch(normalized); len(match) > 0 {
		dayStr := match[1]
		weekday, ok := SpanishDays[dayStr]
		if ok {
			res.Confidence = 0.90
			if !hasTime {
				res.Ambiguous = true
				res.Reason = fmt.Sprintf("Falta la hora para el %s, se usará la hora por defecto", dayStr)
			}
			
			daysDiff := (int(weekday) - int(now.Weekday()) + 7) % 7
			if daysDiff == 0 && hasTime {
				// Si es hoy, verificar si la hora ya pasó
				testTime := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())
				if testTime.Before(now) {
					daysDiff = 7
				}
			} else if daysDiff == 0 && !hasTime {
				daysDiff = 7 // Si no hay hora, se asume el próximo de la semana siguiente
			}

			targetDate := now.AddDate(0, 0, daysDiff)
			res.StartAt = time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), hour, min, 0, 0, now.Location())
			return res, nil
		}
	}

	// 3.5. "a las X" (sin fecha explícita, se asume hoy/mañana)
	if hasTime {
		res.Confidence = 0.85
		res.StartAt = resolveTargetTime(now, hour, min, true)
		return res, nil
	}

	// Si no coincide con ningún patrón temporal conocido
	return res, fmt.Errorf("no se pudo identificar ninguna expresión temporal válida en la frase %q", input)
}

// resolveTargetTime toma un día base y le aplica la hora/minuto, decidiendo si es para hoy o mañana si ya pasó.
func resolveTargetTime(base time.Time, hour, min int, autoTomorrow bool) time.Time {
	target := time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, base.Location())
	
	// Si autoTomorrow es true y la hora ya pasó hoy, se programa para mañana
	if autoTomorrow && target.Before(timeNow()) {
		target = target.AddDate(0, 0, 1)
	}
	return target
}
