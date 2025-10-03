package client

import "time"

type Clock interface {
	Now() time.Time
}
type TokenParams interface {
	GetClientID() string
	getClientSecret() string
	GetScope() string
	GetAuthURL() string
	GetApiURL() string

	IsValid() error
}
