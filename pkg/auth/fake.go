// ABOUTME: Fake authentication for ish mode testing
// ABOUTME: Provides Bearer token auth without real OAuth

package auth

import (
	"fmt"
	"net/http"
	"os"
)

// fakeTransport adds Bearer token authentication to requests
type fakeTransport struct {
	token string
	base  http.RoundTripper
}

func (t *fakeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", t.token))
	return t.base.RoundTrip(req)
}

// NewFakeClient creates an HTTP client with fake Bearer token auth
func NewFakeClient(user string) *http.Client {
	if user == "" {
		user = os.Getenv("ISH_USER")
		if user == "" {
			user = "testuser"
		}
	}

	token := fmt.Sprintf("user:%s", user)

	return &http.Client{
		Transport: &fakeTransport{
			token: token,
			base:  http.DefaultTransport,
		},
	}
}
