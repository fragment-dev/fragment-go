package client

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

	"github.com/Khan/genqlient/graphql"
)

type HttpClient struct {
	*http.Client
	tokenParams TokenParams
	token       *Token
	clock       Clock
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func getClock() Clock {
	return &realClock{}
}

func newHttpClient(clock Clock, tokenParams TokenParams) *HttpClient {
	if clock == nil {
		clock = getClock()
	}
	return &HttpClient{
		Client:      &http.Client{}, // TODO can i unexport this and token params since we're using a getter pattern?
		clock:       clock,
		tokenParams: tokenParams,
	}
}

func (c *HttpClient) GetTokenParams() TokenParams {
	return c.tokenParams.(*GetTokenParams)
}

func (c *HttpClient) GetToken() *Token {
	return c.token
}

func (c *HttpClient) SetToken(token *Token) {
	c.token = token
}

// GetToken retrieves a fresh access token from the API.
func (c *HttpClient) GenerateToken(ctx context.Context) (*Token, error) {
	if err := c.tokenParams.IsValid(); err != nil {
		return nil, err
	}

	var sb strings.Builder
	sb.WriteString(c.tokenParams.GetClientID())
	sb.WriteByte(':')
	sb.WriteString(c.tokenParams.getClientSecret())

	encodedAuthURL := base64.StdEncoding.EncodeToString([]byte(sb.String()))

	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("scope", c.tokenParams.GetScope())
	data.Set("client_id", c.tokenParams.GetClientID())

	req, err := http.NewRequest(http.MethodPost, c.tokenParams.GetAuthURL(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	if ctx != nil {
		req = req.WithContext(ctx)
	}

	req.Header.Add("Authorization", "Basic "+encodedAuthURL)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "fragment-dev/fragment-go")

	if c.Client == nil {
		c.Client = &http.Client{}
	}
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		body, bodyErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if bodyErr != nil {
			return nil, fmt.Errorf("received non-OK status: %d %s (failed to read response body: %v)", resp.StatusCode, resp.Status, bodyErr)
		}
		return nil, fmt.Errorf("received non-OK status: %d %s, response body: %s", resp.StatusCode, resp.Status, string(body))
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var result oauth2Response
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	expirationTime := time.Unix(c.clock.Now().Unix()+(result.ExpiresIn-expiryTimeSkew), 0)

	return &Token{
		AccessToken: result.AccessToken,
		ExpiresAt:   expirationTime,
	}, nil
}

func (c *HttpClient) Do(req *http.Request) (*http.Response, error) {
	token := c.GetToken()
	// If we haven't generated a token yet or the token has expired, get a new one.
	if token == nil || c.clock.Now().After(token.ExpiresAt) {
		newToken, err := c.GenerateToken(context.Background())
		if err != nil {
			return nil, err
		}
		c.SetToken(newToken)
		token = newToken
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("X-Fragment-Client", "go-client")
	return c.Client.Do(req)
}

// NewClient creates a new GraphQL client with the provided authenticated context.
func NewClient(tokenParams TokenParams) (graphql.Client, error) {
	return graphql.NewClient(tokenParams.GetApiURL(), newHttpClient(nil, tokenParams)), nil
}
