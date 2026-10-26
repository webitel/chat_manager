package facebook

import (
	"testing"
)

const (
	testToken = "EAA-business-token"
	testPhone = "102406306073614"
)

func storedNumbers() map[string]string {
	return map[string]string{
		metaWhatsAppToken:   testToken,
		metaWhatsAppNumbers: "15550907777=" + testPhone + ",380501112233=999",
		"wa":                "whatever",
	}
}

// The number reaching us comes from whatever put it on the wire; Meta
// stores it formatted for people. Both reduce to the same digits.
func TestWhatsAppCallCredentialsIgnoresFormatting(t *testing.T) {
	for _, number := range []string{
		"15550907777",
		"+15550907777",
		"+1 (555) 090-7777",
		"1-555-090-7777",
	} {
		creds, ok := whatsAppCallCredentials(storedNumbers(), number)
		if !ok {
			t.Errorf("%q: not resolved", number)

			continue
		}
		if creds.PhoneNumberID != testPhone {
			t.Errorf("%q: PhoneNumberID = %q", number, creds.PhoneNumberID)
		}
		if creds.AccessToken != testToken {
			t.Errorf("%q: AccessToken = %q", number, creds.AccessToken)
		}
	}
}

func TestWhatsAppCallCredentialsRejections(t *testing.T) {
	cases := []struct {
		name     string
		metadata map[string]string
		number   string
	}{
		{"a number this bot does not serve", storedNumbers(), "15559999999"},
		{"no digits at all", storedNumbers(), "anonymous"},
		{"empty number", storedNumbers(), ""},
		{"nothing stored", map[string]string{}, "15550907777"},
		{"nil metadata", nil, "15550907777"},
		{
			"numbers but no token",
			map[string]string{metaWhatsAppNumbers: "15550907777=" + testPhone},
			"15550907777",
		},
		{
			"token but no numbers",
			map[string]string{metaWhatsAppToken: testToken},
			"15550907777",
		},
		{
			// A prefix must not match: 1555090777 is a different number.
			"a shorter number that is a prefix",
			storedNumbers(),
			"1555090777",
		},
		{
			"an entry with no id",
			map[string]string{
				metaWhatsAppToken:   testToken,
				metaWhatsAppNumbers: "15550907777=",
			},
			"15550907777",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, ok := whatsAppCallCredentials(tc.metadata, tc.number); ok {
				t.Fatal("resolved")
			}
		})
	}
}

// Stored is what decides whether a negative Lookup is trusted, or the bot
// has to be started and asked. Getting it wrong either starts every bot on
// every call, or hides a bot that does serve the number.
func TestWhatsAppNumbersStored(t *testing.T) {
	if !whatsAppNumbersStored(storedNumbers()) {
		t.Error("stored numbers were not recognised")
	}
	if whatsAppNumbersStored(map[string]string{"wa": "x", metaWhatsAppToken: testToken}) {
		t.Error("a bot that never stored its numbers looked complete")
	}
	if whatsAppNumbersStored(nil) {
		t.Error("nil metadata looked complete")
	}
}

func TestWhatsAppNumbersMetadataOfNilManager(t *testing.T) {
	if got := whatsAppNumbersMetadata(nil); got != "" {
		t.Fatalf("rendered %q for no manager", got)
	}
}
