// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
// 	protoc        v6.30.2
// source: internal/delivery/grpc/comments/proto/comment.proto

package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type AffectedComment struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            int64                  `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *AffectedComment) Reset() {
	*x = AffectedComment{}
	mi := &file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *AffectedComment) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AffectedComment) ProtoMessage() {}

func (x *AffectedComment) ProtoReflect() protoreflect.Message {
	mi := &file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AffectedComment.ProtoReflect.Descriptor instead.
func (*AffectedComment) Descriptor() ([]byte, []int) {
	return file_internal_delivery_grpc_comments_proto_comment_proto_rawDescGZIP(), []int{0}
}

func (x *AffectedComment) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

type Nothing struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Nothing) Reset() {
	*x = Nothing{}
	mi := &file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Nothing) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Nothing) ProtoMessage() {}

func (x *Nothing) ProtoReflect() protoreflect.Message {
	mi := &file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Nothing.ProtoReflect.Descriptor instead.
func (*Nothing) Descriptor() ([]byte, []int) {
	return file_internal_delivery_grpc_comments_proto_comment_proto_rawDescGZIP(), []int{1}
}

type SubscribeRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	TeamId        int64                  `protobuf:"varint,1,opt,name=team_id,json=teamId,proto3" json:"team_id,omitempty"`
	PostUnionId   int64                  `protobuf:"varint,2,opt,name=post_union_id,json=postUnionId,proto3" json:"post_union_id,omitempty"` // если 0, то подписываемся на всю ленту комментариев
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *SubscribeRequest) Reset() {
	*x = SubscribeRequest{}
	mi := &file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *SubscribeRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SubscribeRequest) ProtoMessage() {}

func (x *SubscribeRequest) ProtoReflect() protoreflect.Message {
	mi := &file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SubscribeRequest.ProtoReflect.Descriptor instead.
func (*SubscribeRequest) Descriptor() ([]byte, []int) {
	return file_internal_delivery_grpc_comments_proto_comment_proto_rawDescGZIP(), []int{2}
}

func (x *SubscribeRequest) GetTeamId() int64 {
	if x != nil {
		return x.TeamId
	}
	return 0
}

func (x *SubscribeRequest) GetPostUnionId() int64 {
	if x != nil {
		return x.PostUnionId
	}
	return 0
}

var File_internal_delivery_grpc_comments_proto_comment_proto protoreflect.FileDescriptor

const file_internal_delivery_grpc_comments_proto_comment_proto_rawDesc = "" +
	"\n" +
	"3internal/delivery/grpc/comments/proto/comment.proto\x12\bcomments\"!\n" +
	"\x0fAffectedComment\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\x03R\x02id\"\t\n" +
	"\aNothing\"O\n" +
	"\x10SubscribeRequest\x12\x17\n" +
	"\ateam_id\x18\x01 \x01(\x03R\x06teamId\x12\"\n" +
	"\rpost_union_id\x18\x02 \x01(\x03R\vpostUnionId2P\n" +
	"\bComments\x12D\n" +
	"\tSubscribe\x12\x1a.comments.SubscribeRequest\x1a\x19.comments.AffectedComment0\x01B\rZ\v./;commentsb\x06proto3"

var (
	file_internal_delivery_grpc_comments_proto_comment_proto_rawDescOnce sync.Once
	file_internal_delivery_grpc_comments_proto_comment_proto_rawDescData []byte
)

func file_internal_delivery_grpc_comments_proto_comment_proto_rawDescGZIP() []byte {
	file_internal_delivery_grpc_comments_proto_comment_proto_rawDescOnce.Do(func() {
		file_internal_delivery_grpc_comments_proto_comment_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_internal_delivery_grpc_comments_proto_comment_proto_rawDesc), len(file_internal_delivery_grpc_comments_proto_comment_proto_rawDesc)))
	})
	return file_internal_delivery_grpc_comments_proto_comment_proto_rawDescData
}

var file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_internal_delivery_grpc_comments_proto_comment_proto_goTypes = []any{
	(*AffectedComment)(nil),  // 0: comments.AffectedComment
	(*Nothing)(nil),          // 1: comments.Nothing
	(*SubscribeRequest)(nil), // 2: comments.SubscribeRequest
}
var file_internal_delivery_grpc_comments_proto_comment_proto_depIdxs = []int32{
	2, // 0: comments.Comments.Subscribe:input_type -> comments.SubscribeRequest
	0, // 1: comments.Comments.Subscribe:output_type -> comments.AffectedComment
	1, // [1:2] is the sub-list for method output_type
	0, // [0:1] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_internal_delivery_grpc_comments_proto_comment_proto_init() }
func file_internal_delivery_grpc_comments_proto_comment_proto_init() {
	if File_internal_delivery_grpc_comments_proto_comment_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_internal_delivery_grpc_comments_proto_comment_proto_rawDesc), len(file_internal_delivery_grpc_comments_proto_comment_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_internal_delivery_grpc_comments_proto_comment_proto_goTypes,
		DependencyIndexes: file_internal_delivery_grpc_comments_proto_comment_proto_depIdxs,
		MessageInfos:      file_internal_delivery_grpc_comments_proto_comment_proto_msgTypes,
	}.Build()
	File_internal_delivery_grpc_comments_proto_comment_proto = out.File
	file_internal_delivery_grpc_comments_proto_comment_proto_goTypes = nil
	file_internal_delivery_grpc_comments_proto_comment_proto_depIdxs = nil
}
