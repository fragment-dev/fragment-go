package auth

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	expiryTimeSkew int64 = 120

	tokenParamsContextKey = "tokenParams"
	tokenContextKey       = "token"
)

var (
	ErrTokenParamsNotFound = errors.New("Token params not found in the provided context.")
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

type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

// AuthenticatedContext defines the interface for a context embedded
// with the Fragment API access token.
type AuthenticatedContext interface {
	context.Context

	GetTokenParams() *GetTokenParams

	SetToken(*Token)
	GetToken() (*Token, bool)
}

type authenticatedContext struct {
	context.Context
}

func (ac *authenticatedContext) GetTokenParams() *GetTokenParams {
	return ac.Value(tokenParamsContextKey).(*GetTokenParams)
}

func (ac *authenticatedContext) GetToken() (*Token, bool) {
	token, ok := ac.Value(tokenContextKey).(*Token)
	return token, ok
}

func (ac *authenticatedContext) SetToken(token *Token) {
	ac.Context = context.WithValue(ac.Context, tokenContextKey, token)
}

// GetAuthenticatedContext returns an AuthenticatedContext embedded with an access token.
func GetAuthenticatedContext(ctx context.Context, params *GetTokenParams) (AuthenticatedContext, error) {
	if invalidErr := params.IsValid(); invalidErr != nil {
		return nil, invalidErr
	}
	if ctx == nil {
		return nil, fmt.Errorf("You must provide a context to GetAuthenticatedContext")
	}
	token, err := GetToken(ctx, *params, nil)
	if err != nil {
		return nil, err
	}
	authedContext := &authenticatedContext{context.WithValue(ctx, tokenParamsContextKey, params)}
	authedContext.SetToken(token)
	return authedContext, nil
}

// GetToken retrieves a fresh access token from the API.
func GetToken(ctx context.Context, params GetTokenParams, client *http.Client) (*Token, error) {
	if !strings.HasSuffix(params.AuthUrl, "oauth2/token") {
		return nil, fmt.Errorf("The AuthUrl passed must end in /oauth2/token")
	}

	var sb strings.Builder
	sb.WriteString(params.ClientId)
	sb.WriteByte(':')
	sb.WriteString(params.ClientSecret)

	encodedAuthUrl := base64.StdEncoding.EncodeToString([]byte(sb.String()))

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", params.Scope)
	data.Set("client_id", params.ClientId)

	req, err := http.NewRequest(http.MethodPost, params.AuthUrl, strings.NewReader(data.Encode()))
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
