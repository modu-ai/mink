.PHONY: proto-generate proto-lint proto-breaking test test-race vet

# buf 바이너리가 PATH에 없어도 go run으로 실행한다.
BUF := go run github.com/bufbuild/buf/cmd/buf

# PATH에 protoc-gen-go, protoc-gen-go-grpc 바이너리가 필요하다.
# make proto-generate 실행 전 아래 명령으로 설치:
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
proto-generate:
	$(BUF) generate

proto-lint:
	$(BUF) lint

proto-breaking:
	$(BUF) breaking --against '.git#branch=main'

test:
	go test ./...

test-race:
	go test -race ./...

vet:
	go vet ./...
