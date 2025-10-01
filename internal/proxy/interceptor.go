package proxy

import (
	"context"
	"net/http"
	"strings"

	"grpc2mcp/internal/mcpconst"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// TODO redo this logic to make it easier to read
func methodRequiresMcpSessionHeader(method string) bool {
	return !(strings.HasSuffix(method, "Initialize") || method == "/grpc.reflection.v1.ServerReflection/ServerReflectionInfo")
}

// TODO figure out why and explain the reason for recopying these context vars
func getInterceptorContext(ctx context.Context, method string) (context.Context, error) {

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{} // Initialize if no metadata exists
	}

	// first migrate the authorization header, if present
	authorizationHeader := md.Get(mcpconst.AuthorizationHeader)
	if len(authorizationHeader) > 0 {
		// what TODO if there are more than one?
		// looks like we have at least one header, use the first.
		md.Set(mcpconst.AuthorizationHeader, authorizationHeader[0])
	}

	// Next step is to check/process the session id, if we're not in an initialize
	if methodRequiresMcpSessionHeader(method) {
		sessionID := md.Get(mcpconst.MCP_SESSION_ID_HEADER)

		// if we dont get a session id with our key, try the lower case version which also works with MCP servers
		if len(sessionID) == 0 {
			sessionID = md.Get(strings.ToLower(mcpconst.MCP_SESSION_ID_HEADER))
		}

		// if still empty, report an error
		if len(sessionID) == 0 {
			return nil, status.Errorf(codes.Unauthenticated, "missing header: %s", mcpconst.MCP_SESSION_ID_HEADER)
		}

		md.Set(mcpconst.MCP_SESSION_ID_HEADER, sessionID[0])
	}

	// now let's create a new context with the md we've assembled
	newCtx := metadata.NewIncomingContext(ctx, md)

	return newCtx, nil
}

// sessionInterceptor is a gRPC unary interceptor that checks for the MCP_SESSION_ID_HEADER header.
func unarySessionInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {

	returnCtx, err := getInterceptorContext(ctx, info.FullMethod)

	if err != nil {
		return nil, err
	}

	return handler(returnCtx, req)
}

func streamSessionInterceptor(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	newCtx, err := getInterceptorContext(ss.Context(), info.FullMethod)

	if err != nil {
		return err
	}
	type serverStream struct {
		grpc.ServerStream
		ctx context.Context
	}
	return handler(srv, &serverStream{ss, newCtx})

}

// initHeadersFromContext extracts metadata added by the interceptors to the context and returns
// it as a map of HTTP headers. It first tries to extract gRPC metadata, and then falls back to
// checking for context values for specific headers.
// TODO simplify this, i think we can do one or the other but dont need both
func initHttpHeadersFromContext(ctx context.Context) map[string]string {
	headersFromContext := map[string]string{}

	// Prefer gRPC metadata if present
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		for k, v := range md {
			if len(v) > 0 {
				// gRPC pseudo-headers and content-type are not valid for forwarding.
				if strings.HasPrefix(k, ":") || strings.ToLower(k) == "content-type" {
					continue
				}
				headersFromContext[http.CanonicalHeaderKey(k)] = v[0]
			}
		}
	}

	// Fallback for specific headers that might be in context values (e.g. during Initialize)
	CANDIDATE_HEADERS := []string{mcpconst.MCP_SESSION_ID_HEADER, mcpconst.AuthorizationHeader}
	for _, candidateHeader := range CANDIDATE_HEADERS {
		// Only check context value if not already found in metadata
		if _, exists := headersFromContext[http.CanonicalHeaderKey(candidateHeader)]; !exists {
			if headerVal := ctx.Value(candidateHeader); headerVal != nil {
				if headerValStr, ok := headerVal.(string); ok && headerValStr != "" {
					headersFromContext[http.CanonicalHeaderKey(candidateHeader)] = headerValStr
				}
			}
		}
	}

	return headersFromContext
}
