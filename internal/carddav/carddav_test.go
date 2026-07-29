package carddav

import (
	"testing"

	"github.com/emersion/go-vcard"
)

func TestExtractContact(t *testing.T) {
	card := vcard.Card{
		"FN":    []*vcard.Field{{Value: "John Doe"}},
		"EMAIL": []*vcard.Field{{Value: "john@example.com"}},
	}
	name, email := extractContact(card)
	if name != "John Doe" {
		t.Errorf("name = %q, want 'John Doe'", name)
	}
	if email != "john@example.com" {
		t.Errorf("email = %q, want 'john@example.com'", email)
	}
}

func TestExtractContactNameByN(t *testing.T) {
	card := vcard.Card{
		"EMAIL": []*vcard.Field{{Value: "jane@example.com"}},
	}
	emailField := &vcard.Field{Value: "Doe;Jane;;;"}
	card["N"] = []*vcard.Field{emailField}

	name, email := extractContact(card)
	if name != "Jane Doe" {
		t.Errorf("name = %q, want 'Jane Doe'", name)
	}
	if email != "jane@example.com" {
		t.Errorf("email = %q", email)
	}
}

func TestExtractContactNoName(t *testing.T) {
	card := vcard.Card{
		"EMAIL": []*vcard.Field{{Value: "no-name@example.com"}},
	}
	name, email := extractContact(card)
	if name != "no-name@example.com" {
		t.Errorf("name = %q, want 'no-name@example.com'", name)
	}
	if email != "no-name@example.com" {
		t.Errorf("email = %q", email)
	}
}

func TestExtractContactNoEmail(t *testing.T) {
	card := vcard.Card{
		"FN": []*vcard.Field{{Value: "No Email"}},
	}
	_, email := extractContact(card)
	if email != "" {
		t.Errorf("email = %q, want empty", email)
	}
}

func TestExtractContactEmptyCard(t *testing.T) {
	card := vcard.Card{}
	name, email := extractContact(card)
	if name != "" || email != "" {
		t.Errorf("name = %q, email = %q, want empty", name, email)
	}
}
