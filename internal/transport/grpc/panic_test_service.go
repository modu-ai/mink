package grpc

// AC-TR-006: 패닉 복구 테스트를 위한 내부 RPC 서비스.
// proto에 노출하지 않으며, 테스트 환경에서만 grpcServer에 동적으로 등록된다.
//
// grpc.ServiceDesc를 직접 등록하여 proto 생성 없이 RPC를 추가한다.

import (
	"context"

	"google.golang.org/grpc"

	"github.com/modu-ai/goose/internal/transport/grpc/gen/goosev1"
)

// PanicTestClient는 테스트 전용 PanicTest RPC 클라이언트다.
type PanicTestClient struct {
	cc grpc.ClientConnInterface
}

// NewPanicTestClient는 PanicTestClient를 반환한다.
func NewPanicTestClient(cc grpc.ClientConnInterface) *PanicTestClient {
	return &PanicTestClient{cc: cc}
}

// TriggerPanic은 서버에서 panic을 유발하는 RPC를 호출한다.
func (c *PanicTestClient) TriggerPanic(ctx context.Context, in *goosev1.PingRequest, opts ...grpc.CallOption) (*goosev1.PingResponse, error) {
	out := new(goosev1.PingResponse)
	err := c.cc.Invoke(ctx, "/goose.internal.PanicTestService/TriggerPanic", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// iPanicTestServer는 PanicTestService 서버 인터페이스다.
type iPanicTestServer interface {
	TriggerPanic(context.Context, *goosev1.PingRequest) (*goosev1.PingResponse, error)
}

// panicTestServerImpl은 panic을 유발하는 핸들러 구현이다.
type panicTestServerImpl struct{}

func (s *panicTestServerImpl) TriggerPanic(_ context.Context, _ *goosev1.PingRequest) (*goosev1.PingResponse, error) {
	panic("boom — intentional test panic")
}

// panicTestServiceDesc는 PanicTestService의 grpc.ServiceDesc이다.
var panicTestServiceDesc = grpc.ServiceDesc{
	ServiceName: "goose.internal.PanicTestService",
	HandlerType: (*iPanicTestServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "TriggerPanic",
			Handler:    panicTestHandler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

func panicTestHandler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(goosev1.PingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(iPanicTestServer).TriggerPanic(ctx, req.(*goosev1.PingRequest))
	}
	if interceptor == nil {
		return handler(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/goose.internal.PanicTestService/TriggerPanic",
	}
	return interceptor(ctx, in, info, handler)
}

// registerPanicTestService는 테스트 전용 PanicTestService를 gRPC 서버에 등록한다.
func registerPanicTestService(s *grpc.Server) {
	s.RegisterService(&panicTestServiceDesc, &panicTestServerImpl{})
}
