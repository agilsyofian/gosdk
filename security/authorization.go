package security

import (
	"context"
)

// Authorizer decides whether the identity carried by a SecurityContext
// is allowed to perform an action identified by permission, e.g.
// "transaction:transfer".
//
// Authentication and authorization are kept as separate concerns:
// Authenticate/AuthenticateSession answer "who are you?"; Authorize
// answers "are you allowed to do this?". Implementations live in the
// authorization subpackage (or may be supplied by the caller) and
// must not depend on transport packages.
type Authorizer interface {
	Authorize(ctx context.Context, securityContext *SecurityContext, permission string) error
}
