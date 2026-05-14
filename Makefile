.PHONY: proto-generate proto-lint proto-breaking test test-race vet brand-lint fmt lint ci-local

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

# brand-lint: AI.GOOSE 브랜드 표기 규범 검증 (SPEC-GOOSE-BRAND-RENAME-001)
# 위반 발견 시 exit 1, 위반 없으면 exit 0
brand-lint:
	bash scripts/check-brand.sh

# fmt: gofmt 위반 파일 출력 + 위반 있으면 exit 1 (auto-fix 하지 않음)
fmt:
	@out="$$(gofmt -l . 2>/dev/null)"; \
	if [ -n "$$out" ]; then \
		echo "gofmt diffs detected. Run 'gofmt -w .' to fix:" >&2; \
		echo "$$out" >&2; \
		exit 1; \
	fi

# lint: go vet (golangci-lint 도입 시 여기로 확장)
lint: vet

# ci-local: pre-push hook 이 호출하는 로컬 CI mirror. GitHub Actions 의
# Go (build / vet / gofmt / test -race) + Brand Notation Check 와 같은 게이트.
# 실패 시 hook 이 push 를 차단한다.
ci-local: fmt vet test-race brand-lint
	@echo "[ci-local] all gates passed"
