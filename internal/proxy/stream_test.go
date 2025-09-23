package proxy

import (
	"context"
	"grpc2mcp/internal/examplemcp"
	"grpc2mcp/pb"
	"io"
	"log"
	"net"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestStream(t *testing.T) {

	handler := examplemcp.RunExampleMcpServer(t.Name(), "/mcp")

	ts := httptest.NewServer(handler)
	defer ts.Close()

	log.Printf("mcp handler listening on: %s", ts.URL)
	s, err := NewServer(ts.URL)
	require.NoError(t, err)

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	serverCancel, err := s.StartProxyToListenerAsync(lis)
	require.NoError(t, err)
	defer serverCancel()

	bufDialer := func(context.Context, string) (net.Conn, error) { return lis.Dial() }
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(bufDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)

	defer conn.Close()

	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
	require.NotNil(t, mcpGrpcClient)

	// doGrpcProxyStreamTests(t, mcpGrpcClient)

}

// TODO these should be done in the e2e as well, not just bufcon
func doGrpcProxyStreamTests(t *testing.T, mcpGrpcClient pb.ModelContextProtocolClient) {

	sessionCtx, err := doProxyInitialize(t.Context(), mcpGrpcClient)
	require.NoErrorf(t, err, "error with doMcpInitialize")

	args := map[string]any{examplemcp.PARAM_A: 1, examplemcp.PARAM_B: 10}

	argsStruct, err := structpb.NewStruct(args)
	require.NoErrorf(t, err, "error unpacking args")

	// first do the send part, stream the contents of an array to the server

	callToolRequests := []*pb.CallToolRequest{
		{Name: examplemcp.TOOL_ADD, Arguments: argsStruct.GetFields()},
		{Name: examplemcp.TOOL_ADD, Arguments: argsStruct.GetFields()},
		{Name: examplemcp.TOOL_ADD, Arguments: argsStruct.GetFields()},
	}

	stream, err := mcpGrpcClient.CallMethodStream(sessionCtx)
	require.NoErrorf(t, err, "error with CallMethodStream")

	for sendCount, callToolReq := range callToolRequests {
		err = stream.Send(callToolReq)
		if err == io.EOF {
			break
		}
		require.NoErrorf(t, err, "error on stream.Send number %d", sendCount)
	}

	stream.CloseSend()

	// now walk through the responses

	for {
		callMethodResult, err := stream.Recv()
		if err == io.EOF {
			break
		}
		require.NoErrorf(t, err, "error on stream.Recv")

		if callMethodResult.GetIsError() {
			require.Fail(t, "callMethodResult was error but didnt expect it")
		}

		log.Printf("got: %v", callMethodResult)
	}

}
