export GO111MODULE=on

VERSION=$(shell date '+%Y%m%d')
BUILD=$(shell git rev-parse HEAD)
BASEDIR=./dist
DIR=${BASEDIR}/temp

LDFLAGS=-ldflags "-s -w -X main.build=${BUILD} -X github.com/poerhiza/fsort/BuildVersion=${VERSION} -buildid=${BUILD}"
GCFLAGS=-gcflags=all=-trimpath=$(shell echo ${HOME})
ASMFLAGS=-asmflags=all=-trimpath=$(shell echo ${HOME})

GOFILES=`go list ./...`
GOFILESNOTEST=`go list ./... | grep -v test`

# Make Directory to store executables
$(shell mkdir -p ${DIR})

all: lint freebsd linux windows

freebsd: lint
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/fsort_freebsd_amd64

linux: lint
	env CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/fsort_linux_amd64

windows: lint
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath ${LDFLAGS} ${GCFLAGS} ${ASMFLAGS} -o ${DIR}/fsort_windows_amd64

tidy:
	@go mod tidy

dep: tidy ## Get the dependencies
	@go get -u github.com/goreleaser/goreleaser
	@go get -v -d ./...
	@go get -u all

lint: ## Lint the files
	@go fmt ${GOFILES}
	@go vet ${GOFILESNOTEST}

sanity:
	goreleaser release --config .goreleaser.yml --rm-dist --skip-validate  --skip-publish --snapshot

release:
	goreleaser release --config .goreleaser.yml

clean:
	rm -rf ${BASEDIR}

.PHONY: all freebsd linux windows tidy dep lint sanity release clean
