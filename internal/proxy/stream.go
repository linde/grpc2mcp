package proxy

import (
	"io"

	mcp "grpc2mcp/pb"
)

// CallMethodStream

func (s *Server) CallMethodStream(stream mcp.ModelContextProtocol_CallMethodStreamServer) error {
	ctx := stream.Context()

	for {
		req, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		resp, err := s.doCallMethodRpc(ctx, req)
		if err != nil {
			// TODO: Should we send an error response on the stream?
			// For now, we just return the error, which will close the stream.
			return err
		}

		if err := stream.Send(resp); err != nil {
			return err
		}
	}
}
