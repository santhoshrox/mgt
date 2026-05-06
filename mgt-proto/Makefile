# Regenerates Go gRPC stubs from proto/mgt/v1/mgt.proto into gen/mgt/v1.
# Requires `protoc` (>= 3.21), `protoc-gen-go`, and `protoc-gen-go-grpc` on
# PATH. Install once with:
#   brew install protobuf
#   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
#   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

PROTO_FILES := $(shell find proto -name '*.proto')

.PHONY: gen
gen:
	protoc \
		--proto_path=proto \
		--go_out=gen --go_opt=paths=source_relative \
		--go-grpc_out=gen --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)
	go mod tidy

.PHONY: clean
clean:
	rm -rf gen/mgt
