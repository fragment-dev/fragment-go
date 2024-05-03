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

	GetTokenParams() TokenParams

	SetToken(*Token)
	GetToken() (*Token, bool)
}
