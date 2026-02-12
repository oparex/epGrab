package main

import (
	"testing"
)

func TestCompareTimeStrings(t *testing.T) {
	tests := []struct {
		time1    string
		time2    string
		expected int // >0 if time1 > time2, 0 if equal, <0 if time1 < time2
		name     string
	}{
		// Same times
		{"12:00", "12:00", 0, "same time"},
		{"00:00", "00:00", 0, "midnight same"},
		
		// Normal comparisons (same format)
		{"10:00", "09:00", 1, "10:00 after 09:00"},
		{"09:00", "10:00", -1, "09:00 before 10:00"},
		{"12:30", "12:00", 1, "12:30 after 12:00 (minutes)"},
		{"12:00", "12:30", -1, "12:00 before 12:30 (minutes)"},
		
		// Midnight boundary
		{"23:00", "00:00", 1, "23:00 after 00:00 (not crossed midnight yet)"},
		{"00:00", "23:00", -1, "00:00 before 23:00"},
		{"23:59", "00:00", 1, "23:59 after 00:00"},
		{"00:01", "23:59", -1, "00:01 before 23:59"},
		
		// Single digit vs double digit hours (the bug scenario)
		{"9:00", "23:00", -1, "9:00 before 23:00"},
		{"23:00", "9:00", 1, "23:00 after 9:00"},
		{"8:00", "10:00", -1, "8:00 before 10:00"},
		{"10:00", "8:00", 1, "10:00 after 8:00"},
		{"1:00", "23:00", -1, "1:00 before 23:00"},
		{"23:00", "1:00", 1, "23:00 after 1:00"},
		
		// More single digit scenarios
		{"0:00", "23:00", -1, "0:00 before 23:00"},
		{"23:00", "0:00", 1, "23:00 after 0:00"},
		{"0:15", "23:45", -1, "0:15 before 23:45"},
		{"23:45", "0:15", 1, "23:45 after 0:15"},
		
		// Mixed formats
		{"09:00", "9:00", 0, "09:00 equals 9:00"},
		{"9:00", "09:00", 0, "9:00 equals 09:00"},
		{"01:00", "1:00", 0, "01:00 equals 1:00"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareTimeStrings(tt.time1, tt.time2)
			
			// Check if the sign matches
			if tt.expected > 0 && result <= 0 {
				t.Errorf("compareTimeStrings(%q, %q) = %d, expected > 0", tt.time1, tt.time2, result)
			} else if tt.expected < 0 && result >= 0 {
				t.Errorf("compareTimeStrings(%q, %q) = %d, expected < 0", tt.time1, tt.time2, result)
			} else if tt.expected == 0 && result != 0 {
				t.Errorf("compareTimeStrings(%q, %q) = %d, expected 0", tt.time1, tt.time2, result)
			}
		})
	}
}

func TestCompareTimeStringsInvalidFormat(t *testing.T) {
	// Test that invalid formats return 0 (equal) to avoid incorrect midnight detection
	tests := []struct {
		time1 string
		time2 string
		name  string
	}{
		{"12:00:00", "13:00:00", "too many colons"},
		{"invalid", "12:00", "invalid time1"},
		{"12:00", "invalid", "invalid time2"},
		{"", "", "empty strings"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Should return 0 for invalid formats to avoid incorrect midnight detection
			result := compareTimeStrings(tt.time1, tt.time2)
			if result != 0 {
				t.Errorf("compareTimeStrings(%q, %q) = %d, expected 0 for invalid format", tt.time1, tt.time2, result)
			}
		})
	}
}
