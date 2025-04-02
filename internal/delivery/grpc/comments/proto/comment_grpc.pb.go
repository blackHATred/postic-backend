// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v6.30.2
// source: internal/delivery/grpc/comments/proto/comment.proto

package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	Comments_Subscribe_FullMethodName = "/comments.Comments/Subscribe"
)

// CommentsClient is the client API for Comments service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Клиент подписывается на изменения в комментариях, передавая ID команды
type CommentsClient interface {
	Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[AffectedComment], error)
}

type commentsClient struct {
	cc grpc.ClientConnInterface
}

func NewCommentsClient(cc grpc.ClientConnInterface) CommentsClient {
	return &commentsClient{cc}
}

func (c *commentsClient) Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[AffectedComment], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &Comments_ServiceDesc.Streams[0], Comments_Subscribe_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[SubscribeRequest, AffectedComment]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Comments_SubscribeClient = grpc.ServerStreamingClient[AffectedComment]

// CommentsServer is the server API for Comments service.
// All implementations must embed UnimplementedCommentsServer
// for forward compatibility.
//
// Клиент подписывается на изменения в комментариях, передавая ID команды
type CommentsServer interface {
	Subscribe(*SubscribeRequest, grpc.ServerStreamingServer[AffectedComment]) error
	mustEmbedUnimplementedCommentsServer()
}

// UnimplementedCommentsServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedCommentsServer struct{}

func (UnimplementedCommentsServer) Subscribe(*SubscribeRequest, grpc.ServerStreamingServer[AffectedComment]) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedCommentsServer) mustEmbedUnimplementedCommentsServer() {}
func (UnimplementedCommentsServer) testEmbeddedByValue()                  {}

// UnsafeCommentsServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to CommentsServer will
// result in compilation errors.
type UnsafeCommentsServer interface {
	mustEmbedUnimplementedCommentsServer()
}

func RegisterCommentsServer(s grpc.ServiceRegistrar, srv CommentsServer) {
	// If the following call pancis, it indicates UnimplementedCommentsServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&Comments_ServiceDesc, srv)
}

func _Comments_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(CommentsServer).Subscribe(m, &grpc.GenericServerStream[SubscribeRequest, AffectedComment]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type Comments_SubscribeServer = grpc.ServerStreamingServer[AffectedComment]

// Comments_ServiceDesc is the grpc.ServiceDesc for Comments service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var Comments_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "comments.Comments",
	HandlerType: (*CommentsServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Subscribe",
			Handler:       _Comments_Subscribe_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "internal/delivery/grpc/comments/proto/comment.proto",
}
