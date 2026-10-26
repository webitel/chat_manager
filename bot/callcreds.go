package bot

// NumberCredentials is what a provider's calling API needs to authorise a
// call placed from one of its numbers.
type NumberCredentials struct {
	// PhoneNumberID is the provider's own identifier for the number, which
	// its API is addressed by. It is not the number.
	PhoneNumberID string
	// AccessToken authorises calls from that number, without a scheme.
	AccessToken string
	// ExpiresAt is unix seconds, or zero when the token does not expire on
	// a schedule. A caller may hold the credentials until then.
	ExpiresAt int64
}

// CallCredentialsProvider is implemented by a running bot whose numbers can
// place calls. It is optional: the API asks for it with a type assertion,
// so a provider that cannot place calls needs to do nothing.
type CallCredentialsProvider interface {
	// CallCredentials resolves one of this provider's own numbers. ok is
	// false when this provider does not serve that number, which is the
	// normal answer while looking for the one that does.
	CallCredentials(number string) (creds NumberCredentials, ok bool)
}

// CallCredentialsMetadata answers the same question from a bot's stored
// metadata, without loading or starting it.
//
// This is the path that matters. Resolving a number has to consider every
// bot that might serve it, and starting a bot costs a round trip to the
// provider; doing that per candidate turns one call into a storm of them.
// Reading metadata costs nothing: the search already fetched it.
type CallCredentialsMetadata struct {
	// Lookup resolves the number. ok is false when this bot does not serve
	// it.
	Lookup func(metadata map[string]string, number string) (creds NumberCredentials, ok bool)
	// Stored reports whether the metadata holds everything Lookup needs.
	// False means a negative answer from Lookup proves nothing, and the
	// bot has to be started and asked instead.
	Stored func(metadata map[string]string) bool
}

// callingProviders are the providers whose bots may serve calls, with the
// metadata reader each one registered.
var callingProviders = map[string]CallCredentialsMetadata{}

// RegisterCalling marks a provider as one whose bots may place calls, and
// gives the reader for its stored metadata. Call it from the provider's
// init, next to Register.
func RegisterCalling(provider string, md CallCredentialsMetadata) {
	callingProviders[provider] = md
}

// CallingMetadata returns the metadata reader for a provider, and whether
// bots of that provider are worth asking about a number at all.
func CallingMetadata(provider string) (CallCredentialsMetadata, bool) {
	md, ok := callingProviders[provider]

	return md, ok
}
