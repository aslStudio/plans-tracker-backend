package habits

import (
	"errors"
	"testing"
)

func TestRepeatRuleValidation(t *testing.T) {
	one, zero, thirtyTwo := 1, 0, 32
	cases := []struct {
		name  string
		rule  RepeatRule
		valid bool
	}{
		{"daily", RepeatRule{Type: RepeatDaily}, true},
		{"weekly", RepeatRule{Type: RepeatWeekly, DaysOfWeek: []int{0, 3}}, true},
		{"monthly", RepeatRule{Type: RepeatMonthly, DayOfMonth: &one}, true},
		{"custom", RepeatRule{Type: RepeatCustom, EveryNDays: &one}, true},
		{"weekly empty", RepeatRule{Type: RepeatWeekly}, false},
		{"weekly duplicate", RepeatRule{Type: RepeatWeekly, DaysOfWeek: []int{1, 1}}, false},
		{"monthly zero", RepeatRule{Type: RepeatMonthly, DayOfMonth: &zero}, false},
		{"monthly 32", RepeatRule{Type: RepeatMonthly, DayOfMonth: &thirtyTwo}, false},
		{"custom zero", RepeatRule{Type: RepeatCustom, EveryNDays: &zero}, false},
		{"unexpected field", RepeatRule{Type: RepeatDaily, DayOfMonth: &one}, false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			err := test.rule.Validate()
			if (err == nil) != test.valid {
				t.Fatalf("validation error = %v, valid = %v", err, test.valid)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
