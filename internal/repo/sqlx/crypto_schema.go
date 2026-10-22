package sqlxrepo

import (
	"fmt"
	"sync"

	"github.com/webitel/crypto/cryptostore/schema"
)

// CryptoInit configures cryptostore/schema.Codec plugin
func CryptoInit() error {
	crypto.init.Do(cryptoInit)
	return crypto.err
}

// guard: crypto.init.(sync.Once)
func cryptoInit() {
	// cryptostore/schema.(BASELINE)
	schema.Register(&crypto.schema)
	// plugin: module environment configuration
	crypto.codec, crypto.err = schema.NewCodec(
		schema.DefaultOptions(),
	)
	if crypto.err == nil {
		// schema MAY define field(s) with the "search" option enabled
		// this require WBTL_CRYPTO_SEARCH_{KERING|KEYFILE} to be specified
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
var crypto = struct {
	// baseline (mandatory) schema
	schema schema.Config
	codec *schema.Codec
	init sync.Once
	err error
} {

	schema: schema.Config{
		Version: 1,
		Units: map[string]*schema.Unit{
			schemaTableChatGate: {
				Fields: map[string]*schema.FieldPolicy{
					"metadata": {Nested: []schema.FieldNested{
						// custom
						{Path: []string{"secret"}},
						// viber
						// telegram
						{Path: []string{"token"}},
						// infobip_whatsapp
						{Path: []string{"api_key"}},
						// gotd
						{Path: []string{"api_hash"}},
						// gotd: backup sensitive data
						{Path: []string{".auth"}},
						{Path: []string{".gotd"}}, // base64; {AuthKey,AuthKeyID,Salt}
						// messenger
						{Path: []string{"client_secret"}},
						{Path: []string{"whatsapp_token"}},
						// messenger: backup sensitive data 
						{Path: []string{"fb"}}, // facebook: pages
						{Path: []string{"ig"}}, // instagram: pages
						{Path: []string{"wa"}}, // whatsapp: numbers
					}},
				},
			},
		},
	},

}
