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
	if time.Now().After(token.ExpiresAt) {
		// TODO: Implement token refresh
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
