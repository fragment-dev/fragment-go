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
	"sync"
	"time"

	"github.com/Khan/genqlient/graphql"
)

// defaultTokenTimeout bounds a token request, including when the caller's
// context has no deadline.
const defaultTokenTimeout = 30 * time.Second

type HttpClient struct {
	client       *http.Client
	tokenParams  TokenParams
	tokenTimeout time.Duration
	clock        Clock

	// mu guards token, and is held for the whole of a refresh so that
	// concurrent requests share one token request rather than each making one.
	mu    sync.Mutex
	token *Token
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
		client:       &http.Client{},
		clock:        clock,
		tokenParams:  tokenParams,
		tokenTimeout: defaultTokenTimeout,
	}
}

func (c *HttpClient) GetTokenParams() TokenParams {
	return c.tokenParams.(*GetTokenParams)
}

func (c *HttpClient) GetToken() *Token {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.token
}

func (c *HttpClient) SetToken(token *Token) {
	c.mu.Lock()
	defer c.mu.Unlock()
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

	if ctx == nil {
		ctx = context.Background()
	}
	timeout := c.tokenTimeout
	if timeout <= 0 {
		timeout = defaultTokenTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenParams.GetAuthURL(), strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}

	req.Header.Add("Authorization", "Basic "+encodedAuthURL)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("User-Agent", "fragment-dev/fragment-go")

	if c.client == nil {
		c.client = &http.Client{}
	}
	resp, err := c.client.Do(req)
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
	token, err := c.validToken(req.Context())
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("X-Fragment-Client", "go-client")
	return c.client.Do(req)
}

// validToken returns the cached token, generating a new one if we haven't
// generated a token yet or the token has expired.
func (c *HttpClient) validToken(ctx context.Context) (*Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != nil && !c.clock.Now().After(c.token.ExpiresAt) {
		return c.token, nil
	}

	token, err := c.GenerateToken(ctx)
	if err != nil {
		return nil, err
	}
	c.token = token
	return token, nil
}

// NewClient creates a new GraphQL client with the provided authenticated context.
func NewClient(tokenParams TokenParams) (graphql.Client, error) {
	return graphql.NewClient(tokenParams.GetApiURL(), newHttpClient(nil, tokenParams)), nil
}
