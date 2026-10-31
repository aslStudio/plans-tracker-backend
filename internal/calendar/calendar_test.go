package calendar

import (
	"testing"
	"time"
)

func TestRecurrenceAndRange(t *testing.T) {
	day := 31
	every := 2
	cases := []struct {
		name string
		rule *RepeatRule
		date string
		want bool
	}{
		{"one off", nil, "2026-02-02", false},
		{"daily", &RepeatRule{Type: "daily"}, "2026-02-02", true},
		{"weekly", &RepeatRule{Type: "weekly", DaysOfWeek: []int{0}}, "2026-02-08", true},
		{"weekly other day", &RepeatRule{Type: "weekly", DaysOfWeek: []int{0}}, "2026-02-02", false},
		{"monthly missing day", &RepeatRule{Type: "monthly", DayOfMonth: &day}, "2026-02-28", false},
		{"custom", &RepeatRule{Type: "custom", EveryNDays: &every}, "2026-02-02", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := EventInput{Date: "2026-01-31", RepeatRule: tc.rule}
			if got := Occurs(e, tc.date); got != tc.want {
				t.Fatalf("Occurs = %v, want %v", got, tc.want)
			}
		})
	}
	e := EventInput{Date: "2026-01-31", RepeatRule: &RepeatRule{Type: "daily"}, ExcludedDates: []string{"2026-02-01"}}
	start, _ := time.Parse(DateLayout, "2026-02-01")
	if VisibleInRange(e, start, start) {
		t.Fatal("excluded occurrence was visible")
	}
	end := start.AddDate(0, 0, 1)
	if !VisibleInRange(e, start, end) {
		t.Fatal("next occurrence was missing")
	}
}

func TestEventValidation(t *testing.T) {
	e := EventInput{Title: "Plan", Date: "2026-01-31", StartMinute: 1380, DurationMinutes: 60, Color: "#fff"}
	if err := e.Validate(); err != nil {
		t.Fatal(err)
	}
	e.DurationMinutes = 61
	if err := e.Validate(); err == nil {
		t.Fatal("overnight event accepted")
	}
	e.DurationMinutes = 60
	e.RepeatRule = &RepeatRule{Type: "weekly", DaysOfWeek: []int{1, 1}}
	if err := e.Validate(); err == nil {
		t.Fatal("duplicate weekdays accepted")
	}
	e.RepeatRule = &RepeatRule{Type: "daily"}
	e.ExcludedDates = []string{"2026-02-01", "2026-02-01"}
	if err := e.Validate(); err == nil {
		t.Fatal("duplicate exclusions accepted")
	}
}
