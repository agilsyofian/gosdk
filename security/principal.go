package security

// Principal represents an authenticated identity as established at the
// moment of authentication.
//
// When stateful session is enabled (the recommended default for this
// SDK), the Session -- not the Principal derived directly from a raw
// credential such as a JWT -- is the authoritative source of user
// context (roles, permissions, device info, etc). A Principal is still
// attached to the Session when it is created (see SessionManager.Create),
// and it is that Principal, retrieved back out of the session, which
// application code should trust.
type Principal struct {
	// Subject is the stable identifier of the authenticated entity,
	// e.g. a user ID.
	Subject string

	// Username is a human-readable identifier, if applicable.
	Username string

	// Roles lists the roles assigned to the principal.
	Roles []string

	// Permissions lists the fine-grained permissions assigned to the
	// principal, e.g. "transaction:transfer".
	Permissions []string

	// Claims holds any additional authentication-related data. This
	// should not be used as a dumping ground for business/session
	// attributes -- see Session.Attributes for that.
	Claims map[string]any
}

// HasRole reports whether the principal has the given role.
func (p *Principal) HasRole(role string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission reports whether the principal has the given
// permission.
func (p *Principal) HasPermission(permission string) bool {
	if p == nil {
		return false
	}
	for _, perm := range p.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// Claim retrieves an additional authentication claim by name. It returns
// false when the principal, its claims map, or the requested claim is absent.
func (p *Principal) Claim(name string) (any, bool) {
	if p == nil || p.Claims == nil {
		return nil, false
	}
	value, ok := p.Claims[name]
	return value, ok
}
