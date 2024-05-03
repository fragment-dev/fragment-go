package client

import (
	"net/http"
	"time"

	"github.com/Khan/genqlient/graphql"
	"github.com/fragment-dev/fragment-go/auth"
)

type HttpClient struct {
	*http.Client

	auth.AuthenticatedContext
	clock Clock
}

type realClock struct{}

func (realClock) Now() time.Time {
	return time.Now()
}

func getClock() Clock {
	return &realClock{}
}

func newHttpClient(ctx auth.AuthenticatedContext, clock Clock) *HttpClient {
	if clock == nil {
		clock = getClock()
	}
	return &HttpClient{
		Client:               &http.Client{},
		AuthenticatedContext: ctx,
		clock:                clock,
	}
}

func (c *HttpClient) Do(req *http.Request) (*http.Response, error) {
	token, _ := c.AuthenticatedContext.GetToken()
	// If the token has expired, get a new one.
	if c.clock.Now().After(token.ExpiresAt) {
		token, err := auth.GetToken(
			c.AuthenticatedContext,
			c.AuthenticatedContext.GetTokenParams(),
			nil)
		if err != nil {
			return nil, err
		}

		c.AuthenticatedContext.SetToken(token)
	}
	// Issue the request within the authenticated context, if a context hasn't been set.
	if req.Context() == nil {
		req = req.WithContext(c.AuthenticatedContext)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("X-Fragment-Client", "go-client")
	return c.Client.Do(req)
}

// NewClient creates a new GraphQL client with the provided authenticated context.
func NewClient(ctx auth.AuthenticatedContext) (graphql.Client, error) {
	tokenParams := ctx.GetTokenParams()
	return graphql.NewClient(tokenParams.GetApiUrl(), newHttpClient(ctx, nil)), nil
}
