package gotd

import (
	"context"
	"encoding/base64"
)

// NOTE: here we have double base64 encoding overhead
//
// - first for protobuf binary to text   ; Bot.Metadata.(map[string]string)
// - second cryptostore encryption wrap  ; plaintext | ciphertext
//
// BUT: in such case we fully support crypto key migration !..

var (
	binText = base64.RawStdEncoding
)

func (c *session) backupData(_ context.Context, data []byte) (text string) {
	return binText.EncodeToString(data)
}

func (c *session) restoreData(_ context.Context, text string) (data []byte, err error) {
	return binText.DecodeString(text)
}

// 	// FIXME: maybe we can use "cbox:" default cryptostore.FrameText() value schema ?
// 	//        to enable cryptobox encryption key(s) migration ..
// 	// IDEA:  while metadata JSON value is NOT marked with "cbox:" prefix 
// 	//        & NO nested[ "fb", "ig", "wa" ] schema attributes defined -
// 	//        cmd/cryptoctl will not encrypt such JPath(s) due to missing declaration
// 	//        BUT when we mark value with "cbox:" prefix - postgres.cryptostore_kids() will see this self-marked attributes to process (its NOT true)
// 	// ISSUE: but while fetching gateway metadata, driver will try [auto-]decrypt "cbox:.." value which will cause data corruption  =(( 
// 	//
// const backupForm = "cbak." // self-marked ; DOT(".") outside of base64 alphabet

// func (c *session) backupData(ctx context.Context, data []byte) string {
// 	if len(data) == 0 {
// 		// no data
// 		return ""
// 	}
// 	// // v2 / strong encryption
// 	// blob, err := bot.Crypto().EncryptText(ctx, data)
// 	// if err == nil {
// 	// 	return blob // ciphertext
// 	// }
// 	// // keep it plain until next update ..
// 	// c.App.Gateway.Log.Warn(
// 	// 	"backup: failed to protect data",
// 	// 	slog.String("error", err.Error()),
// 	// )
// 	// // v1 / weak encoding
// 	// return binaryText.EncodeToString(data)

// 	blob, err := bot.Cipher().Encrypt(ctx, data)
// 	strong := (err == nil) // succeed ?
// 	if !strong { // err != nil {
// 		// LOG: failed to encypt sensitive data
// 		c.App.Gateway.Log.Warn(
// 			"backup: failed to protect data",
// 			slog.String("error", err.Error()),
// 		)
// 		// unprotected: plain
// 		blob = data
// 	}
// 	text := binaryText.EncodeToString(blob)
// 	if text != "" && strong {
// 		text = (backupForm + text)
// 	}
// 	// OK ; ( strong ? v2 : v1 )
// 	return text
// }

// func (c *session) restoreData(ctx context.Context, text string) (data []byte, err error) {
// 	if text == "" {
// 		// no data
// 		return nil, nil
// 	}
// 	// ? v2 / strong (ciphertext)
// 	text, strong := strings.CutPrefix(text, backupForm)
// 	data, err = binaryText.DecodeString(text)
// 	if err != nil {
// 		c.App.Gateway.Log.Error(
// 			"backup: failed to decode data",
// 			slog.String("error", err.Error()),
// 		)
// 		return nil, fmt.Errorf("backup: failed to decode data; %w", err)
// 	}
// 	if !strong {
// 		// OK ; v1 / weak (plaintext)
// 		return data, nil
// 	}
// 	// ! v2 / strong (ciphertext)
// 	data, err = bot.Cipher().Decrypt(ctx, data)
// 	if err != nil {
// 		c.App.Gateway.Log.Error(
// 			"backup: failed to decrypt data",
// 			slog.String("error", err.Error()),
// 		)
// 		return nil, fmt.Errorf("backup: failed to decrypt data; %w", err)
// 	}
// 	// OK ; v2 / strong (ciphertext)
// 	return data, nil
// }