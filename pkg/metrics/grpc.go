package metrics

import (
	"context"
	"time"
	
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func UnaryServerInterceptor(serviceName string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start).Seconds()
		method := info.FullMethod
		stat := "OK"
		if err != nil {
			stat = status.Code(err).String()
		}
		
		GRPCRequestsTotal.WithLabelValues(serviceName, method, stat).Inc()
		GRPCRequestDuration.WithLabelValues(serviceName, method).Observe(duration)
		
		return resp, err
	}
}

func StreamServerInterceptor(serviceName string) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		start := time.Now()
		err := handler(srv, stream)
		duration := time.Since(start).Seconds()
		method := info.FullMethod
		stat := "OK"
		if err != nil {
			stat = status.Code(err).String()
		}
		
		GRPCRequestsTotal.WithLabelValues(serviceName, method, stat).Inc()
		GRPCRequestDuration.WithLabelValues(serviceName, method).Observe(duration)
		
		return err
	}
}