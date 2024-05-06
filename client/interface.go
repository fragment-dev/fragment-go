package client

import "time"

type Clock interface {
	Now() time.Time
}
