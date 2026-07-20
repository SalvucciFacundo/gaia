package cron

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Schedule represents a parsed cron expression.
// Supports standard 5-field cron: minute hour day month weekday.
type Schedule struct {
	minute  []int
	hour    []int
	day     []int
	month   []int
	weekday []int
}

// ParseSchedule parses a cron expression and returns a Schedule.
// Format: "minute hour day month weekday" (5 fields).
// Each field can be: *, N, N-M, */N, N,M,O.
func ParseSchedule(expr string) (*Schedule, error) {
	fields := strings.Fields(expr)
	if len(fields) != 5 {
		return nil, fmt.Errorf("invalid cron expression: expected 5 fields, got %d", len(fields))
	}

	s := &Schedule{}
	var err error

	s.minute, err = parseCronField(fields[0], 0, 59)
	if err != nil {
		return nil, fmt.Errorf("minute field: %w", err)
	}
	s.hour, err = parseCronField(fields[1], 0, 23)
	if err != nil {
		return nil, fmt.Errorf("hour field: %w", err)
	}
	s.day, err = parseCronField(fields[2], 1, 31)
	if err != nil {
		return nil, fmt.Errorf("day field: %w", err)
	}
	s.month, err = parseCronField(fields[3], 1, 12)
	if err != nil {
		return nil, fmt.Errorf("month field: %w", err)
	}
	s.weekday, err = parseCronField(fields[4], 0, 6)
	if err != nil {
		return nil, fmt.Errorf("weekday field: %w", err)
	}

	return s, nil
}

// Next returns the next time after 't' that matches the schedule.
func (s *Schedule) Next(t time.Time) time.Time {
	// Start at the next minute
	t = t.Truncate(time.Minute).Add(time.Minute)

	// Search up to 4 years ahead (leap year safety)
	for i := 0; i < 525600*4; i++ {
		if s.matches(t) {
			return t
		}
		t = t.Add(time.Minute)
	}

	return time.Time{} // should never reach here
}

// matches checks if time t matches the schedule.
func (s *Schedule) matches(t time.Time) bool {
	if !contains(s.minute, t.Minute()) {
		return false
	}
	if !contains(s.hour, t.Hour()) {
		return false
	}
	if !contains(s.day, t.Day()) {
		return false
	}
	if !contains(s.month, int(t.Month())) {
		return false
	}
	if !contains(s.weekday, int(t.Weekday())) {
		return false
	}
	return true
}

// parseCronField parses a single cron field into a slice of values.
// Supports: *, N, N-M, */N, N,M,O.
func parseCronField(field string, min, max int) ([]int, error) {
	if field == "*" {
		values := make([]int, max-min+1)
		for i := 0; i < len(values); i++ {
			values[i] = min + i
		}
		return values, nil
	}

	// Step: */N
	if strings.HasPrefix(field, "*/") {
		step, err := strconv.Atoi(field[2:])
		if err != nil {
			return nil, fmt.Errorf("invalid step: %s", field[2:])
		}
		var values []int
		for i := min; i <= max; i += step {
			values = append(values, i)
		}
		return values, nil
	}

	// Comma-separated list
	var values []int
	parts := strings.Split(field, ",")
	for _, part := range parts {
		// Range: N-M
		if strings.Contains(part, "-") {
			rangeParts := strings.SplitN(part, "-", 2)
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid range start: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid range end: %s", rangeParts[1])
			}
			for i := start; i <= end; i++ {
				if i >= min && i <= max {
					values = append(values, i)
				}
			}
			continue
		}

		// Single value
		v, err := strconv.Atoi(part)
		if err != nil {
			return nil, fmt.Errorf("invalid value: %s", part)
		}
		if v < min || v > max {
			return nil, fmt.Errorf("value %d out of range [%d, %d]", v, min, max)
		}
		values = append(values, v)
	}

	return values, nil
}

// contains checks if a slice contains a value.
func contains(slice []int, val int) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}
