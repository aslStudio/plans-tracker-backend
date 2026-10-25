package tasks

import (
	"testing"
	"time"
)

func mustDate(t *testing.T, value string) time.Time {
	t.Helper()
	date, err := time.Parse(time.DateOnly, value)
	if err != nil {
		t.Fatal(err)
	}
	return date
}

func TestTaskOccursOn(t *testing.T) {
	anchor := mustDate(t, "2026-01-30")
	monday := 1
	day31 := 31
	every3 := 3
	cases := []struct {
		name string
		task Task
		date string
		want bool
	}{
		{"one-off range", Task{StartDate: &anchor, EndDate: ptrDate(t, "2026-02-02")}, "2026-01-31", true},
		{"one-off before", Task{StartDate: &anchor, EndDate: ptrDate(t, "2026-02-02")}, "2026-01-29", false},
		{"backlog", Task{CreatedAt: anchor}, "2026-01-30", false},
		{"daily after original end", Task{StartDate: &anchor, EndDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatDaily}}, "2026-03-02", true},
		{"weekly selected day", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatWeekly, DaysOfWeek: []int{monday}}}, "2026-02-02", true},
		{"weekly other day", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatWeekly, DaysOfWeek: []int{monday}}}, "2026-02-03", false},
		{"monthly missing day", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatMonthly, DayOfMonth: &day31}}, "2026-02-28", false},
		{"monthly next month", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatMonthly, DayOfMonth: &day31}}, "2026-03-31", true},
		{"custom interval", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatCustom, EveryNDays: &every3}}, "2026-02-02", true},
		{"custom off interval", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatCustom, EveryNDays: &every3}}, "2026-02-01", false},
		{"before anchor", Task{StartDate: &anchor, RepeatRule: &RepeatRule{Type: RepeatDaily}}, "2026-01-29", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.task.OccursOn(mustDate(t, tc.date)); got != tc.want {
				t.Errorf("OccursOn(%s) = %v, want %v", tc.date, got, tc.want)
			}
		})
	}
}

func ptrDate(t *testing.T, value string) *time.Time { date := mustDate(t, value); return &date }

func TestValidateRepeatRuleRequiresMatchingFields(t *testing.T) {
	for _, rule := range []*RepeatRule{
		{Type: RepeatWeekly}, {Type: RepeatMonthly}, {Type: RepeatCustom},
		{Type: RepeatDaily, DaysOfWeek: []int{1}},
		{Type: RepeatWeekly, DaysOfWeek: []int{1, 1}},
	} {
		if err := validateRepeatRule(rule); err == nil {
			t.Errorf("accepted invalid rule: %#v", rule)
		}
	}
}
