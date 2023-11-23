// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: chat/messages/peer.proto

package messages

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Peer contact.
type Peer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Contact unique **ID**entifier.
	// Contact **type**-specific string.
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Contact **type** provider.
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
	// Contact display **name**.
	Name string `protobuf:"bytes,3,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Peer) Reset() {
	*x = Peer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_messages_peer_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Peer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Peer) ProtoMessage() {}

func (x *Peer) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_peer_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Peer.ProtoReflect.Descriptor instead.
func (*Peer) Descriptor() ([]byte, []int) {
	return file_chat_messages_peer_proto_rawDescGZIP(), []int{0}
}

func (x *Peer) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Peer) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *Peer) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

// InputPeer identity.
type InputPeer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Type of the input peer.
	//
	// Types that are assignable to Input:
	//
	//	*InputPeer_ChatId
	//	*InputPeer_Peer
	Input isInputPeer_Input `protobuf_oneof:"input"`
}

func (x *InputPeer) Reset() {
	*x = InputPeer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_messages_peer_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *InputPeer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputPeer) ProtoMessage() {}

func (x *InputPeer) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_peer_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputPeer.ProtoReflect.Descriptor instead.
func (*InputPeer) Descriptor() ([]byte, []int) {
	return file_chat_messages_peer_proto_rawDescGZIP(), []int{1}
}

func (m *InputPeer) GetInput() isInputPeer_Input {
	if m != nil {
		return m.Input
	}
	return nil
}

func (x *InputPeer) GetChatId() string {
	if x, ok := x.GetInput().(*InputPeer_ChatId); ok {
		return x.ChatId
	}
	return ""
}

func (x *InputPeer) GetPeer() *Peer {
	if x, ok := x.GetInput().(*InputPeer_Peer); ok {
		return x.Peer
	}
	return nil
}

type isInputPeer_Input interface {
	isInputPeer_Input()
}

type InputPeer_ChatId struct {
	// Unique chat identifier.
	ChatId string `protobuf:"bytes,1,opt,name=chat_id,json=chatId,proto3,oneof"`
}

type InputPeer_Peer struct {
	// Unique peer member of the chat.
	Peer *Peer `protobuf:"bytes,2,opt,name=peer,proto3,oneof"`
}

func (*InputPeer_ChatId) isInputPeer_Input() {}

func (*InputPeer_Peer) isInputPeer_Input() {}

var File_chat_messages_peer_proto protoreflect.FileDescriptor

var file_chat_messages_peer_proto_rawDesc = []byte{
	0x0a, 0x18, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f,
	0x70, 0x65, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x77, 0x65, 0x62, 0x69,
	0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x22, 0x3e, 0x0a, 0x04, 0x50, 0x65, 0x65, 0x72,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x74, 0x79, 0x70, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x59, 0x0a, 0x09, 0x49, 0x6e, 0x70, 0x75,
	0x74, 0x50, 0x65, 0x65, 0x72, 0x12, 0x19, 0x0a, 0x07, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x06, 0x63, 0x68, 0x61, 0x74, 0x49, 0x64,
	0x12, 0x28, 0x0a, 0x04, 0x70, 0x65, 0x65, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12,
	0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x50, 0x65,
	0x65, 0x72, 0x48, 0x00, 0x52, 0x04, 0x70, 0x65, 0x65, 0x72, 0x42, 0x07, 0x0a, 0x05, 0x69, 0x6e,
	0x70, 0x75, 0x74, 0x42, 0x39, 0x5a, 0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x6d,
	0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x2f, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_chat_messages_peer_proto_rawDescOnce sync.Once
	file_chat_messages_peer_proto_rawDescData = file_chat_messages_peer_proto_rawDesc
)

func file_chat_messages_peer_proto_rawDescGZIP() []byte {
	file_chat_messages_peer_proto_rawDescOnce.Do(func() {
		file_chat_messages_peer_proto_rawDescData = protoimpl.X.CompressGZIP(file_chat_messages_peer_proto_rawDescData)
	})
	return file_chat_messages_peer_proto_rawDescData
}

var file_chat_messages_peer_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_chat_messages_peer_proto_goTypes = []interface{}{
	(*Peer)(nil),      // 0: webitel.chat.Peer
	(*InputPeer)(nil), // 1: webitel.chat.InputPeer
}
var file_chat_messages_peer_proto_depIdxs = []int32{
	0, // 0: webitel.chat.InputPeer.peer:type_name -> webitel.chat.Peer
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_chat_messages_peer_proto_init() }
func file_chat_messages_peer_proto_init() {
	if File_chat_messages_peer_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_chat_messages_peer_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Peer); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_chat_messages_peer_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*InputPeer); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	file_chat_messages_peer_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*InputPeer_ChatId)(nil),
		(*InputPeer_Peer)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_chat_messages_peer_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_chat_messages_peer_proto_goTypes,
		DependencyIndexes: file_chat_messages_peer_proto_depIdxs,
		MessageInfos:      file_chat_messages_peer_proto_msgTypes,
	}.Build()
	File_chat_messages_peer_proto = out.File
	file_chat_messages_peer_proto_rawDesc = nil
	file_chat_messages_peer_proto_goTypes = nil
	file_chat_messages_peer_proto_depIdxs = nil
}
