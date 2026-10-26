package bot

import (
	"context"
	"net/http"
	"testing"
)

// fakeCaller is a running bot that serves one number.
type fakeCaller struct {
	number string
	creds  NumberCredentials
	asked  []string
}

var (
	_ Provider                = (*fakeCaller)(nil)
	_ CallCredentialsProvider = (*fakeCaller)(nil)
)

func (f *fakeCaller) String() string                             { return "fake" }
func (f *fakeCaller) SendNotify(context.Context, *Update) error  { return nil }
func (f *fakeCaller) WebHook(http.ResponseWriter, *http.Request) {}
func (f *fakeCaller) Register(context.Context, string) error     { return nil }
func (f *fakeCaller) Deregister(context.Context) error           { return nil }
func (f *fakeCaller) Close() error                               { return nil }

func (f *fakeCaller) CallCredentials(number string) (NumberCredentials, bool) {
	f.asked = append(f.asked, number)

	if number != f.number {
		return NumberCredentials{}, false
	}

	return f.creds, true
}

func TestRunningCallCredentials(t *testing.T) {
	want := NumberCredentials{PhoneNumberID: "102406306073614", AccessToken: "tok"}

	caller := &fakeCaller{number: "15550907777", creds: want}
	srv := &Service{
		profiles: map[int64]*Gateway{
			7: {External: caller},
		},
	}

	t.Run("a running bot answers from memory", func(t *testing.T) {
		got, ok := srv.runningCallCredentials(7, "15550907777")
		if !ok {
			t.Fatal("not resolved")
		}
		if got != want {
			t.Fatalf("credentials = %+v, want %+v", got, want)
		}
	})

	t.Run("a number it does not serve", func(t *testing.T) {
		if _, ok := srv.runningCallCredentials(7, "15559999999"); ok {
			t.Fatal("resolved a number it does not serve")
		}
	})

	// The whole point: a bot that is not loaded must not be started, so a
	// number nobody serves costs nothing and cannot fail on a dead app.
	t.Run("a bot that is not running is not started", func(t *testing.T) {
		if _, ok := srv.runningCallCredentials(999, "15550907777"); ok {
			t.Fatal("resolved through a bot that is not running")
		}
	})

	t.Run("an empty registry", func(t *testing.T) {
		empty := &Service{}

		if _, ok := empty.runningCallCredentials(7, "15550907777"); ok {
			t.Fatal("resolved with no bots running")
		}
	})
}

func TestCallingRegistry(t *testing.T) {
	const provider = "test-provider-calling"

	if _, ok := CallingMetadata("test-provider-silent"); ok {
		t.Fatal("an unregistered provider was reported as calling")
	}

	RegisterCalling(provider, CallCredentialsMetadata{
		Lookup: func(map[string]string, string) (NumberCredentials, bool) {
			return NumberCredentials{PhoneNumberID: "id", AccessToken: "tok"}, true
		},
		Stored: func(map[string]string) bool { return true },
	})
	t.Cleanup(func() { delete(callingProviders, provider) })

	md, ok := CallingMetadata(provider)
	if !ok {
		t.Fatal("a registered provider was not reported as calling")
	}
	if _, found := md.Lookup(nil, "15550907777"); !found {
		t.Fatal("the registered lookup was not returned")
	}
	if !md.Stored(nil) {
		t.Fatal("the registered completeness check was not returned")
	}
}
