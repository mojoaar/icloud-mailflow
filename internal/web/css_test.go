package web

import (
	"strings"
	"testing"
)

// TestInlineStyleBudget guards against inline-style regrowth while the remaining
// styles are migrated to classes. Lower the budget as they are extracted.
func TestInlineStyleBudget(t *testing.T) {
	const budget = 5

	entries, err := templatesFS.ReadDir("templates")
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}
	total := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		data, err := templatesFS.ReadFile("templates/" + e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		total += strings.Count(string(data), `style="`)
	}
	if total > budget {
		t.Errorf("inline style attributes = %d, budget %d; prefer CSS classes", total, budget)
	}
}
