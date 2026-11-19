// Package httphandler Package handler Package http Package demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=domain.User:UserDetail -converter-pkg=../../../../app/shared/converters -ignore domain.User.PasswordHash,domain.User.BiometricHash
package httphandler

import (
	_ "user-service/modules/user/domain"
)
