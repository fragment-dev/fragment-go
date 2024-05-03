package client

import (
	"context"
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
	token := c.AuthenticatedContext.Value(auth.TokenContextKey).(*auth.Token)
	// If the token has expired, get a new one.
	if time.Now().After(token.ExpiresAt) {
		if token, err := auth.GetToken(
			c.AuthenticatedContext,
			*c.AuthenticatedContext.Value(auth.TokenParamsContextKey).(*auth.GetTokenParams)); err != nil {
			return nil, err
		} else {
			c.AuthenticatedContext = context.WithValue(c.AuthenticatedContext, auth.TokenContextKey, token)
		}
	}
	// Issue the request within the authenticated context, if a context hasn't been set.
	if req.Context() == nil {
		req = req.WithContext(c.AuthenticatedContext)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	req.Header.Set("X-Fragment-Client", "go-client")
	return c.Client.Do(req)
}

func NewClient(ctx auth.AuthenticatedContext) (graphql.Client, error) {
	tokenParams, ok := ctx.Value(auth.TokenParamsContextKey).(*auth.GetTokenParams)
	if !ok {
		return nil, auth.ErrTokenParamsNotFound
	}
	_, ok = ctx.Value(auth.TokenContextKey).(*auth.Token)
	if !ok {
		token, err := auth.GetToken(ctx, *tokenParams)
		if err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, auth.TokenContextKey, token)
	}
	return graphql.NewClient(tokenParams.ApiUrl, newHttpClient(ctx)), nil
}
