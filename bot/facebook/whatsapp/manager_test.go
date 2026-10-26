package whatsapp

import "testing"

// Meta reports a number as display_phone_number, formatted for people:
// "+1 555 090 7777". A caller placing a call has it in whatever shape its
// own signalling carried, usually bare digits. Both have to match.
func TestLookupPhoneNumberIgnoresFormatting(t *testing.T) {
	const (
		waID   = "106540352242922"
		stored = "+1 555 090 7777"
	)

	m := &Manager{
		AccessToken: "tok",
		PhoneNumbers: map[string]*WhatsAppPhoneNumber{
			waID: {WhatsAppBusinessAccountToNumberCurrentStatus: &WhatsAppBusinessAccountToNumberCurrentStatus{
				ID:          waID,
				PhoneNumber: stored,
			}},
		},
	}

	for _, asked := range []string{
		"15550907777",
		"+15550907777",
		"+1 555 090 7777",
		"+1 (555) 090-7777",
		"1-555-090-7777",
	} {
		got := m.LookupPhoneNumber(asked)
		if got == nil {
			t.Errorf("LookupPhoneNumber(%q) found nothing", asked)

			continue
		}
		if got.ID != waID {
			t.Errorf("LookupPhoneNumber(%q).ID = %q, want %q", asked, got.ID, waID)
		}
	}
}

func TestLookupPhoneNumberRejects(t *testing.T) {
	m := &Manager{
		PhoneNumbers: map[string]*WhatsAppPhoneNumber{
			"1": {WhatsAppBusinessAccountToNumberCurrentStatus: &WhatsAppBusinessAccountToNumberCurrentStatus{
				ID: "1", PhoneNumber: "+1 555 090 7777",
			}},
			// A half-registered entry must not panic the scan.
			"2": {},
		},
	}

	cases := map[string]string{
		"a different number":   "380730367300",
		"a prefix is no match": "1555090",
		"empty":                "",
		"no digits at all":     "+ () -",
	}

	for name, asked := range cases {
		if got := m.LookupPhoneNumber(asked); got != nil {
			t.Errorf("%s: LookupPhoneNumber(%q) = %q, want nothing", name, asked, got.ID)
		}
	}
}

func TestLookupPhoneNumberOnEmptyManager(t *testing.T) {
	// Before any account is registered, and before the manager exists.
	if got := (&Manager{}).LookupPhoneNumber("15550907777"); got != nil {
		t.Fatalf("got %v from an empty manager", got)
	}

	var nilManager *Manager
	if got := nilManager.LookupPhoneNumber("15550907777"); got != nil {
		t.Fatalf("got %v from a nil manager", got)
	}
}

func TestEachPhoneNumberHandlesNilManager(t *testing.T) {
	var m *Manager

	called := false
	m.EachPhoneNumber(func(*WhatsAppPhoneNumber) { called = true })

	if called {
		t.Fatal("a nil manager yielded a number")
	}
}
