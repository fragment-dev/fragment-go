package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockTokenParams struct {
	authUrl string
}

func (mtp mockTokenParams) GetClientId() string {
	return "test_client_id"
}

func (mtp mockTokenParams) GetClientSecret() string {
	return "test_client_secret"
}

func (mtp mockTokenParams) GetScope() string {
	return "*"
}

func (mtp mockTokenParams) GetAuthUrl() string {
	return mtp.authUrl
}

func (mtp mockTokenParams) GetApiUrl() string {
	return "https://api.fragment.dev/graphql"
}

func (mtp mockTokenParams) IsValid() error {
	return nil
}

func TestGetAuthenticatedContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			t.Errorf("Failed to parse form: %s", err)
		}
		// Validate query parameters
		if r.Form.Get("client_id") != "test_client_id" {
			t.Errorf("Expected client_id test_client_id, got %s", r.URL.Query().Get("client_id"))
		}
		if r.Form.Get("scope") != "*" {
			t.Errorf("Expected client_secret *, got %s", r.URL.Query().Get("scope"))
		}
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("Expected grant_type client_credentials, got %s", r.URL.Query().Get("grant_type"))
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

	authedContext, err := GetAuthenticatedContext(context.TODO(), mockTokenParams{server.URL})
	if err != nil {
		t.Errorf("Got error from GetAuthenticatedContext: %s", err)
	}

	token, ok := authedContext.GetToken()
	if !ok {
		t.Errorf("Failed to get token from context")
	}
	if token.AccessToken != "token" {
		t.Errorf("Expected access token token, got %s", token.AccessToken)
	}
	if !time.Unix(time.Now().Unix()+3600-expiryTimeSkew+1, 0).After(token.ExpiresAt) {
		t.Errorf("Expected token to expire in 3600 seconds, got %s", token.ExpiresAt)
	}
}
