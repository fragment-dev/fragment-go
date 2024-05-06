package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	expiryTimeSkew int64 = 120

	TokenParamsContextKey = "tokenParams"
	TokenContextKey       = "token"
)

// GetTokenParams defines the parameters required to get an access token.
type GetTokenParams struct {
	// The client ID of the application.
	// Required: true
	ClientId string `json:"client_id"`
	// The client secret of the application.
	// Required: true
	ClientSecret string `json:"client_secret"`
	// The scope of the access token.
	// Required: true
	Scope string `json:"scope"`
	// The URL of the token endpoint.
	// Required: true
	AuthUrl string `json:"auth_url"`
	// The API URL for this token.
	// Required: true
	ApiUrl string `json:"api_url"`
}

func (gtp *GetTokenParams) GetClientId() string {
	return gtp.ClientId
}

func (gtp *GetTokenParams) GetClientSecret() string {
	return gtp.ClientSecret
}

func (gtp *GetTokenParams) GetScope() string {
	return gtp.Scope
}

func (gtp *GetTokenParams) GetAuthUrl() string {
	return gtp.AuthUrl
}

func (gtp *GetTokenParams) GetApiUrl() string {
	return gtp.ApiUrl
}

func (gtp *GetTokenParams) IsValid() error {
	if !strings.HasSuffix(gtp.AuthUrl, "oauth2/token") {
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

type authenticatedContext struct {
	context.Context
}

func (ac *authenticatedContext) GetTokenParams() TokenParams {
	return ac.Value(TokenParamsContextKey).(*GetTokenParams)
}

func (ac *authenticatedContext) GetToken() (*Token, bool) {
	token, ok := ac.Value(TokenContextKey).(*Token)
	return token, ok
}

func (ac *authenticatedContext) SetToken(token *Token) {
	ac.Context = context.WithValue(ac.Context, TokenContextKey, token)
}

// GetAuthenticatedContext returns an AuthenticatedContext embedded with an access token.
func GetAuthenticatedContext(ctx context.Context, params TokenParams) (AuthenticatedContext, error) {
	if invalidErr := params.IsValid(); invalidErr != nil {
		return nil, invalidErr
	}
	if ctx == nil {
		return nil, fmt.Errorf("You must provide a context to GetAuthenticatedContext")
	}
	token, err := GetToken(ctx, params, nil)
	if err != nil {
		return nil, err
	}
	authedContext := &authenticatedContext{context.WithValue(ctx, TokenParamsContextKey, params)}
	authedContext.SetToken(token)
	return authedContext, nil
}

// GetToken retrieves a fresh access token from the API.
func GetToken(ctx context.Context, params TokenParams, client *http.Client) (*Token, error) {
	if err := params.IsValid(); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(params.GetClientId())
	sb.WriteByte(':')
	sb.WriteString(params.GetClientSecret())

	encodedAuthUrl := base64.StdEncoding.EncodeToString([]byte(sb.String()))

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", params.GetScope())
	data.Set("client_id", params.GetClientId())

	req, err := http.NewRequest(http.MethodPost, params.GetAuthUrl(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	req.Header.Add("Authorization", "Basic "+encodedAuthUrl)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "*/*")

	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Received non-OK status")
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	var result oauth2Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	expirationTime := time.Unix(time.Now().Unix()+(result.ExpiresIn-expiryTimeSkew), 0)

	return &Token{
		AccessToken: result.AccessToken,
		ExpiresAt:   expirationTime,
	}, nil
}
