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
}

func newHttpClient(ctx auth.AuthenticatedContext) *HttpClient {
	return &HttpClient{
		Client:               &http.Client{},
		AuthenticatedContext: ctx,
	}
}

func (c *HttpClient) Do(req *http.Request) (*http.Response, error) {
	token, _ := c.AuthenticatedContext.GetToken()
	// If the token has expired, get a new one.
	if time.Now().After(token.ExpiresAt) {
		token, err := auth.GetToken(
			c.AuthenticatedContext,
			*c.AuthenticatedContext.GetTokenParams(),
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
	_, ok := ctx.GetToken()
	if !ok {
		token, err := auth.GetToken(ctx, *tokenParams, nil)
		if err != nil {
			return nil, err
		}
		ctx.SetToken(token)
	}
	return graphql.NewClient(tokenParams.ApiUrl, newHttpClient(ctx)), nil
}
