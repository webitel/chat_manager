package sqlxrepo

import (
	"fmt"
	"sync"

	"github.com/webitel/crypto/cryptostore/schema"
	wcrypto "github.com/webitel/webitel-cryptoctl/schema"
)

// CryptoInit configures cryptostore/schema.Codec plugin
func CryptoInit() error {
	crypto.init.Do(cryptoInit)
	return crypto.err
}

// guard: crypto.init.(sync.Once)
func cryptoInit() {
	// cryptostore/schema.(BASELINE)
	schema.Register(wcrypto.Chat) // shared chat schema (webitel-cryptoctl)
	// plugin: module environment configuration
	crypto.codec, crypto.err = schema.NewCodec(
		schema.DefaultOptions(),
	)
	if crypto.err == nil {
		// schema MAY define field(s) with the "search" option enabled
		// this require CRYPTO_SEARCH_{KEYRING|KEYFILE} to be specified
		crypto.err = crypto.codec.RequireIndex()
	}
	if crypto.err != nil {
		// wrap up general error details
		crypto.err = fmt.Errorf("crypto: configuration ; %w", crypto.err)
	}
}

// Crypto schema.Codec for data encryption
func Crypto() *schema.Codec {
	err := CryptoInit() // lazy: init
	if err != nil {
		panic(err)
	}
	return crypto.codec
}

// module: cryptostore/schema
var crypto struct {
	codec *schema.Codec
	init  sync.Once
	err   error
}
