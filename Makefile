.PHONY: build install test test-dep test-gomodfuzz test-core

pkg-path-base = github.com/codeactual/aws-mockery

get-git-ref-sha = $(shell git rev-parse --short HEAD)
get-git-ref-label = $(shell git symbolic-ref -q --short HEAD || git describe --tags --exact-match)
get-git-dirty = $(shell git diff-index --quiet HEAD; echo $$? | sed 's/1/-dirty/' | sed 's/0//')

define go-version-ldflags
	$(eval git-ref-sha := $(call get-git-ref-sha))
	$(eval git-ref-label := $(call get-git-ref-label))
	$(eval git-dirty := $(call get-git-dirty))
	$(eval tmpl := -X ${pkg-path-base}/internal/ldflags.Version=$(word 3, $(strip $(shell go version)))-${git-ref-sha}-${git-ref-label}${git-dirty})
	$(eval version-ldflags := $(call tmpl))
endef

build:
	@mkdir -p ./build
	$(call go-version-ldflags)
	$(eval bin-name := $(shell basename ${pkg-path-base}))
	@GO111MODULE=on go build -ldflags "${version-ldflags}" -v -o ./build/${bin-name} ./cmd/${bin-name}
	@ls -la ./build/
	@./build/${bin-name} --version

install:
	$(call go-version-ldflags)
	$(eval bin-name := $(shell basename ${pkg-path-base}))
	@GO111MODULE=on go install -ldflags "${version-ldflags}" -v ./cmd/${bin-name}
	@ls -la `which ${bin-name}`
	@${bin-name} --version

# - Run ./internal/cage test suites
test-core: build
	@mkdir -p ./testdata/cover
	@CGO_ENABLED=1 go test -race -coverprofile=./testdata/cover/cover.out ./internal/cage/...
	@go tool cover -func=./testdata/cover/cover.out 2>&1 | tee ./testdata/cover/cover.tmp
	@head -n -1 ./testdata/cover/cover.tmp | sed 's/:[0-9]\+://g' | sort > ./testdata/cover/index.txt
	@tail -n 1 ./testdata/cover/cover.tmp | sed 's/^[^0-9]\+//' >> ./testdata/cover/index.txt
	@rm ./testdata/cover/cover.tmp ./testdata/cover/cover.out

test-gomodfuzz:
	@./testdata/bin/integration

test-dep:
	@go get -v github.com/codeactual/gomodfuzz/cmd/gomodfuzz

test: test-core test-gomodfuzz
