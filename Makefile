.PHONY: all generate lint install-tools

all: generate lint

# Tool versions
BUF_VERSION := v1.50.0
PROTOC_GEN_GO_VERSION := v1.36.11
PROTOC_GEN_GO_GRPC_VERSION := v1.5.1

# Generate Go code from proto files
generate:
	@echo "Generating Go code from proto files..."
	@buf generate

# Lint proto files
lint:
	@echo "Linting proto files..."
	@buf lint

# Install required tools
install-tools:
	@echo "Installing gRPC and buf tools..."
	@go install github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION)
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@$(PROTOC_GEN_GO_VERSION)
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@$(PROTOC_GEN_GO_GRPC_VERSION)

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	@find pkg/api -name "*.pb.go" -delete
