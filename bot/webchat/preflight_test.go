package webchat

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	pb "github.com/webitel/chat_manager/api/proto/bot"
	"github.com/webitel/chat_manager/bot"
)

func TestWebChatBot_Preflight(t *testing.T) {
	tests := []struct {
		name        string
		allowOrigin string
		origin      string
		wantCode    int
		wantAllowed bool
	}{
		{name: "allowed origin", allowOrigin: "https://a.example.com", origin: "https://a.example.com", wantCode: http.StatusOK, wantAllowed: true},
		{name: "foreign origin", allowOrigin: "https://a.example.com", origin: "https://b.example.com", wantCode: http.StatusForbidden},
		{name: "any origin", allowOrigin: "*", origin: "https://b.example.com", wantCode: http.StatusOK, wantAllowed: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider, err := New(&bot.Gateway{
				Bot: &pb.Bot{Metadata: map[string]string{"allow_origin": tt.allowOrigin}},
				Log: slog.New(slog.DiscardHandler),
			}, nil)
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			req := httptest.NewRequest(http.MethodOptions, "/chat/gw?filename=a.txt", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", http.MethodPost)

			rsp := httptest.NewRecorder()

			provider.(*WebChatBot).Preflight(rsp, req)

			if rsp.Code != tt.wantCode {
				t.Errorf("code = %d, want %d", rsp.Code, tt.wantCode)
			}

			wantOrigin, wantCredentials := "", ""
			if tt.wantAllowed {
				wantOrigin, wantCredentials = tt.origin, "true"
			}

			if got := rsp.Header().Get("Access-Control-Allow-Origin"); got != wantOrigin {
				t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, wantOrigin)
			}

			if got := rsp.Header().Get("Access-Control-Allow-Credentials"); got != wantCredentials {
				t.Errorf("Access-Control-Allow-Credentials = %q, want %q", got, wantCredentials)
			}
		})
	}
}
