package client

import (
	"fmt"
	"strings"
	"time"
)

type contextKey string

const (
	expiryTimeSkew int64 = 120

	TokenParamsContextKey contextKey = "tokenParams"
	TokenContextKey       contextKey = "token"
)

// GetTokenParams defines the parameters required to get an access token.
type GetTokenParams struct {
	// The client ID of the application.
	// Required: true
	ClientID string `json:"client_id"`
	// The client secret of the application.
	// Required: true
	ClientSecret string `json:"client_secret"`
	// The scope of the access token.
	// Required: true
	Scope string `json:"scope"`
	// The URL of the token endpoint.
	// Required: true
	AuthURL string `json:"auth_url"`
	// The API URL for this token.
	// Required: true
	ApiURL string `json:"api_url"`
}

func (gtp *GetTokenParams) GetClientID() string {
	return gtp.ClientID
}

func (gtp *GetTokenParams) getClientSecret() string {
	return gtp.ClientSecret
}

func (gtp *GetTokenParams) GetScope() string {
	return gtp.Scope
}

func (gtp *GetTokenParams) GetAuthURL() string {
	return gtp.AuthURL
}

func (gtp *GetTokenParams) GetApiURL() string {
	return gtp.ApiURL
}

func (gtp *GetTokenParams) IsValid() error {
	if !strings.HasSuffix(gtp.AuthURL, "oauth2/token") {
		return fmt.Errorf("The AuthURL must end with /oauth2/token")
	}
	return nil
}

type oauth2Response struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

// Token represents a Fragment API access token.
type Token struct {
	// The Access Token.
	AccessToken string
	// The expiration time for this token.
	ExpiresAt time.Time
}
