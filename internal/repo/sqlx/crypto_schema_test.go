package sqlxrepo

import (
	"reflect"
	"testing"

	cryptoschema "github.com/webitel/crypto/cryptostore/schema"
	wcrypto "github.com/webitel/webitel-cryptoctl/schema"
)

// TestCryptoBaselineIsChat: the encryption baseline is the shared chat schema from
// webitel-cryptoctl (also shipped to cryptoctl), not a local copy.
func TestCryptoBaselineIsChat(t *testing.T) {
	t.Setenv("CRYPTO_CIPHER_KEYRING", "correct-horse-battery-staple-001!")
	if err := CryptoInit(); err != nil {
		t.Fatal(err)
	}
	if got := cryptoschema.Baseline(); !reflect.DeepEqual(got, wcrypto.Chat) {
		t.Fatalf("baseline = %+v, want webitel-cryptoctl Chat %+v", got, wcrypto.Chat)
	}
	if Crypto().Unit(schemaTableChatGate) == nil {
		t.Fatalf("codec has no unit %s", schemaTableChatGate)
	}
}
