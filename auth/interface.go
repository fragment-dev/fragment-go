package auth

import "context"

type TokenParams interface {
	GetClientId() string
	GetClientSecret() string
	GetScope() string
	GetAuthUrl() string
	GetApiUrl() string

	IsValid() error
}

// AuthenticatedContext defines the interface for a context embedded
// with the Fragment API access token.
type AuthenticatedContext interface {
	context.Context

	// GetTokenParams returns the token parameters used to authenticate
	// with the Fragment API.
	GetTokenParams() TokenParams

	// SetToken sets the access token in the context.
	SetToken(*Token)

	// GetToken returns the access token from the context.
	GetToken() (*Token, bool)
}
