// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v4.24.3
// source: chat/messages/dialog.proto

package messages

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Chat Dialog. Conversation info.
type Dialog struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// [D]omain[C]omponent primary ID.
	Dc int64 `protobuf:"varint,1,opt,name=dc,proto3" json:"dc,omitempty"`
	// The Conversation thread unique ID.
	Id string `protobuf:"bytes,2,opt,name=id,proto3" json:"id,omitempty"`
	// [VIA] Text gateway [FROM] originated thru ...
	Via *Peer `protobuf:"bytes,3,opt,name=via,proto3" json:"via,omitempty"`
	// [FROM]: Originator.
	// Leg[A]. Contact / User.
	From *Peer `protobuf:"bytes,4,opt,name=from,proto3" json:"from,omitempty"`
	// Timestamp of the latest activity.
	Date int64 `protobuf:"varint,6,opt,name=date,proto3" json:"date,omitempty"`
	// Title of the dialog.
	Title string `protobuf:"bytes,7,opt,name=title,proto3" json:"title,omitempty"`
	// Timestamp when dialog was closed.
	// Zero value means - connected (online)
	// Otherwise - disconnected (offline)
	Closed int64 `protobuf:"varint,8,opt,name=closed,proto3" json:"closed,omitempty"`
	// Timestamp when dialog started.
	Started int64 `protobuf:"varint,9,opt,name=started,proto3" json:"started,omitempty"`
	// The latest (top) message.
	Message *Message `protobuf:"bytes,10,opt,name=message,proto3" json:"message,omitempty"`
	// Context. Variables. Environment.
	Context map[string]string `protobuf:"bytes,11,rep,name=context,proto3" json:"context,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// [TO]: Participants.
	// Leg[A+]. Schema / Agent.
	Members []*Chat `protobuf:"bytes,12,rep,name=members,proto3" json:"members,omitempty"`
}

func (x *Dialog) Reset() {
	*x = Dialog{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_messages_dialog_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Dialog) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Dialog) ProtoMessage() {}

func (x *Dialog) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_dialog_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Dialog.ProtoReflect.Descriptor instead.
func (*Dialog) Descriptor() ([]byte, []int) {
	return file_chat_messages_dialog_proto_rawDescGZIP(), []int{0}
}

func (x *Dialog) GetDc() int64 {
	if x != nil {
		return x.Dc
	}
	return 0
}

func (x *Dialog) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Dialog) GetVia() *Peer {
	if x != nil {
		return x.Via
	}
	return nil
}

func (x *Dialog) GetFrom() *Peer {
	if x != nil {
		return x.From
	}
	return nil
}

func (x *Dialog) GetDate() int64 {
	if x != nil {
		return x.Date
	}
	return 0
}

func (x *Dialog) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *Dialog) GetClosed() int64 {
	if x != nil {
		return x.Closed
	}
	return 0
}

func (x *Dialog) GetStarted() int64 {
	if x != nil {
		return x.Started
	}
	return 0
}

func (x *Dialog) GetMessage() *Message {
	if x != nil {
		return x.Message
	}
	return nil
}

func (x *Dialog) GetContext() map[string]string {
	if x != nil {
		return x.Context
	}
	return nil
}

func (x *Dialog) GetMembers() []*Chat {
	if x != nil {
		return x.Members
	}
	return nil
}

// ChatDialogs dataset
type ChatDialogs struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Dataset page of Dialog(s).
	Data []*Dialog `protobuf:"bytes,1,rep,name=data,proto3" json:"data,omitempty"`
	// Page number of results.
	Page int32 `protobuf:"varint,2,opt,name=page,proto3" json:"page,omitempty"`
	// Next page available ?
	Next bool `protobuf:"varint,3,opt,name=next,proto3" json:"next,omitempty"`
}

func (x *ChatDialogs) Reset() {
	*x = ChatDialogs{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_messages_dialog_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChatDialogs) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatDialogs) ProtoMessage() {}

func (x *ChatDialogs) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_dialog_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChatDialogs.ProtoReflect.Descriptor instead.
func (*ChatDialogs) Descriptor() ([]byte, []int) {
	return file_chat_messages_dialog_proto_rawDescGZIP(), []int{1}
}

func (x *ChatDialogs) GetData() []*Dialog {
	if x != nil {
		return x.Data
	}
	return nil
}

func (x *ChatDialogs) GetPage() int32 {
	if x != nil {
		return x.Page
	}
	return 0
}

func (x *ChatDialogs) GetNext() bool {
	if x != nil {
		return x.Next
	}
	return false
}

type ChatDialogsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Page number to return. **default**: 1.
	Page int32 `protobuf:"varint,1,opt,name=page,proto3" json:"page,omitempty"`
	// Page records limit. **default**: 16.
	Size int32 `protobuf:"varint,2,opt,name=size,proto3" json:"size,omitempty"`
	// Search term: peer.name
	Q string `protobuf:"bytes,5,opt,name=q,proto3" json:"q,omitempty"`
	// Sort records by { fields } specification.
	Sort []string `protobuf:"bytes,3,rep,name=sort,proto3" json:"sort,omitempty"`
	// Fields [Q]uery to build result dataset record.
	Fields []string `protobuf:"bytes,4,rep,name=fields,proto3" json:"fields,omitempty"`
	// Set of unique chat IDentifier(s).
	// Accept: dialog -or- member ID.
	Id []string `protobuf:"bytes,6,rep,name=id,proto3" json:"id,omitempty"`
	// [VIA] Text gateway.
	Via *Peer `protobuf:"bytes,7,opt,name=via,proto3" json:"via,omitempty"`
	// [PEER] Member of ...
	Peer *Peer `protobuf:"bytes,8,opt,name=peer,proto3" json:"peer,omitempty"`
	// Date within timerange.
	Date *Timerange `protobuf:"bytes,9,opt,name=date,proto3" json:"date,omitempty"`
	// Dialogs ONLY that are currently [not] active( closed: ? ).
	Online *wrapperspb.BoolValue `protobuf:"bytes,10,opt,name=online,proto3" json:"online,omitempty"`
}

func (x *ChatDialogsRequest) Reset() {
	*x = ChatDialogsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_chat_messages_dialog_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChatDialogsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatDialogsRequest) ProtoMessage() {}

func (x *ChatDialogsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_dialog_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChatDialogsRequest.ProtoReflect.Descriptor instead.
func (*ChatDialogsRequest) Descriptor() ([]byte, []int) {
	return file_chat_messages_dialog_proto_rawDescGZIP(), []int{2}
}

func (x *ChatDialogsRequest) GetPage() int32 {
	if x != nil {
		return x.Page
	}
	return 0
}

func (x *ChatDialogsRequest) GetSize() int32 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *ChatDialogsRequest) GetQ() string {
	if x != nil {
		return x.Q
	}
	return ""
}

func (x *ChatDialogsRequest) GetSort() []string {
	if x != nil {
		return x.Sort
	}
	return nil
}

func (x *ChatDialogsRequest) GetFields() []string {
	if x != nil {
		return x.Fields
	}
	return nil
}

func (x *ChatDialogsRequest) GetId() []string {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *ChatDialogsRequest) GetVia() *Peer {
	if x != nil {
		return x.Via
	}
	return nil
}

func (x *ChatDialogsRequest) GetPeer() *Peer {
	if x != nil {
		return x.Peer
	}
	return nil
}

func (x *ChatDialogsRequest) GetDate() *Timerange {
	if x != nil {
		return x.Date
	}
	return nil
}

func (x *ChatDialogsRequest) GetOnline() *wrapperspb.BoolValue {
	if x != nil {
		return x.Online
	}
	return nil
}

var File_chat_messages_dialog_proto protoreflect.FileDescriptor

var file_chat_messages_dialog_proto_rawDesc = []byte{
	0x0a, 0x1a, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f,
	0x64, 0x69, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x77, 0x65,
	0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x1a, 0x18, 0x63, 0x68, 0x61, 0x74,
	0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x70, 0x65, 0x65, 0x72, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x18, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x73, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b,
	0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1e, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x77, 0x72, 0x61,
	0x70, 0x70, 0x65, 0x72, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xaa, 0x03, 0x0a, 0x06,
	0x44, 0x69, 0x61, 0x6c, 0x6f, 0x67, 0x12, 0x0e, 0x0a, 0x02, 0x64, 0x63, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x02, 0x64, 0x63, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x24, 0x0a, 0x03, 0x76, 0x69, 0x61, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68,
	0x61, 0x74, 0x2e, 0x50, 0x65, 0x65, 0x72, 0x52, 0x03, 0x76, 0x69, 0x61, 0x12, 0x26, 0x0a, 0x04,
	0x66, 0x72, 0x6f, 0x6d, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x77, 0x65, 0x62,
	0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x50, 0x65, 0x65, 0x72, 0x52, 0x04,
	0x66, 0x72, 0x6f, 0x6d, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x65, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x04, 0x64, 0x61, 0x74, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c,
	0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x16,
	0x0a, 0x06, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x64, 0x18, 0x08, 0x20, 0x01, 0x28, 0x03, 0x52, 0x06,
	0x63, 0x6c, 0x6f, 0x73, 0x65, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65,
	0x64, 0x18, 0x09, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x64,
	0x12, 0x2f, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x15, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74,
	0x2e, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67,
	0x65, 0x12, 0x3b, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x18, 0x0b, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x21, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61,
	0x74, 0x2e, 0x44, 0x69, 0x61, 0x6c, 0x6f, 0x67, 0x2e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74,
	0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x12, 0x2c,
	0x0a, 0x07, 0x6d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x73, 0x18, 0x0c, 0x20, 0x03, 0x28, 0x0b, 0x32,
	0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x43,
	0x68, 0x61, 0x74, 0x52, 0x07, 0x6d, 0x65, 0x6d, 0x62, 0x65, 0x72, 0x73, 0x1a, 0x3a, 0x0a, 0x0c,
	0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03,
	0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14,
	0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x5f, 0x0a, 0x0b, 0x43, 0x68, 0x61, 0x74,
	0x44, 0x69, 0x61, 0x6c, 0x6f, 0x67, 0x73, 0x12, 0x28, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18,
	0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e,
	0x63, 0x68, 0x61, 0x74, 0x2e, 0x44, 0x69, 0x61, 0x6c, 0x6f, 0x67, 0x52, 0x04, 0x64, 0x61, 0x74,
	0x61, 0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x04, 0x70, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x65, 0x78, 0x74, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x04, 0x6e, 0x65, 0x78, 0x74, 0x22, 0xb5, 0x02, 0x0a, 0x12, 0x43, 0x68,
	0x61, 0x74, 0x44, 0x69, 0x61, 0x6c, 0x6f, 0x67, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04,
	0x70, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x0c, 0x0a, 0x01, 0x71, 0x18, 0x05, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x01, 0x71, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x6f, 0x72, 0x74, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x73, 0x6f, 0x72, 0x74, 0x12, 0x16, 0x0a, 0x06, 0x66, 0x69,
	0x65, 0x6c, 0x64, 0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c,
	0x64, 0x73, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x06, 0x20, 0x03, 0x28, 0x09, 0x52, 0x02,
	0x69, 0x64, 0x12, 0x24, 0x0a, 0x03, 0x76, 0x69, 0x61, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x50,
	0x65, 0x65, 0x72, 0x52, 0x03, 0x76, 0x69, 0x61, 0x12, 0x26, 0x0a, 0x04, 0x70, 0x65, 0x65, 0x72,
	0x18, 0x08, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c,
	0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x50, 0x65, 0x65, 0x72, 0x52, 0x04, 0x70, 0x65, 0x65, 0x72,
	0x12, 0x2b, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17,
	0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x54, 0x69,
	0x6d, 0x65, 0x72, 0x61, 0x6e, 0x67, 0x65, 0x52, 0x04, 0x64, 0x61, 0x74, 0x65, 0x12, 0x32, 0x0a,
	0x06, 0x6f, 0x6e, 0x6c, 0x69, 0x6e, 0x65, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e,
	0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x06, 0x6f, 0x6e, 0x6c, 0x69, 0x6e,
	0x65, 0x42, 0x39, 0x5a, 0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f,
	0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x6d, 0x61, 0x6e,
	0x61, 0x67, 0x65, 0x72, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63,
	0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_chat_messages_dialog_proto_rawDescOnce sync.Once
	file_chat_messages_dialog_proto_rawDescData = file_chat_messages_dialog_proto_rawDesc
)

func file_chat_messages_dialog_proto_rawDescGZIP() []byte {
	file_chat_messages_dialog_proto_rawDescOnce.Do(func() {
		file_chat_messages_dialog_proto_rawDescData = protoimpl.X.CompressGZIP(file_chat_messages_dialog_proto_rawDescData)
	})
	return file_chat_messages_dialog_proto_rawDescData
}

var file_chat_messages_dialog_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_chat_messages_dialog_proto_goTypes = []interface{}{
	(*Dialog)(nil),               // 0: webitel.chat.Dialog
	(*ChatDialogs)(nil),          // 1: webitel.chat.ChatDialogs
	(*ChatDialogsRequest)(nil),   // 2: webitel.chat.ChatDialogsRequest
	nil,                          // 3: webitel.chat.Dialog.ContextEntry
	(*Peer)(nil),                 // 4: webitel.chat.Peer
	(*Message)(nil),              // 5: webitel.chat.Message
	(*Chat)(nil),                 // 6: webitel.chat.Chat
	(*Timerange)(nil),            // 7: webitel.chat.Timerange
	(*wrapperspb.BoolValue)(nil), // 8: google.protobuf.BoolValue
}
var file_chat_messages_dialog_proto_depIdxs = []int32{
	4,  // 0: webitel.chat.Dialog.via:type_name -> webitel.chat.Peer
	4,  // 1: webitel.chat.Dialog.from:type_name -> webitel.chat.Peer
	5,  // 2: webitel.chat.Dialog.message:type_name -> webitel.chat.Message
	3,  // 3: webitel.chat.Dialog.context:type_name -> webitel.chat.Dialog.ContextEntry
	6,  // 4: webitel.chat.Dialog.members:type_name -> webitel.chat.Chat
	0,  // 5: webitel.chat.ChatDialogs.data:type_name -> webitel.chat.Dialog
	4,  // 6: webitel.chat.ChatDialogsRequest.via:type_name -> webitel.chat.Peer
	4,  // 7: webitel.chat.ChatDialogsRequest.peer:type_name -> webitel.chat.Peer
	7,  // 8: webitel.chat.ChatDialogsRequest.date:type_name -> webitel.chat.Timerange
	8,  // 9: webitel.chat.ChatDialogsRequest.online:type_name -> google.protobuf.BoolValue
	10, // [10:10] is the sub-list for method output_type
	10, // [10:10] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_chat_messages_dialog_proto_init() }
func file_chat_messages_dialog_proto_init() {
	if File_chat_messages_dialog_proto != nil {
		return
	}
	file_chat_messages_peer_proto_init()
	file_chat_messages_chat_proto_init()
	file_chat_messages_message_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_chat_messages_dialog_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Dialog); i {
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
		file_chat_messages_dialog_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChatDialogs); i {
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
		file_chat_messages_dialog_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChatDialogsRequest); i {
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
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_chat_messages_dialog_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_chat_messages_dialog_proto_goTypes,
		DependencyIndexes: file_chat_messages_dialog_proto_depIdxs,
		MessageInfos:      file_chat_messages_dialog_proto_msgTypes,
	}.Build()
	File_chat_messages_dialog_proto = out.File
	file_chat_messages_dialog_proto_rawDesc = nil
	file_chat_messages_dialog_proto_goTypes = nil
	file_chat_messages_dialog_proto_depIdxs = nil
}