VERSION=$(shell bin/authnds64 --version)

GIT_COMMIT=$(shell git rev-list -1 HEAD )
BUILD_TIME=$(shell date --utc +%Y%m%d_%H%M%SZ)
GIT_CLEAN=$(shell git status | grep -E "working (tree|directory) clean" | wc -l)

# Last git tag
LAST_GIT_TAG=$(shell git describe --abbrev=0 --tags 2> /dev/null)

# this=1 if the current commit is the tagged commit (ie, if this is a release build)
GIT_IS_TAG_COMMIT=$(shell git describe --abbrev=0 --tags > /dev/null 2> /dev/null && echo "1" || echo "0")

# Used when a tag isn't available
GIT_BRANCH=$(shell git rev-parse --abbrev-ref HEAD)

# Build variables
BUILD_VARS=-X main.GitCommit=${GIT_COMMIT} -X main.GitBranch=${GIT_BRANCH} -X main.BuildTime=${BUILD_TIME} -X main.GitClean=${GIT_CLEAN} -X main.LastGitTag=${LAST_GIT_TAG} -X main.GitTagIsCommit=${GIT_IS_TAG_COMMIT}
BUILD_FILES=authnds.go config.go configbackend.go configbackend_helpers.go password.go version.go

#####################
# High level commands
#####################

# Build and run - used for development
run: setup devrun cleanup

# Run the integration test on linux64 (eventually allow the binary to be set)
test: runtest

# Run build process for all binaries
all: setup binaries verify cleanup

# Run build process for only linux64
fast: setup linux64 verify cleanup

# list of binary formats to build
binaries: linux32 linux64 linuxarm32 linuxarm64 darwin64 win32 win64

# Setup commands to always run
setup: getdeps bindata format

#####################
# Subcommands
#####################

# Run integration test
runtest:
	./scripts/travis/integration-test.sh cleanup

# Get all dependencies
getdeps:
	go get -d ./...

updatetest:
	./scripts/travis/integration-test.sh

bindata:
	go get -u github.com/jteeuwen/go-bindata/... && ${GOPATH}/bin/go-bindata -pkg=main assets && gofmt -w bindata.go


cleanup:
	rm bindata.go

format:
	go fmt

devrun:
	go run ${BUILD_FILES} -c config.toml


linux32:
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authnds32 ${BUILD_FILES} && cd bin && sha256sum authnds32 > authnds32.sha256

linux64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authnds64 ${BUILD_FILES} && cd bin && sha256sum authnds64 > authnds64.sha256

linuxarm32:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authnds-arm32 ${BUILD_FILES} && cd bin && sha256sum authnds-arm32 > authnds-arm32.sha256

linuxarm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authnds-arm64 ${BUILD_FILES} && cd bin && sha256sum authnds-arm64 > authnds-arm64.sha256

darwin64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authndsOSX ${BUILD_FILES} && cd bin && sha256sum authndsOSX > authndsOSX.sha256

win32:
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authnds-win32 ${BUILD_FILES} && cd bin && sha256sum authnds-win32 > authnds-win32.sha256

win64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -a -installsuffix cgo -ldflags "${BUILD_VARS}" -o bin/authnds-win64 ${BUILD_FILES} && cd bin && sha256sum authnds-win64 > authnds-win64.sha256


verify:
	cd bin && sha256sum *.sha256 -c && cd ../;
