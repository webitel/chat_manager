// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.4
// source: bot/facebook/internal/whatsapp.proto

package proro

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

type WhatsApp struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Accounts []*WhatsApp_BusinessAccount `protobuf:"bytes,1,rep,name=accounts,proto3" json:"accounts,omitempty"`
}

func (x *WhatsApp) Reset() {
	*x = WhatsApp{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WhatsApp) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WhatsApp) ProtoMessage() {}

func (x *WhatsApp) ProtoReflect() protoreflect.Message {
	mi := &file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WhatsApp.ProtoReflect.Descriptor instead.
func (*WhatsApp) Descriptor() ([]byte, []int) {
	return file_bot_facebook_v12_internal_whatsapp_proto_rawDescGZIP(), []int{0}
}

func (x *WhatsApp) GetAccounts() []*WhatsApp_BusinessAccount {
	if x != nil {
		return x.Accounts
	}
	return nil
}

type WhatsApp_PhoneNumber struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id           string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	PhoneNumber  string `protobuf:"bytes,2,opt,name=phoneNumber,proto3" json:"phoneNumber,omitempty"`
	VerifiedName string `protobuf:"bytes,3,opt,name=verifiedName,proto3" json:"verifiedName,omitempty"`
}

func (x *WhatsApp_PhoneNumber) Reset() {
	*x = WhatsApp_PhoneNumber{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WhatsApp_PhoneNumber) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WhatsApp_PhoneNumber) ProtoMessage() {}

func (x *WhatsApp_PhoneNumber) ProtoReflect() protoreflect.Message {
	mi := &file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WhatsApp_PhoneNumber.ProtoReflect.Descriptor instead.
func (*WhatsApp_PhoneNumber) Descriptor() ([]byte, []int) {
	return file_bot_facebook_v12_internal_whatsapp_proto_rawDescGZIP(), []int{0, 0}
}

func (x *WhatsApp_PhoneNumber) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *WhatsApp_PhoneNumber) GetPhoneNumber() string {
	if x != nil {
		return x.PhoneNumber
	}
	return ""
}

func (x *WhatsApp_PhoneNumber) GetVerifiedName() string {
	if x != nil {
		return x.VerifiedName
	}
	return ""
}

type WhatsApp_BusinessAccount struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id           string                  `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Name         string                  `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
	PhoneNumbers []*WhatsApp_PhoneNumber `protobuf:"bytes,3,rep,name=phoneNumbers,proto3" json:"phoneNumbers,omitempty"`
	Subscribed   bool                    `protobuf:"varint,4,opt,name=subscribed,proto3" json:"subscribed,omitempty"`
}

func (x *WhatsApp_BusinessAccount) Reset() {
	*x = WhatsApp_BusinessAccount{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *WhatsApp_BusinessAccount) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*WhatsApp_BusinessAccount) ProtoMessage() {}

func (x *WhatsApp_BusinessAccount) ProtoReflect() protoreflect.Message {
	mi := &file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use WhatsApp_BusinessAccount.ProtoReflect.Descriptor instead.
func (*WhatsApp_BusinessAccount) Descriptor() ([]byte, []int) {
	return file_bot_facebook_v12_internal_whatsapp_proto_rawDescGZIP(), []int{0, 1}
}

func (x *WhatsApp_BusinessAccount) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *WhatsApp_BusinessAccount) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *WhatsApp_BusinessAccount) GetPhoneNumbers() []*WhatsApp_PhoneNumber {
	if x != nil {
		return x.PhoneNumbers
	}
	return nil
}

func (x *WhatsApp_BusinessAccount) GetSubscribed() bool {
	if x != nil {
		return x.Subscribed
	}
	return false
}

var File_bot_facebook_v12_internal_whatsapp_proto protoreflect.FileDescriptor

var file_bot_facebook_v12_internal_whatsapp_proto_rawDesc = []byte{
	0x0a, 0x28, 0x62, 0x6f, 0x74, 0x2f, 0x66, 0x61, 0x63, 0x65, 0x62, 0x6f, 0x6f, 0x6b, 0x2e, 0x76,
	0x31, 0x32, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x77, 0x68, 0x61, 0x74,
	0x73, 0x61, 0x70, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x77, 0x65, 0x62, 0x69,
	0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x22, 0xdb, 0x02, 0x0a,
	0x08, 0x57, 0x68, 0x61, 0x74, 0x73, 0x41, 0x70, 0x70, 0x12, 0x46, 0x0a, 0x08, 0x61, 0x63, 0x63,
	0x6f, 0x75, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x77, 0x65,
	0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x57,
	0x68, 0x61, 0x74, 0x73, 0x41, 0x70, 0x70, 0x2e, 0x42, 0x75, 0x73, 0x69, 0x6e, 0x65, 0x73, 0x73,
	0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x52, 0x08, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74,
	0x73, 0x1a, 0x63, 0x0a, 0x0b, 0x50, 0x68, 0x6f, 0x6e, 0x65, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x20, 0x0a, 0x0b, 0x70, 0x68, 0x6f, 0x6e, 0x65, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0b, 0x70, 0x68, 0x6f, 0x6e, 0x65, 0x4e, 0x75, 0x6d, 0x62,
	0x65, 0x72, 0x12, 0x22, 0x0a, 0x0c, 0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x65, 0x64, 0x4e, 0x61,
	0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x76, 0x65, 0x72, 0x69, 0x66, 0x69,
	0x65, 0x64, 0x4e, 0x61, 0x6d, 0x65, 0x1a, 0xa1, 0x01, 0x0a, 0x0f, 0x42, 0x75, 0x73, 0x69, 0x6e,
	0x65, 0x73, 0x73, 0x41, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x4a,
	0x0a, 0x0c, 0x70, 0x68, 0x6f, 0x6e, 0x65, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x73, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x26, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63,
	0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x57, 0x68, 0x61, 0x74, 0x73, 0x41, 0x70, 0x70,
	0x2e, 0x50, 0x68, 0x6f, 0x6e, 0x65, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x52, 0x0c, 0x70, 0x68,
	0x6f, 0x6e, 0x65, 0x4e, 0x75, 0x6d, 0x62, 0x65, 0x72, 0x73, 0x12, 0x1e, 0x0a, 0x0a, 0x73, 0x75,
	0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x52, 0x0a,
	0x73, 0x75, 0x62, 0x73, 0x63, 0x72, 0x69, 0x62, 0x65, 0x64, 0x42, 0x41, 0x5a, 0x3f, 0x67, 0x69,
	0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c,
	0x2f, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x2f, 0x62, 0x6f,
	0x74, 0x2f, 0x66, 0x61, 0x63, 0x65, 0x62, 0x6f, 0x6f, 0x6b, 0x2e, 0x76, 0x31, 0x32, 0x2f, 0x69,
	0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x3b, 0x70, 0x72, 0x6f, 0x72, 0x6f, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_bot_facebook_v12_internal_whatsapp_proto_rawDescOnce sync.Once
	file_bot_facebook_v12_internal_whatsapp_proto_rawDescData = file_bot_facebook_v12_internal_whatsapp_proto_rawDesc
)

func file_bot_facebook_v12_internal_whatsapp_proto_rawDescGZIP() []byte {
	file_bot_facebook_v12_internal_whatsapp_proto_rawDescOnce.Do(func() {
		file_bot_facebook_v12_internal_whatsapp_proto_rawDescData = protoimpl.X.CompressGZIP(file_bot_facebook_v12_internal_whatsapp_proto_rawDescData)
	})
	return file_bot_facebook_v12_internal_whatsapp_proto_rawDescData
}

var file_bot_facebook_v12_internal_whatsapp_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_bot_facebook_v12_internal_whatsapp_proto_goTypes = []interface{}{
	(*WhatsApp)(nil),                 // 0: webitel.chat.bot.WhatsApp
	(*WhatsApp_PhoneNumber)(nil),     // 1: webitel.chat.bot.WhatsApp.PhoneNumber
	(*WhatsApp_BusinessAccount)(nil), // 2: webitel.chat.bot.WhatsApp.BusinessAccount
}
var file_bot_facebook_v12_internal_whatsapp_proto_depIdxs = []int32{
	2, // 0: webitel.chat.bot.WhatsApp.accounts:type_name -> webitel.chat.bot.WhatsApp.BusinessAccount
	1, // 1: webitel.chat.bot.WhatsApp.BusinessAccount.phoneNumbers:type_name -> webitel.chat.bot.WhatsApp.PhoneNumber
	2, // [2:2] is the sub-list for method output_type
	2, // [2:2] is the sub-list for method input_type
	2, // [2:2] is the sub-list for extension type_name
	2, // [2:2] is the sub-list for extension extendee
	0, // [0:2] is the sub-list for field type_name
}

func init() { file_bot_facebook_v12_internal_whatsapp_proto_init() }
func file_bot_facebook_v12_internal_whatsapp_proto_init() {
	if File_bot_facebook_v12_internal_whatsapp_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WhatsApp); i {
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
		file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WhatsApp_PhoneNumber); i {
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
		file_bot_facebook_v12_internal_whatsapp_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*WhatsApp_BusinessAccount); i {
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
			RawDescriptor: file_bot_facebook_v12_internal_whatsapp_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_bot_facebook_v12_internal_whatsapp_proto_goTypes,
		DependencyIndexes: file_bot_facebook_v12_internal_whatsapp_proto_depIdxs,
		MessageInfos:      file_bot_facebook_v12_internal_whatsapp_proto_msgTypes,
	}.Build()
	File_bot_facebook_v12_internal_whatsapp_proto = out.File
	file_bot_facebook_v12_internal_whatsapp_proto_rawDesc = nil
	file_bot_facebook_v12_internal_whatsapp_proto_goTypes = nil
	file_bot_facebook_v12_internal_whatsapp_proto_depIdxs = nil
}
