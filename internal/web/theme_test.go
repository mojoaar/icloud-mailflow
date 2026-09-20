package web

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var themeBlockRe = regexp.MustCompile(`\[data-theme="([a-z-]+)"\]\s*\{([^}]*)\}`)
var tokenRe = regexp.MustCompile(`--([a-z-]+):\s*([^;]+);`)

var requiredThemeTokens = []string{
	"bg", "surface", "border", "text", "muted", "accent", "accent-fg",
	"primary-bg", "primary-hover", "primary-fg", "green", "red", "danger-fg",
	"warn", "chart-text", "chart-grid", "chart-accent", "chart-series",
}

func parseThemeBlocks(t *testing.T, css string) map[string]map[string]string {
	t.Helper()
	blocks := map[string]map[string]string{}
	for _, m := range themeBlockRe.FindAllStringSubmatch(css, -1) {
		name := m[1]
		tokens := map[string]string{}
		for _, tk := range tokenRe.FindAllStringSubmatch(m[2], -1) {
			tokens[tk[1]] = strings.TrimSpace(tk[2])
		}
		blocks[name] = tokens
	}
	return blocks
}

func parseThemeFamilies(t *testing.T, settingsHTML string) []string {
	t.Helper()
	selectRe := regexp.MustCompile(`(?s)<select id="theme-family".*?</select>`)
	sel := selectRe.FindString(settingsHTML)
	if sel == "" {
		t.Fatal("theme-family select not found in settings.html")
	}
	optRe := regexp.MustCompile(`value="([a-z-]+)"`)
	var families []string
	for _, m := range optRe.FindAllStringSubmatch(sel, -1) {
		families = append(families, m[1])
	}
	return families
}

func relLum(hex string) (float64, bool) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) != 6 {
		return 0, false
	}
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, false
	}
	ch := func(c uint64) float64 {
		s := float64(c) / 255
		if s <= 0.03928 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	r := ch(v >> 16 & 0xff)
	g := ch(v >> 8 & 0xff)
	b := ch(v & 0xff)
	return 0.2126*r + 0.7152*g + 0.0722*b, true
}

func contrast(a, b string) (float64, bool) {
	la, ok := relLum(a)
	if !ok {
		return 0, false
	}
	lb, ok := relLum(b)
	if !ok {
		return 0, false
	}
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05), true
}

func TestThemeParityAndTokens(t *testing.T) {
	cssBytes, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	settingsBytes, err := templatesFS.ReadFile("templates/settings.html")
	if err != nil {
		t.Fatalf("read settings.html: %v", err)
	}

	blocks := parseThemeBlocks(t, string(cssBytes))
	families := parseThemeFamilies(t, string(settingsBytes))
	if len(families) == 0 {
		t.Fatal("no theme families found in settings")
	}

	for _, fam := range families {
		for _, mode := range []string{"dark", "light"} {
			key := fam + "-" + mode
			tokens, ok := blocks[key]
			if !ok {
				t.Errorf("missing [data-theme=%q] block", key)
				continue
			}
			for _, want := range requiredThemeTokens {
				if tokens[want] == "" {
					t.Errorf("%s: missing token --%s", key, want)
				}
			}
		}
	}

	inSelect := map[string]bool{}
	for _, fam := range families {
		inSelect[fam] = true
	}
	for key := range blocks {
		fam := strings.TrimSuffix(strings.TrimSuffix(key, "-dark"), "-light")
		if !inSelect[fam] {
			t.Errorf("[data-theme=%q] has no matching option in the Settings theme dropdown", key)
		}
	}
}

func TestThemeContrast(t *testing.T) {
	cssBytes, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	blocks := parseThemeBlocks(t, string(cssBytes))

	type pair struct{ fg, bg string }
	textPairs := []pair{
		{"text", "bg"}, {"text", "surface"},
		{"muted", "bg"}, {"muted", "surface"},
		{"accent", "bg"}, {"accent", "surface"},
	}
	uiPairs := []pair{
		{"green", "bg"}, {"green", "surface"},
		{"red", "bg"}, {"red", "surface"},
		{"warn", "bg"}, {"warn", "surface"},
		{"accent-fg", "accent"},
		{"primary-fg", "primary-bg"},
		{"danger-fg", "red"},
	}

	check := func(block string, tokens map[string]string, pairs []pair, min float64) {
		for _, p := range pairs {
			fg, bg := tokens[p.fg], tokens[p.bg]
			if fg == "" || bg == "" {
				continue
			}
			ratio, ok := contrast(fg, bg)
			if !ok {
				t.Errorf("%s: unparseable colours --%s=%s --%s=%s", block, p.fg, fg, p.bg, bg)
				continue
			}
			if ratio < min {
				t.Errorf("%s: --%s on --%s = %.2f:1, want >= %.1f (%s on %s)", block, p.fg, p.bg, ratio, min, fg, bg)
			}
		}
	}

	if len(blocks) == 0 {
		t.Fatal("no theme blocks found")
	}
	for name, tokens := range blocks {
		check(name, tokens, textPairs, 4.5)
		check(name, tokens, uiPairs, 3.0)
	}
}

func TestThemeSeriesFormat(t *testing.T) {
	cssBytes, err := staticFS.ReadFile("static/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	blocks := parseThemeBlocks(t, string(cssBytes))
	for name, tokens := range blocks {
		series := tokens["chart-series"]
		if series == "" {
			t.Errorf("%s: missing --chart-series", name)
			continue
		}
		parts := strings.Split(series, ",")
		if len(parts) < 4 {
			t.Errorf("%s: --chart-series has %d colours, want at least 4", name, len(parts))
		}
		for _, p := range parts {
			if _, ok := relLum(strings.TrimSpace(p)); !ok {
				t.Errorf("%s: --chart-series entry %q is not a hex colour", name, strings.TrimSpace(p))
			}
		}
	}
}
