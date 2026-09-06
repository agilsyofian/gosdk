module github.com/agilsyofian/gosdk/security/session/redis

go 1.26.8

replace (
	github.com/agilsyofian/gosdk/core/errors => ./../../../core/errors
	github.com/agilsyofian/gosdk/security => ./../..
	github.com/agilsyofian/gosdk/storage/cache => ./../../../storage/cache
)

require (
	github.com/agilsyofian/gosdk/core/errors v0.0.1
	github.com/agilsyofian/gosdk/security v0.0.1
	github.com/agilsyofian/gosdk/storage/cache v0.0.1
)
