package graph

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/url"
)

// https://developers.facebook.com/docs/graph-api/security/#appsecret_proof
func SecretProof(accessToken, clientSecret string) string {
	
	algo := sha256.New
	hash := hmac.New(algo, []byte(clientSecret))
	_, _ = hash.Write([]byte(accessToken))
	hsum := hash.Sum(nil)

	return hex.EncodeToString(hsum)
}

const (

	ParamAccessToken = "access_token"
	ParamSecretProof = "appsecret_proof"
)

func FormRequest(form url.Values, accessToken, clientSecret string) url.Values {

	if form == nil {
		form = url.Values{}
	}

	if accessToken != "" {
		form.Set(ParamAccessToken, accessToken)
		if clientSecret != "" {
			form.Set(ParamSecretProof, SecretProof(
				accessToken, clientSecret,
			))
		}
	}

	return form
}