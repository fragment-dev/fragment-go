test:
	go test -cover ./...

codegen:
	go run main.go \
		--input=queries/queries.graphql \
		--output=queries/queries.go \
		--package=queries

lint:
	go fmt ./...
