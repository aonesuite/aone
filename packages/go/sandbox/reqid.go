package sandbox

import "context"

type reqidContextKey struct{}

// WithReqid stores a request ID in ctx. SDK HTTP requests created with the
// derived context propagate the value as the X-Reqid header, which helps callers
// correlate application logs with server-side request logs.
func WithReqid(ctx context.Context, reqid string) context.Context {
	return context.WithValue(ctx, reqidContextKey{}, reqid)
}

// ReqidFromContext extracts a request ID previously stored by WithReqid. The
// boolean return value is false when no non-empty request ID is present.
func ReqidFromContext(ctx context.Context) (string, bool) {
	reqid, ok := ctx.Value(reqidContextKey{}).(string)
	return reqid, ok && reqid != ""
}
