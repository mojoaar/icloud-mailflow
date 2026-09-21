package web

import (
	"regexp"
	"strings"
	"testing"
)

// TestTemplatesLabelsAssociated ensures every <label> either points at a control
// via for= or wraps one inline.
func TestTemplatesLabelsAssociated(t *testing.T) {
	entries, err := templatesFS.ReadDir("templates")
	if err != nil {
		t.Fatalf("read templates dir: %v", err)
	}
	labelRe := regexp.MustCompile(`(?s)<label\b[^>]*>`)

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".html") {
			continue
		}
		data, err := templatesFS.ReadFile("templates/" + e.Name())
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		content := string(data)
		for _, loc := range labelRe.FindAllStringIndex(content, -1) {
			tag := content[loc[0]:loc[1]]
			if strings.Contains(tag, "for=") {
				continue
			}
			rest := content[loc[1]:]
			inner := rest
			if end := strings.Index(rest, "</label>"); end >= 0 {
				inner = rest[:end]
			}
			if strings.Contains(inner, "<input") || strings.Contains(inner, "<select") || strings.Contains(inner, "<textarea") {
				continue
			}
			t.Errorf("%s: <label> has no for= and no nested control: %s", e.Name(), tag)
		}
	}
}

func TestToastHasRole(t *testing.T) {
	data, err := templatesFS.ReadFile("templates/toast.html")
	if err != nil {
		t.Fatalf("read toast.html: %v", err)
	}
	if !strings.Contains(string(data), "role=") {
		t.Error("toast.html must set a role for screen readers")
	}
}

func TestNavHasAriaCurrent(t *testing.T) {
	data, err := templatesFS.ReadFile("templates/base.html")
	if err != nil {
		t.Fatalf("read base.html: %v", err)
	}
	if !strings.Contains(string(data), `aria-current="page"`) {
		t.Error("base.html nav must mark the current page with aria-current")
	}
}
