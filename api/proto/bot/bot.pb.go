// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.4
// source: bot.proto

package bot

import (
	chat "github.com/webitel/chat_manager/api/proto/chat"
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

// Reference
type Refer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Readonly. Object Unique IDentifier.
	Id int64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Readonly. Human-readable display name.
	Name string `protobuf:"bytes,2,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Refer) Reset() {
	*x = Refer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Refer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Refer) ProtoMessage() {}

func (x *Refer) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Refer.ProtoReflect.Descriptor instead.
func (*Refer) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{0}
}

func (x *Refer) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Refer) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

// ChatUpdates defines optional text/template(s)
// for some kind of chat updates message notifications
type ChatUpdates struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Title of the NEW chat.
	// Context: chat.Account.
	// Default: {{.FirstName}} {{.LastName}}
	Title string `protobuf:"bytes,1,opt,name=title,proto3" json:"title,omitempty"`
	// Close chat message.
	// Context: none.
	Close string `protobuf:"bytes,2,opt,name=close,proto3" json:"close,omitempty"`
	// Join member update.
	// Context: chat.Account.
	Join string `protobuf:"bytes,3,opt,name=join,proto3" json:"join,omitempty"`
	// Left member update.
	// Context: chat.Account.
	Left string `protobuf:"bytes,4,opt,name=left,proto3" json:"left,omitempty"`
}

func (x *ChatUpdates) Reset() {
	*x = ChatUpdates{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChatUpdates) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChatUpdates) ProtoMessage() {}

func (x *ChatUpdates) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChatUpdates.ProtoReflect.Descriptor instead.
func (*ChatUpdates) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{1}
}

func (x *ChatUpdates) GetTitle() string {
	if x != nil {
		return x.Title
	}
	return ""
}

func (x *ChatUpdates) GetClose() string {
	if x != nil {
		return x.Close
	}
	return ""
}

func (x *ChatUpdates) GetJoin() string {
	if x != nil {
		return x.Join
	}
	return ""
}

func (x *ChatUpdates) GetLeft() string {
	if x != nil {
		return x.Left
	}
	return ""
}

// webitel.chat.server.Profile
type Bot struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Readonly. Object Unique IDentifier.
	Id int64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Readonly. Domain that owns this bot
	Dc *Refer `protobuf:"bytes,2,opt,name=dc,proto3" json:"dc,omitempty"`
	// Required. Relative URI to register and serve this chat bot updates on.
	Uri string `protobuf:"bytes,3,opt,name=uri,proto3" json:"uri,omitempty"`
	// Required. Name this chat bot
	Name string `protobuf:"bytes,4,opt,name=name,proto3" json:"name,omitempty"`
	// Required. Flow schema to connect and serve inbound communication(s)
	Flow *Refer `protobuf:"bytes,5,opt,name=flow,proto3" json:"flow,omitempty"`
	// Optional. Enabled indicates whether this bot is activated or not
	Enabled bool `protobuf:"varint,6,opt,name=enabled,proto3" json:"enabled,omitempty"`
	// Required. Provider communication type to serve this bot connection(s)
	Provider string `protobuf:"bytes,7,opt,name=provider,proto3" json:"provider,omitempty"`
	// Optional. Provider specific bot settings
	Metadata map[string]string `protobuf:"bytes,8,rep,name=metadata,proto3" json:"metadata,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Updates message templates
	Updates *ChatUpdates `protobuf:"bytes,9,opt,name=updates,proto3" json:"updates,omitempty"`
	// Readonly. Created at timestamp
	CreatedAt int64 `protobuf:"varint,10,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	// Readonly. Created by user
	CreatedBy *Refer `protobuf:"bytes,11,opt,name=created_by,json=createdBy,proto3" json:"created_by,omitempty"`
	// Readonly. Updated at timestamp
	UpdatedAt int64 `protobuf:"varint,12,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	// Readonly. Updated by user
	UpdatedBy *Refer `protobuf:"bytes,13,opt,name=updated_by,json=updatedBy,proto3" json:"updated_by,omitempty"`
}

func (x *Bot) Reset() {
	*x = Bot{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Bot) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Bot) ProtoMessage() {}

func (x *Bot) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Bot.ProtoReflect.Descriptor instead.
func (*Bot) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{2}
}

func (x *Bot) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Bot) GetDc() *Refer {
	if x != nil {
		return x.Dc
	}
	return nil
}

func (x *Bot) GetUri() string {
	if x != nil {
		return x.Uri
	}
	return ""
}

func (x *Bot) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Bot) GetFlow() *Refer {
	if x != nil {
		return x.Flow
	}
	return nil
}

func (x *Bot) GetEnabled() bool {
	if x != nil {
		return x.Enabled
	}
	return false
}

func (x *Bot) GetProvider() string {
	if x != nil {
		return x.Provider
	}
	return ""
}

func (x *Bot) GetMetadata() map[string]string {
	if x != nil {
		return x.Metadata
	}
	return nil
}

func (x *Bot) GetUpdates() *ChatUpdates {
	if x != nil {
		return x.Updates
	}
	return nil
}

func (x *Bot) GetCreatedAt() int64 {
	if x != nil {
		return x.CreatedAt
	}
	return 0
}

func (x *Bot) GetCreatedBy() *Refer {
	if x != nil {
		return x.CreatedBy
	}
	return nil
}

func (x *Bot) GetUpdatedAt() int64 {
	if x != nil {
		return x.UpdatedAt
	}
	return 0
}

func (x *Bot) GetUpdatedBy() *Refer {
	if x != nil {
		return x.UpdatedBy
	}
	return nil
}

type SearchBotRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// ----- Base Filters ---------------------------
	// Selection by unique IDentifier(s)
	Id []int64 `protobuf:"varint,1,rep,packed,name=id,proto3" json:"id,omitempty"` // by id(s)
	// Selection by [D]omain [C]omponent IDentifier
	Dc int64 `protobuf:"varint,2,opt,name=dc,proto3" json:"dc,omitempty"`
	// Selection by general search term
	Q string `protobuf:"bytes,3,opt,name=q,proto3" json:"q,omitempty"`
	// ----- Object-Specific Filters ------------------
	// Selection by caseExactSubstringsMatch relative URI component
	Uri string `protobuf:"bytes,4,opt,name=uri,proto3" json:"uri,omitempty"` // caseExactSubstringsMatch
	// Selection by caseIgnoreSubstringsMatch chat bot name
	Name string `protobuf:"bytes,5,opt,name=name,proto3" json:"name,omitempty"` // caseIgnoreSubstringsMatch
	// Selection by flow schema IDentifier
	Flow int64 `protobuf:"varint,6,opt,name=flow,proto3" json:"flow,omitempty"`
	// Selection by caseExactStringMatch service provider's type name
	Provider []string `protobuf:"bytes,7,rep,name=provider,proto3" json:"provider,omitempty"` // caseIgnoreStringMatch
	// ----- Search Options -------------------------
	Fields []string `protobuf:"bytes,10,rep,name=fields,proto3" json:"fields,omitempty"` // select: output (fields,...)
	Sort   []string `protobuf:"bytes,11,rep,name=sort,proto3" json:"sort,omitempty"`     // select: order by (fields,...)
	Page   int32    `protobuf:"varint,12,opt,name=page,proto3" json:"page,omitempty"`    // select: offset {page}
	Size   int32    `protobuf:"varint,13,opt,name=size,proto3" json:"size,omitempty"`    // select: limit {size}
}

func (x *SearchBotRequest) Reset() {
	*x = SearchBotRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SearchBotRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchBotRequest) ProtoMessage() {}

func (x *SearchBotRequest) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchBotRequest.ProtoReflect.Descriptor instead.
func (*SearchBotRequest) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{3}
}

func (x *SearchBotRequest) GetId() []int64 {
	if x != nil {
		return x.Id
	}
	return nil
}

func (x *SearchBotRequest) GetDc() int64 {
	if x != nil {
		return x.Dc
	}
	return 0
}

func (x *SearchBotRequest) GetQ() string {
	if x != nil {
		return x.Q
	}
	return ""
}

func (x *SearchBotRequest) GetUri() string {
	if x != nil {
		return x.Uri
	}
	return ""
}

func (x *SearchBotRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *SearchBotRequest) GetFlow() int64 {
	if x != nil {
		return x.Flow
	}
	return 0
}

func (x *SearchBotRequest) GetProvider() []string {
	if x != nil {
		return x.Provider
	}
	return nil
}

func (x *SearchBotRequest) GetFields() []string {
	if x != nil {
		return x.Fields
	}
	return nil
}

func (x *SearchBotRequest) GetSort() []string {
	if x != nil {
		return x.Sort
	}
	return nil
}

func (x *SearchBotRequest) GetPage() int32 {
	if x != nil {
		return x.Page
	}
	return 0
}

func (x *SearchBotRequest) GetSize() int32 {
	if x != nil {
		return x.Size
	}
	return 0
}

type SearchBotResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Current page number
	Page int32 `protobuf:"varint,1,opt,name=page,proto3" json:"page,omitempty"` // {page} current number !
	// Next indicates whether there are more result page(s)
	Next bool `protobuf:"varint,2,opt,name=next,proto3" json:"next,omitempty"`
	// Items page results
	Items []*Bot `protobuf:"bytes,3,rep,name=items,proto3" json:"items,omitempty"`
}

func (x *SearchBotResponse) Reset() {
	*x = SearchBotResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SearchBotResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SearchBotResponse) ProtoMessage() {}

func (x *SearchBotResponse) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SearchBotResponse.ProtoReflect.Descriptor instead.
func (*SearchBotResponse) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{4}
}

func (x *SearchBotResponse) GetPage() int32 {
	if x != nil {
		return x.Page
	}
	return 0
}

func (x *SearchBotResponse) GetNext() bool {
	if x != nil {
		return x.Next
	}
	return false
}

func (x *SearchBotResponse) GetItems() []*Bot {
	if x != nil {
		return x.Items
	}
	return nil
}

type SelectBotRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Unique Bot IDentifier to lookup for
	Id int64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Unique Bot service relative URI
	Uri string `protobuf:"bytes,2,opt,name=uri,proto3" json:"uri,omitempty"`
	// Fields to be returned
	Fields []string `protobuf:"bytes,3,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *SelectBotRequest) Reset() {
	*x = SelectBotRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SelectBotRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SelectBotRequest) ProtoMessage() {}

func (x *SelectBotRequest) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SelectBotRequest.ProtoReflect.Descriptor instead.
func (*SelectBotRequest) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{5}
}

func (x *SelectBotRequest) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *SelectBotRequest) GetUri() string {
	if x != nil {
		return x.Uri
	}
	return ""
}

func (x *SelectBotRequest) GetFields() []string {
	if x != nil {
		return x.Fields
	}
	return nil
}

type UpdateBotRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// New Bot revision
	Bot *Bot `protobuf:"bytes,1,opt,name=bot,proto3" json:"bot,omitempty"`
	// Fields for partial update. PATCH
	Fields []string `protobuf:"bytes,2,rep,name=fields,proto3" json:"fields,omitempty"`
}

func (x *UpdateBotRequest) Reset() {
	*x = UpdateBotRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateBotRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateBotRequest) ProtoMessage() {}

func (x *UpdateBotRequest) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateBotRequest.ProtoReflect.Descriptor instead.
func (*UpdateBotRequest) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{6}
}

func (x *UpdateBotRequest) GetBot() *Bot {
	if x != nil {
		return x.Bot
	}
	return nil
}

func (x *UpdateBotRequest) GetFields() []string {
	if x != nil {
		return x.Fields
	}
	return nil
}

type SendMessageRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// recepient identification ...
	ExternalUserId string `protobuf:"bytes,1,opt,name=external_user_id,json=externalUserId,proto3" json:"external_user_id,omitempty"`
	ProfileId      int64  `protobuf:"varint,2,opt,name=profile_id,json=profileId,proto3" json:"profile_id,omitempty"`
	// int64 conversation_id = 3;
	Message *chat.Message `protobuf:"bytes,4,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *SendMessageRequest) Reset() {
	*x = SendMessageRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendMessageRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendMessageRequest) ProtoMessage() {}

func (x *SendMessageRequest) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendMessageRequest.ProtoReflect.Descriptor instead.
func (*SendMessageRequest) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{7}
}

func (x *SendMessageRequest) GetExternalUserId() string {
	if x != nil {
		return x.ExternalUserId
	}
	return ""
}

func (x *SendMessageRequest) GetProfileId() int64 {
	if x != nil {
		return x.ProfileId
	}
	return 0
}

func (x *SendMessageRequest) GetMessage() *chat.Message {
	if x != nil {
		return x.Message
	}
	return nil
}

type SendMessageResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// // webitel.chat.server.Error error = 1;
	// webitel.chat.server.Message message = 1;
	Bindings map[string]string `protobuf:"bytes,1,rep,name=bindings,proto3" json:"bindings,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"` // SENT message binding variables
}

func (x *SendMessageResponse) Reset() {
	*x = SendMessageResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_bot_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendMessageResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendMessageResponse) ProtoMessage() {}

func (x *SendMessageResponse) ProtoReflect() protoreflect.Message {
	mi := &file_bot_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendMessageResponse.ProtoReflect.Descriptor instead.
func (*SendMessageResponse) Descriptor() ([]byte, []int) {
	return file_bot_proto_rawDescGZIP(), []int{8}
}

func (x *SendMessageResponse) GetBindings() map[string]string {
	if x != nil {
		return x.Bindings
	}
	return nil
}

var File_bot_proto protoreflect.FileDescriptor

var file_bot_proto_rawDesc = []byte{
	0x0a, 0x09, 0x62, 0x6f, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x77, 0x65, 0x62,
	0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x1a, 0x0d, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x2b, 0x0a, 0x05,
	0x52, 0x65, 0x66, 0x65, 0x72, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x61, 0x0a, 0x0b, 0x43, 0x68, 0x61,
	0x74, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x73, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x69, 0x74, 0x6c,
	0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x69, 0x74, 0x6c, 0x65, 0x12, 0x14,
	0x0a, 0x05, 0x63, 0x6c, 0x6f, 0x73, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x63,
	0x6c, 0x6f, 0x73, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6a, 0x6f, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x04, 0x6a, 0x6f, 0x69, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x6c, 0x65, 0x66, 0x74,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6c, 0x65, 0x66, 0x74, 0x22, 0xac, 0x04, 0x0a,
	0x03, 0x42, 0x6f, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x02, 0x69, 0x64, 0x12, 0x27, 0x0a, 0x02, 0x64, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x17, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e,
	0x62, 0x6f, 0x74, 0x2e, 0x52, 0x65, 0x66, 0x65, 0x72, 0x52, 0x02, 0x64, 0x63, 0x12, 0x10, 0x0a,
	0x03, 0x75, 0x72, 0x69, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x69, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x12, 0x2b, 0x0a, 0x04, 0x66, 0x6c, 0x6f, 0x77, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x17, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74,
	0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x52, 0x65, 0x66, 0x65, 0x72, 0x52, 0x04, 0x66, 0x6c, 0x6f, 0x77,
	0x12, 0x18, 0x0a, 0x07, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x07, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x64, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72,
	0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x70, 0x72,
	0x6f, 0x76, 0x69, 0x64, 0x65, 0x72, 0x12, 0x3f, 0x0a, 0x08, 0x6d, 0x65, 0x74, 0x61, 0x64, 0x61,
	0x74, 0x61, 0x18, 0x08, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74,
	0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x42, 0x6f, 0x74, 0x2e,
	0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x6d,
	0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x12, 0x37, 0x0a, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x73, 0x18, 0x09, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74,
	0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x43, 0x68, 0x61, 0x74,
	0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x73, 0x52, 0x07, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x73,
	0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x0a,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x12,
	0x36, 0x0a, 0x0a, 0x63, 0x72, 0x65, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x62, 0x79, 0x18, 0x0b, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68,
	0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x52, 0x65, 0x66, 0x65, 0x72, 0x52, 0x09, 0x63, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x64, 0x42, 0x79, 0x12, 0x1d, 0x0a, 0x0a, 0x75, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x64, 0x5f, 0x61, 0x74, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x75, 0x70, 0x64,
	0x61, 0x74, 0x65, 0x64, 0x41, 0x74, 0x12, 0x36, 0x0a, 0x0a, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65,
	0x64, 0x5f, 0x62, 0x79, 0x18, 0x0d, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x77, 0x65, 0x62,
	0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x52, 0x65,
	0x66, 0x65, 0x72, 0x52, 0x09, 0x75, 0x70, 0x64, 0x61, 0x74, 0x65, 0x64, 0x42, 0x79, 0x1a, 0x3b,
	0x0a, 0x0d, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0xea, 0x01, 0x0a, 0x10,
	0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x03, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64,
	0x12, 0x0e, 0x0a, 0x02, 0x64, 0x63, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x64, 0x63,
	0x12, 0x0c, 0x0a, 0x01, 0x71, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x01, 0x71, 0x12, 0x10,
	0x0a, 0x03, 0x75, 0x72, 0x69, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x69,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x66, 0x6c, 0x6f, 0x77, 0x18, 0x06, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x04, 0x66, 0x6c, 0x6f, 0x77, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x18, 0x07, 0x20, 0x03, 0x28, 0x09, 0x52, 0x08, 0x70, 0x72, 0x6f, 0x76,
	0x69, 0x64, 0x65, 0x72, 0x12, 0x16, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x0a,
	0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x12, 0x12, 0x0a, 0x04,
	0x73, 0x6f, 0x72, 0x74, 0x18, 0x0b, 0x20, 0x03, 0x28, 0x09, 0x52, 0x04, 0x73, 0x6f, 0x72, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x0c, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04,
	0x70, 0x61, 0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x0d, 0x20, 0x01,
	0x28, 0x05, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x22, 0x68, 0x0a, 0x11, 0x53, 0x65, 0x61, 0x72,
	0x63, 0x68, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x70, 0x61, 0x67, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x04, 0x70, 0x61, 0x67,
	0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x04, 0x6e, 0x65, 0x78, 0x74, 0x12, 0x2b, 0x0a, 0x05, 0x69, 0x74, 0x65, 0x6d, 0x73, 0x18, 0x03,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x15, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63,
	0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x42, 0x6f, 0x74, 0x52, 0x05, 0x69, 0x74, 0x65,
	0x6d, 0x73, 0x22, 0x4c, 0x0a, 0x10, 0x53, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x42, 0x6f, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x69, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x69, 0x12, 0x16, 0x0a, 0x06, 0x66, 0x69, 0x65, 0x6c,
	0x64, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73,
	0x22, 0x53, 0x0a, 0x10, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x12, 0x27, 0x0a, 0x03, 0x62, 0x6f, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x15, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74,
	0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x42, 0x6f, 0x74, 0x52, 0x03, 0x62, 0x6f, 0x74, 0x12, 0x16, 0x0a,
	0x06, 0x66, 0x69, 0x65, 0x6c, 0x64, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x06, 0x66,
	0x69, 0x65, 0x6c, 0x64, 0x73, 0x22, 0x95, 0x01, 0x0a, 0x12, 0x53, 0x65, 0x6e, 0x64, 0x4d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x28, 0x0a, 0x10,
	0x65, 0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c, 0x5f, 0x75, 0x73, 0x65, 0x72, 0x5f, 0x69, 0x64,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x65, 0x78, 0x74, 0x65, 0x72, 0x6e, 0x61, 0x6c,
	0x55, 0x73, 0x65, 0x72, 0x49, 0x64, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x72, 0x6f, 0x66, 0x69, 0x6c,
	0x65, 0x5f, 0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x70, 0x72, 0x6f, 0x66,
	0x69, 0x6c, 0x65, 0x49, 0x64, 0x12, 0x36, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c,
	0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x2e, 0x4d, 0x65, 0x73,
	0x73, 0x61, 0x67, 0x65, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x22, 0xa3, 0x01,
	0x0a, 0x13, 0x53, 0x65, 0x6e, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x4f, 0x0a, 0x08, 0x62, 0x69, 0x6e, 0x64, 0x69, 0x6e, 0x67,
	0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65,
	0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x4d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x42,
	0x69, 0x6e, 0x64, 0x69, 0x6e, 0x67, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x08, 0x62, 0x69,
	0x6e, 0x64, 0x69, 0x6e, 0x67, 0x73, 0x1a, 0x3b, 0x0a, 0x0d, 0x42, 0x69, 0x6e, 0x64, 0x69, 0x6e,
	0x67, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x32, 0xe5, 0x03, 0x0a, 0x04, 0x42, 0x6f, 0x74, 0x73, 0x12, 0x5c, 0x0a, 0x0b,
	0x53, 0x65, 0x6e, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x24, 0x2e, 0x77, 0x65,
	0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x53,
	0x65, 0x6e, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x25, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74,
	0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3b, 0x0a, 0x09, 0x43, 0x72,
	0x65, 0x61, 0x74, 0x65, 0x42, 0x6f, 0x74, 0x12, 0x15, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65,
	0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x42, 0x6f, 0x74, 0x1a, 0x15,
	0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f,
	0x74, 0x2e, 0x42, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x48, 0x0a, 0x09, 0x53, 0x65, 0x6c, 0x65, 0x63,
	0x74, 0x42, 0x6f, 0x74, 0x12, 0x22, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63,
	0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x53, 0x65, 0x6c, 0x65, 0x63, 0x74, 0x42, 0x6f,
	0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74,
	0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x42, 0x6f, 0x74, 0x22,
	0x00, 0x12, 0x48, 0x0a, 0x09, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x42, 0x6f, 0x74, 0x12, 0x22,
	0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f,
	0x74, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x1a, 0x15, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61,
	0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x42, 0x6f, 0x74, 0x22, 0x00, 0x12, 0x56, 0x0a, 0x09, 0x44,
	0x65, 0x6c, 0x65, 0x74, 0x65, 0x42, 0x6f, 0x74, 0x12, 0x22, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74,
	0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x53, 0x65, 0x61, 0x72,
	0x63, 0x68, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x23, 0x2e, 0x77,
	0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e,
	0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x22, 0x00, 0x12, 0x56, 0x0a, 0x09, 0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x42, 0x6f, 0x74,
	0x12, 0x22, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e,
	0x62, 0x6f, 0x74, 0x2e, 0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x42, 0x6f, 0x74, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x23, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63,
	0x68, 0x61, 0x74, 0x2e, 0x62, 0x6f, 0x74, 0x2e, 0x53, 0x65, 0x61, 0x72, 0x63, 0x68, 0x42, 0x6f,
	0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x2f, 0x5a, 0x2d, 0x67,
	0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65,
	0x6c, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x2f, 0x61,
	0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x6f, 0x74, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_bot_proto_rawDescOnce sync.Once
	file_bot_proto_rawDescData = file_bot_proto_rawDesc
)

func file_bot_proto_rawDescGZIP() []byte {
	file_bot_proto_rawDescOnce.Do(func() {
		file_bot_proto_rawDescData = protoimpl.X.CompressGZIP(file_bot_proto_rawDescData)
	})
	return file_bot_proto_rawDescData
}

var file_bot_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_bot_proto_goTypes = []interface{}{
	(*Refer)(nil),               // 0: webitel.chat.bot.Refer
	(*ChatUpdates)(nil),         // 1: webitel.chat.bot.ChatUpdates
	(*Bot)(nil),                 // 2: webitel.chat.bot.Bot
	(*SearchBotRequest)(nil),    // 3: webitel.chat.bot.SearchBotRequest
	(*SearchBotResponse)(nil),   // 4: webitel.chat.bot.SearchBotResponse
	(*SelectBotRequest)(nil),    // 5: webitel.chat.bot.SelectBotRequest
	(*UpdateBotRequest)(nil),    // 6: webitel.chat.bot.UpdateBotRequest
	(*SendMessageRequest)(nil),  // 7: webitel.chat.bot.SendMessageRequest
	(*SendMessageResponse)(nil), // 8: webitel.chat.bot.SendMessageResponse
	nil,                         // 9: webitel.chat.bot.Bot.MetadataEntry
	nil,                         // 10: webitel.chat.bot.SendMessageResponse.BindingsEntry
	(*chat.Message)(nil),        // 11: webitel.chat.server.Message
}
var file_bot_proto_depIdxs = []int32{
	0,  // 0: webitel.chat.bot.Bot.dc:type_name -> webitel.chat.bot.Refer
	0,  // 1: webitel.chat.bot.Bot.flow:type_name -> webitel.chat.bot.Refer
	9,  // 2: webitel.chat.bot.Bot.metadata:type_name -> webitel.chat.bot.Bot.MetadataEntry
	1,  // 3: webitel.chat.bot.Bot.updates:type_name -> webitel.chat.bot.ChatUpdates
	0,  // 4: webitel.chat.bot.Bot.created_by:type_name -> webitel.chat.bot.Refer
	0,  // 5: webitel.chat.bot.Bot.updated_by:type_name -> webitel.chat.bot.Refer
	2,  // 6: webitel.chat.bot.SearchBotResponse.items:type_name -> webitel.chat.bot.Bot
	2,  // 7: webitel.chat.bot.UpdateBotRequest.bot:type_name -> webitel.chat.bot.Bot
	11, // 8: webitel.chat.bot.SendMessageRequest.message:type_name -> webitel.chat.server.Message
	10, // 9: webitel.chat.bot.SendMessageResponse.bindings:type_name -> webitel.chat.bot.SendMessageResponse.BindingsEntry
	7,  // 10: webitel.chat.bot.Bots.SendMessage:input_type -> webitel.chat.bot.SendMessageRequest
	2,  // 11: webitel.chat.bot.Bots.CreateBot:input_type -> webitel.chat.bot.Bot
	5,  // 12: webitel.chat.bot.Bots.SelectBot:input_type -> webitel.chat.bot.SelectBotRequest
	6,  // 13: webitel.chat.bot.Bots.UpdateBot:input_type -> webitel.chat.bot.UpdateBotRequest
	3,  // 14: webitel.chat.bot.Bots.DeleteBot:input_type -> webitel.chat.bot.SearchBotRequest
	3,  // 15: webitel.chat.bot.Bots.SearchBot:input_type -> webitel.chat.bot.SearchBotRequest
	8,  // 16: webitel.chat.bot.Bots.SendMessage:output_type -> webitel.chat.bot.SendMessageResponse
	2,  // 17: webitel.chat.bot.Bots.CreateBot:output_type -> webitel.chat.bot.Bot
	2,  // 18: webitel.chat.bot.Bots.SelectBot:output_type -> webitel.chat.bot.Bot
	2,  // 19: webitel.chat.bot.Bots.UpdateBot:output_type -> webitel.chat.bot.Bot
	4,  // 20: webitel.chat.bot.Bots.DeleteBot:output_type -> webitel.chat.bot.SearchBotResponse
	4,  // 21: webitel.chat.bot.Bots.SearchBot:output_type -> webitel.chat.bot.SearchBotResponse
	16, // [16:22] is the sub-list for method output_type
	10, // [10:16] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_bot_proto_init() }
func file_bot_proto_init() {
	if File_bot_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_bot_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Refer); i {
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
		file_bot_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChatUpdates); i {
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
		file_bot_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Bot); i {
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
		file_bot_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SearchBotRequest); i {
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
		file_bot_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SearchBotResponse); i {
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
		file_bot_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SelectBotRequest); i {
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
		file_bot_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateBotRequest); i {
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
		file_bot_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendMessageRequest); i {
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
		file_bot_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendMessageResponse); i {
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
			RawDescriptor: file_bot_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_bot_proto_goTypes,
		DependencyIndexes: file_bot_proto_depIdxs,
		MessageInfos:      file_bot_proto_msgTypes,
	}.Build()
	File_bot_proto = out.File
	file_bot_proto_rawDesc = nil
	file_bot_proto_goTypes = nil
	file_bot_proto_depIdxs = nil
}
