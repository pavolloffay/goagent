package grpc

import (
	"context"
	"strings"

	"github.com/traceableai/goagent/sdk"
	"github.com/traceableai/goagent/sdk/internal/container"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// WrapUnaryClientInterceptor returns an interceptor that records the request and response message's body
// and serialize it as JSON.
func WrapUnaryClientInterceptor(delegateInterceptor grpc.UnaryClientInterceptor, spanFromContext sdk.SpanFromContext) grpc.UnaryClientInterceptor {
	defaultAttributes := map[string]string{
		"rpc.system": "grpc",
	}
	if containerID, err := container.GetID(); err == nil {
		defaultAttributes["container_id"] = containerID
	}

	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		var header metadata.MD
		var trailer metadata.MD

		// GRPC interceptors do not support request/response parsing so the only way to
		// achieve it is by wrapping the invoker (where we can still access the current
		// span).
		wrappedInvoker := func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
			span := spanFromContext(ctx)
			if span.IsNoop() || span == nil {
				// isNoop means either the span is not sampled or there was no span
				// in the request context which means this invoker is not used
				// inside an instrumented invoker, hence we just invoke the delegate
				// round tripper.
				return invoker(ctx, method, req, reply, cc, opts...)
			}
			for key, value := range defaultAttributes {
				span.SetAttribute(key, value)
			}

			pieces := strings.Split(method[1:], "/")
			span.SetAttribute("rpc.service", pieces[0])
			span.SetAttribute("rpc.method", pieces[1])

			reqBody, err := marshalMessageableJSON(req)
			if len(reqBody) > 0 && err == nil {
				span.SetAttribute("rpc.request.body", string(reqBody))
			}

			setAttributesFromRequestOutgoingMetadata(ctx, span)

			err = invoker(ctx, method, req, reply, cc, opts...)
			if err != nil {
				return err
			}

			setAttributesFromMetadata("response", header, span)
			setAttributesFromMetadata("response", trailer, span)

			resBody, err := marshalMessageableJSON(reply)
			if len(resBody) > 0 && err == nil {
				span.SetAttribute("rpc.response.body", string(resBody))
			}

			return err
		}

		// Even if user pases a header or trailer the data is being populated
		// in all the headers and trailers registered.
		opts = append(opts, grpc.Header(&header), grpc.Trailer(&trailer))

		return delegateInterceptor(ctx, method, req, reply, cc, wrappedInvoker, opts...)
	}
}