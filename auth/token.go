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

	TokenParamsContextKey = "tokenParams"
	TokenContextKey       = "token"
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

type getTokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

type AuthenticatedContext = context.Context

func GetAuthenticatedContext(ctx context.Context, params *GetTokenParams) (AuthenticatedContext, error) {
	if invalidErr := params.IsValid(); invalidErr != nil {
		return nil, invalidErr
	}
	if ctx == nil {
		return nil, fmt.Errorf("You must provide a context to GetAuthenticatedContext")
	}
	return context.WithValue(ctx, TokenParamsContextKey, params), nil
}

func GetAuthenticatedContextWithToken(ctx context.Context, params *GetTokenParams, token *Token) (AuthenticatedContext, error) {
	authenticatedContext, err := GetAuthenticatedContext(ctx, params)
	if err != nil {
		return nil, err
	}
	return context.WithValue(authenticatedContext, TokenContextKey, token), nil
}

func GetToken(ctx context.Context, params GetTokenParams) (*Token, error) {
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

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	var result getTokenResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	expirationTime := time.Unix(time.Now().Unix()+(result.ExpiresIn-expiryTimeSkew), 0)

	return &Token{
		AccessToken: result.AccessToken,
		ExpiresAt:   expirationTime,
	}, nil
}
