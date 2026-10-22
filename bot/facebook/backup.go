package facebook

import (
	"encoding/base64"
)

// NOTE: here we have double base64-based encoding
//
// - first for protobuf binary to text   ; Bot.Metadata.(map[string]string)
// - second cryptostore encryption wrap  ; plaintext | ciphertext
//
// BUT: in such case we fully support crypto key migration !..

var (
	binText = base64.RawURLEncoding
)

func backupData(data []byte) (text string) {
	return binText.EncodeToString(data)
	// text, err := encryptData(data)
	// if err != nil {
	// 	panic(fmt.Errorf("failed to encrypt sensitive data; %w", err))
	// }
	// return text
}

func restoreData(text string) (data []byte, err error) {
	return binText.DecodeString(text)
	// data, err := decryptData(text)
	// // if err != nil {
	// // 	err = fmt.Errorf("failed to decrypt sensitive data; %w", err)
	// // }
	// return data, err
}


// const (
// 	// FIXME: maybe we can use "cbox:" default cryptostore.FrameText() value schema ?
// 	//        to enable cryptobox encryption key(s) migration ..
// 	// IDEA:  while metadata JSON value is NOT marked with "cbox:" prefix 
// 	//        & NO nested[ "fb", "ig", "wa" ] schema attributes declared -
// 	//        cmd/cryptoctl will not encrypt such JPath(s) due to missing in cryptostore/schema declaration
// 	//        BUT when we mark value with "cbox:" prefix - postgres.cryptostore_kids() will see this self-marked attributes to process
// 	// ISSUE: but while fetching gateway metadata, driver will try [auto]decrypt "cbox:.." value which will cause data corruption  =(( 
// 	//
// 	binForm = "cbak." // self-marked ; DOT(".") outside of base64 alphabet
// )

// func encryptData(data []byte) (text string, err error) {
// 	if len(data) == 0 {
// 		// no data
// 		return "", nil
// 	}
// 	// v2. Strong (encryption) encoding !
// 	blob, err := bot.Cipher().Encrypt(
// 		context.Background(), data,
// 	)
// 	if err != nil {
// 		return "", fmt.Errorf("backup: failed to encrypt sensitive data; %w", err)
// 	}
// 	// v1. Weak (text) encoding
// 	text = binText.EncodeToString(blob)
// 	if text != "" {
// 		text = (binForm + text)
// 	}
// 	return text, nil
// }

// func decryptData(text string) (data []byte, err error) {
// 	if text == "" {
// 		// no data
// 		return nil, nil
// 	}
// 	// 0. Check is LATEST (strong) encoding ?
// 	text, strong := strings.CutPrefix(text, binForm)
// 	// 1. Weak (legacy) encoding
// 	data, err = binText.DecodeString(text)
// 	if err != nil {
// 		return nil, fmt.Errorf("backup: failed to decode data; %w", err)
// 	}
// 	// 2. Strong (encryption) encoding ?
// 	if strong {
// 		blob := data // ciphertext
// 		data, err = bot.Cipher().Decrypt(
// 			context.Background(), data,
// 		)
// 		if err != nil {
// 			// failed to decrypt sensitive data
// 			return blob, fmt.Errorf("backup: failed to decrypt data; %w", err)
// 		}
// 	}
// 	// OK
// 	return data, nil
// }