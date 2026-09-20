package config

import "testing"

func TestValidTimezone(t *testing.T) {
	valid := []string{"UTC", "Europe/Copenhagen", "America/New_York", "Pacific/Auckland", "Asia/Kolkata"}
	for _, tz := range valid {
		if !ValidTimezone(tz) {
			t.Errorf("ValidTimezone(%q) = false, want true", tz)
		}
	}

	invalid := []string{"", "  ", "Mars/Base", "not a zone", "Europe/Nowhere"}
	for _, tz := range invalid {
		if ValidTimezone(tz) {
			t.Errorf("ValidTimezone(%q) = true, want false", tz)
		}
	}
}

func TestTimezoneSuggestionsAreValid(t *testing.T) {
	suggestions := TimezoneSuggestions()
	if len(suggestions) < 50 {
		t.Fatalf("expected a substantial suggestion list, got %d", len(suggestions))
	}
	for _, tz := range suggestions {
		if !ValidTimezone(tz) {
			t.Errorf("suggestion %q is not a valid timezone", tz)
		}
	}
}
