// Package grpc Package handler Package grpc Package demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=domain.User:*user.Response -converter-pkg=../../../../app/shared/converters
package grpchandler

import (
	_ "user-service/modules/user/domain"
	_ "user-service/proto/user"
)
