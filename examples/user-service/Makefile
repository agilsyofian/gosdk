proto:
	cd proto && protoc --go_out=./ --go_opt=paths=source_relative \
        --go-grpc_out=./ --go-grpc_opt=paths=source_relative \
        ./**/*.proto
swagger:
	go get -u github.com/swaggo/gin-swagger
	go get -u github.com/swaggo/files
	swag init --parseDependency true

.PHONY: proto