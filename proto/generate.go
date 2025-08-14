//go:generate protoc --proto_path=. --go_out=../pb --go_opt=paths=source_relative --go-grpc_out=../pb --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false mcp.proto

package proto
