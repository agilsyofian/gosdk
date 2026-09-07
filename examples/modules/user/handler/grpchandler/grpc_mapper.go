// Package grpc Package handler Package grpc Package demonstrates mapgen: one go:generate directive, blank
// imports to make the type selectors resolvable, nothing else.
//
//go:generate go tool mapgen -types=model.User:*user.Response -converter-pkg=../../../../app/shared/converters
package grpchandler

import (
	_ "github.com/agilsyofian/gosdk/example/modules/user/domain"
	_ "github.com/agilsyofian/gosdk/example/proto/user"
)
