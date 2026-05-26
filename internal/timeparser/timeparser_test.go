package timeparser

import (
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	// Fijar el tiempo actual para los tests: Lunes 25 de Mayo de 2026, 15:00:00
	location, _ := time.LoadLocation("Local")
	fixedNow := time.Date(2026, 5, 25, 15, 0, 0, 0, location)
	
	// Mockear timeNow
	timeNow = func() time.Time {
		return fixedNow
	}

	tests := []struct {
		input       string
		defaultTime string
		expectErr   bool
		expected    time.Time
		expectRrule string
		expectAmbig bool
	}{
		// 1. Relativos
		{
			input:     "en 5 minutos",
			expectErr: false,
			expected:  fixedNow.Add(5 * time.Minute),
		},
		{
			input:     "en un minuto",
			expectErr: false,
			expected:  fixedNow.Add(1 * time.Minute),
		},
		{
			input:     "en media hora",
			expectErr: false,
			expected:  fixedNow.Add(30 * time.Minute),
		},
		{
			input:     "en un cuarto de hora",
			expectErr: false,
			expected:  fixedNow.Add(15 * time.Minute),
		},
		{
			input:     "en 2 horas",
			expectErr: false,
			expected:  fixedNow.Add(2 * time.Hour),
		},
		{
			input:     "en 3 dias",
			expectErr: false,
			expected:  fixedNow.AddDate(0, 0, 3),
		},

		// 2. Absolutos e Indicadores
		{
			input:     "hoy a las 17:30",
			expectErr: false,
			expected:  time.Date(2026, 5, 25, 17, 30, 0, 0, location),
		},
		{
			input:     "mañana a las 9",
			expectErr: false,
			expected:  time.Date(2026, 5, 26, 9, 0, 0, 0, location),
		},
		{
			input:     "pasado mañana a las 8 pm",
			expectErr: false,
			expected:  time.Date(2026, 5, 27, 20, 0, 0, 0, location),
		},
		{
			input:     "el viernes a las 10 am",
			expectErr: false,
			expected:  time.Date(2026, 5, 29, 10, 0, 0, 0, location), // Viernes 29
		},

		// 3. Heurísticas AM/PM e incremento de día
		{
			// Son las 15:00. "a las 8" (am ya pasó, se asume pm -> 20:00 hoy)
			input:     "a las 8",
			expectErr: false,
			expected:  time.Date(2026, 5, 25, 20, 0, 0, 0, location),
		},
		{
			// Son las 15:00. "a las 2" (ambos pasaron hoy en am/pm, se asume mañana a las 14:00 por descarte o 2 am mañana. En nuestro caso, resolveTargetTime se mueve al día siguiente)
			input:     "a las 14:00",
			expectErr: false,
			expected:  time.Date(2026, 5, 26, 14, 0, 0, 0, location),
		},

		// 4. Ambigüedades
		{
			input:       "mañana",
			defaultTime: "08:30",
			expectErr:   false,
			expected:    time.Date(2026, 5, 26, 8, 30, 0, 0, location),
			expectAmbig: true,
		},

		// 5. Recurrencias
		{
			input:       "cada 30 minutos",
			expectErr:   false,
			expected:    fixedNow.Add(30 * time.Minute),
			expectRrule: "FREQ=MINUTELY;INTERVAL=30",
		},
		{
			input:       "todos los dias a las 10:00",
			expectErr:   false,
			expected:    time.Date(2026, 5, 26, 10, 0, 0, 0, location), // Ya pasaron las 10 am hoy (15:00), se asume mañana
			expectRrule: "FREQ=DAILY;INTERVAL=1",
		},
		{
			input:       "todos los lunes a las 9 am",
			expectErr:   false,
			expected:    time.Date(2026, 6, 1, 9, 0, 0, 0, location), // El siguiente lunes (Lunes 1 de Junio)
			expectRrule: "FREQ=WEEKLY;BYDAY=MO",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			defTime := "09:00"
			if tt.defaultTime != "" {
				defTime = tt.defaultTime
			}
			res, err := Parse(tt.input, defTime)
			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if !res.StartAt.Equal(tt.expected) {
				t.Errorf("Expected StartAt %v, got %v for input %q", tt.expected, res.StartAt, tt.input)
			}

			if res.RecurrenceRule != tt.expectRrule {
				t.Errorf("Expected RRULE %q, got %q", tt.expectRrule, res.RecurrenceRule)
			}

			if res.Ambiguous != tt.expectAmbig {
				t.Errorf("Expected Ambiguous %v, got %v", tt.expectAmbig, res.Ambiguous)
			}
		})
	}
}
