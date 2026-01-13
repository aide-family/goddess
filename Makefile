GOHOSTOS:=$(shell go env GOHOSTOS)
VERSION=$(shell git describe --tags --always)
BUILD_TIME=$(shell date '+%Y-%m-%dT%H:%M:%SZ')
AUTHOR=$(shell git log -1 --format='%an')
AUTHOR_EMAIL=$(shell git log -1 --format='%ae')
REPO=$(shell git config remote.origin.url)

ifeq ($(GOHOSTOS), windows)
	#the `find.exe` is different from `find` in bash/shell.
	#to see https://docs.microsoft.com/en-us/windows-server/administration/windows-commands/find.
	#changed to use git-bash.exe to run find cli or other cli friendly, caused of every developer has a Git.
	Git_Bash=$(subst \,/,$(subst cmd\,bin\bash.exe,$(dir $(shell where git))))
	PROTO_FILES=$(shell $(Git_Bash) -c "find proto/goddess -name *.proto")
	# Use mkdir -p equivalent for Windows
	MKDIR=mkdir
	RM=del /f /q
else
	PROTO_FILES=$(shell find proto/goddess -name *.proto)
	MKDIR=mkdir -p
	RM=rm -f
endif

.PHONY: init
# initialize the goddess environment
init:
	@echo "Initializing goddess environment"
	go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.36.3
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.5.1
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-http/v2@latest
	go install github.com/go-kratos/kratos/cmd/protoc-gen-go-errors/v2@latest
	go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest
	go install github.com/google/wire/cmd/wire@latest
	go install github.com/aide-family/stringer@v1.1.3
	go install github.com/protoc-gen/i18n-gen@latest
	go install golang.org/x/tools/gopls@latest
	go install github.com/go-kratos/kratos/cmd/kratos/v2@latest

.PHONY: api
# generate the api files
api:
	@echo "Generating api files"
	@if [ "$(GOHOSTOS)" = "windows" ]; then \
		$(Git_Bash) -c "rm -rf ./pkg/*.pb.go"; \
		if [ ! -d "./pkg" ]; then $(MKDIR) ./pkg; fi \
	else \
		rm -rf ./pkg/*.pb.go; \
		if [ ! -d "./pkg" ]; then $(MKDIR) ./pkg; fi \
	fi
	protoc --proto_path=./proto/goddess \
	       --proto_path=./proto/third_party \
 	       --go_out=paths=source_relative:./pkg \
 	       --go-http_out=paths=source_relative:./pkg \
 	       --go-grpc_out=paths=source_relative:./pkg \
	       --openapi_out=fq_schema_naming=true,default_response=false:./internal/server/swagger \
	       --experimental_allow_proto3_optional \
	       $(PROTO_FILES)

.PHONY: wire
# generate the wire files
wire:
	@echo "Generating wire files"
	wire ./...

.PHONY: vobj
# generate the vobj files
vobj:
	@echo "Generating vobj files"
	cd pkg/vobj && go generate .

.PHONY: gorm-gen
# generate the gorm files
gorm-gen:
	@echo "Generating gorm files"
	go run ./cmd/gorm gorm gen

.PHONY: gorm-migrate
# migrate the gorm files
gorm-migrate:
	@echo "Migrating gorm files"
	go run ./cmd/gorm gorm migrate