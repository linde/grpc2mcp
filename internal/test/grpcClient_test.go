package test

// func TestGrpcClient(t *testing.T) {

// 	assert := assert.New(t)
// 	assert.NotNil(assert)

// 	grpcServerAddr := ":8080"

// 	conn, err := grpc.NewClient(grpcServerAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
// 	assert.NoError(err)
// 	defer conn.Close()

// 	mcpGrpcClient := pb.NewModelContextProtocolClient(conn)
// 	sessionCtx, err := doInitialize(context.Background(), mcpGrpcClient)
// 	assert.NoError(err)

// 	listToolsResult, err := mcpGrpcClient.ListTools(sessionCtx, &pb.ListToolsRequest{})
// 	assert.NoError(err)

// 	log.Printf("listToolsResult: %v", listToolsResult)

// }
