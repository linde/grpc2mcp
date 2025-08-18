package test

import (
	"context"
	"grpc2mcp/pb"
	"log"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestE2e(t *testing.T) {

	assert := assert.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	sessionCtx, err := doInitialize(ctx, mcpGrpcClient)
	assert.NoError(err)
	if err != nil {
		return
	}

	PingResult, err := mcpGrpcClient.Ping(sessionCtx, &pb.PingRequest{})
	assert.NoError(err)
	log.Printf("ping: %v", PingResult)

	listToolsResult, err := mcpGrpcClient.ListTools(sessionCtx, &pb.ListToolsRequest{})
	assert.NoError(err)

	// this tests our ListTools rpc making sure that our target tools are present
	// TODO make the trivy server export these somehow, even statically
targetToolsLoop:
	for _, target := range []string{"add", "mult", "lower"} {
		for _, tool := range listToolsResult.Tools {
			if tool.Name == target {
				log.Printf("ListTools: found target %s", target)
				continue targetToolsLoop
			}
		}
		assert.Failf("ListTools", "missing tool %s", target)
	}

}

func TestToolCall(t *testing.T) {

	assert := assert.New(t)

	conn, closeFunc, err := SetupMcpAndProxyAsync(t.Name())
	assert.NoError(err)
	defer conn.Close()
	defer closeFunc()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	ctx := context.Background()

	sessionCtx, err := doInitialize(ctx, mcpGrpcClient)
	assert.NoError(err)
	if err != nil {
		return
	}
	assert.NotNil(sessionCtx)

	callToolRequest := &pb.CallToolRequest{
		Name: "add",
		Arguments: map[string]*structpb.Value{
			"a": structpb.NewNumberValue(10),
			"b": structpb.NewNumberValue(1),
		},
	}
	callMethodResult, err := mcpGrpcClient.CallMethod(sessionCtx, callToolRequest)
	assert.NoError(err)
	assert.NotNil(callMethodResult)
	result := callMethodResult.GetStructuredContent().GetFields()["result"].GetNumberValue()
	assert.Equal(float64(11), result)

}

// this runs a blocking example trivy mcp server; it's commented out to prevent runs during tests
// func TestTrivyServer(t *testing.T) {

// 	assert := assert.New(t)

// 	handler := RunTrivyServer(t.Name())
// 	mcpListener, trivyServerCancelFunc, err := RunServerAsync(handler)
// 	assert.NoError(err)
// 	defer trivyServerCancelFunc()

// 	mcpTcpAddr, _ := mcpListener.Addr().(*net.TCPAddr)
// 	log.Printf("mcp handler listening on: %s", mcpTcpAddr)

// 	select {}

// }
