package security

import (
	"github.com/agilsyofian/gosdk/security"
)

// Middleware is the Gin security adapter. It is responsible for:
//
//  1. Extracting credentials from the HTTP request
//  2. Calling the SecurityManager
//  3. Creating the SecurityContext
//  4. Injecting it into the request's context.Context
//  5. Handling authentication errors
//  6. Optionally enforcing permissions
//
// It does not implement authentication logic itself -- see the
// package doc comment.
type Middleware struct {
	manager security.SecurityManager
	config  Config
}

// New constructs a Gin security Middleware bound to the given
// SecurityManager and Config.
func New(manager security.SecurityManager, config Config) *Middleware {
	return &Middleware{manager: manager, config: config}
}
