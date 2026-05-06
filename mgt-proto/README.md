# mgt-proto

Shared Protobuf / gRPC schema for the mgt platform. The CLI ([`mgt-cli`](../mgt-cli)) consumes this as a gRPC client; the backend ([`mgt-be`](../mgt-be)) implements it.

## Layout

```
proto/mgt/v1/mgt.proto    # the schema
gen/mgt/v1/mgt.pb.go      # generated (do not edit)
gen/mgt/v1/mgt_grpc.pb.go # generated (do not edit)
```

Both `mgt-be` and `mgt-cli` depend on this module via a local `replace` directive in their `go.mod`, so changes to the proto schema flow through immediately after re-running codegen.

## Regenerating stubs

```bash
brew install protobuf
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

cd mgt-proto
make gen
```

`make gen` runs `protoc` and then `go mod tidy` so the generated files always link against the right runtime versions.

## Adding a method

1. Edit `proto/mgt/v1/mgt.proto`. Add the rpc + request/response messages.
2. Run `make gen`.
3. Implement the handler in `mgt-be/internal/grpcserver/`.
4. Wire a typed wrapper in `mgt-cli/pkg/client/client.go`.
5. Rebuild both modules: `(cd ../mgt-be && go build ./...) && (cd ../mgt-cli && go build ./...)`.
