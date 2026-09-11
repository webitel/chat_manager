package bot

import (
	"fmt"

	"github.com/webitel/crypto/cryptobox"
	// "github.com/webitel/crypto/cryptostore"
)

// Cipher returns the process-wide cryptobox.Cipher
// initialized from environment configuration
func Cipher() cryptobox.Cipher {
	cbox, err := cryptobox.Default()
	if err != nil {
		panic(fmt.Errorf("cryptobox: configuration ; %w", err))
	}
	return cbox
}

// func Crypto() *cryptostore.Codec {
// 	codec, err := cryptostore.Default()
// 	if err != nil {
// 		panic(fmt.Errorf("cryptostore: configuration ; %w", err))
// 	}
// 	return codec
// }