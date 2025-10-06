package client

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type mockAlwaysAfterClock struct{}

func (mockAlwaysAfterClock) Now() time.Time {
	return time.Unix(9999999999, 0) // very far in the future
}

type mockAlwaysBeforeClock struct{}

func (mockAlwaysBeforeClock) Now() time.Time {
	return time.Unix(0, 0)
}

func getMockServer(t_ *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"new_access_token","expires_in":3600}`))
	}))
}

func TestTokenRefresh(t *testing.T) {
	server := getMockServer(t)
	defer server.Close()

	tokenParams := &GetTokenParams{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Scope:        "*",
		AuthURL:      server.URL + "/oauth2/token",
		ApiURL:       server.URL,
	}
	client := newHttpClient(&mockAlwaysAfterClock{}, tokenParams)

	// Set an initial expired token
	client.SetToken(&Token{
		AccessToken: "old_expired_token",
		ExpiresAt:   time.Unix(1, 0), // Already expired
	})

	serverURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("Failed to parse server URL: %s", err)
	}

	// This should attempt to refresh the token because it's expired
	_, err = client.Do(&http.Request{URL: serverURL, Header: http.Header{}})
	if err != nil {
		t.Errorf("Expected no error, got: %s", err)
	}
	if client.GetToken().AccessToken != "new_access_token" {
		t.Errorf("Expected client's access token to equal mock returned access token")
	}

	// The next attempt should skip refresh the token because it's still valid
	currentToken := client.GetToken()
	_, err = client.Do(&http.Request{URL: serverURL, Header: http.Header{}})
	if err != nil {
		t.Errorf("Expected no error, got: %s", err)
	}
	if currentToken != client.GetToken() {
		t.Errorf("Expected client's token to stay the same")
	}
}

func TestGenerateToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %s", err)
		}
		// Validate query parameters
		if r.Form.Get("client_id") != "test_client_id" {
			t.Errorf("Expected client_id test_client_id, got %s", r.Form.Get("client_id"))
		}
		if r.Form.Get("scope") != "*" {
			t.Errorf("Expected scope *, got %s", r.Form.Get("scope"))
		}
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("Expected grant_type client_credentials, got %s", r.Form.Get("grant_type"))
		}

		// Validate request headers
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Expected Content-Type application/x-www-form-urlencoded, got %s", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Accept") != "*/*" {
			t.Errorf("Expected Accept */*, got %s", r.Header.Get("Accept"))
		}

		authHeader := strings.Split(r.Header.Get("Authorization"), " ")
		if len(authHeader) != 2 {
			t.Errorf("Expected Authorization header to have 2 parts, got %d", len(authHeader))
		}
		if authHeader[0] != "Basic" {
			t.Errorf("Expected Authorization header to start with Basic, got %s", authHeader[0])
		}

		hb, err := base64.StdEncoding.DecodeString(authHeader[1])
		if err != nil {
			t.Errorf("Failed to decode base64: %s", err)
		}
		if string(hb) != "test_client_id:test_client_secret" {
			t.Errorf("Expected Authorization header to be test_client_id:test_client_secret, got %s", string(hb))
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"token","expires_in":3600}`))
	}))
	defer server.Close()

	tokenParams := &GetTokenParams{
		ClientID:     "test_client_id",
		ClientSecret: "test_client_secret",
		Scope:        "*",
		AuthURL:      server.URL + "/oauth2/token",
		ApiURL:       server.URL,
	}
	client := newHttpClient(nil, tokenParams)

	token, err := client.GenerateToken(context.TODO())
	if err != nil {
		t.Errorf("Got error from GenerateToken: %s", err)
	}

	if token.AccessToken != "token" {
		t.Errorf("Expected access token token, got %s", token.AccessToken)
	}
	if !time.Unix(time.Now().Unix()+3600-expiryTimeSkew+1, 0).After(token.ExpiresAt) {
		t.Errorf("Expected token to expire in ~3600 seconds, got %s", token.ExpiresAt)
	}
}
