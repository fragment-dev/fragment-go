package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/fragment-dev/fragment-go/auth"
)

type mockAlwaysAfterClock struct{}

func (mockAlwaysAfterClock) Now() time.Time {
	return time.Unix(1, 0)
}

type mockAlwaysBeforeClock struct{}

func (mockAlwaysBeforeClock) Now() time.Time {
	return time.Unix(0, 0)
}

type mockAuthContext struct {
	context.Context
}

func (mac *mockAuthContext) GetToken() (*auth.Token, bool) {
	token, ok := mac.Context.Value(auth.TokenContextKey).(*auth.Token)
	return token, ok
}

func (mac *mockAuthContext) SetToken(token *auth.Token) {
	mac.Context = context.WithValue(mac.Context, auth.TokenContextKey, token)
}

func (mac *mockAuthContext) GetTokenParams() auth.TokenParams {
	return mac.Context.Value(auth.TokenParamsContextKey).(*auth.MockTokenParams)
}

func getMockedAuthenticatedContext(serverUrl string) *mockAuthContext {
	mac := &mockAuthContext{
		context.WithValue(
			context.TODO(), auth.TokenParamsContextKey, &auth.MockTokenParams{serverUrl})}
	mac.SetToken(&auth.Token{
		AccessToken: "access_token",
		ExpiresAt:   time.Unix(0, 0),
	})
	return mac
}

func getMockServer(t_ *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Return a 200 if the content type is not application/x-www-form-urlencoded
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusForbidden)
		}
		w.Write([]byte(`{"access_token":"new_access_token","expires_in":3600}`))
	}))
}

func TestTokenRefresh(t *testing.T) {
	server := getMockServer(t)
	defer server.Close()

	mac := getMockedAuthenticatedContext(server.URL)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Errorf("Failed to parse server URL: %s", err)
	}

	_, err = newHttpClient(mac, &mockAlwaysAfterClock{}).Do(&http.Request{URL: serverURL, Header: http.Header{}})
	// Check that the request was successful
	if err == nil {
		t.Errorf("Expected error, got nil")
	}
}

func TestTokenSkipRefresh(t *testing.T) {
	server := getMockServer(t)
	defer server.Close()

	mac := getMockedAuthenticatedContext(server.URL)

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Errorf("Failed to parse server URL: %s", err)
	}

	_, err = newHttpClient(mac, &mockAlwaysBeforeClock{}).Do(&http.Request{URL: serverURL, Header: http.Header{}})
	// Check that the request was successful
	if err != nil {
		t.Errorf("Got error from Do: %s", err)
	}
}
