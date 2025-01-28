// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.35.2
// 	protoc        v5.29.0
// source: chat/messages/message.proto

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

// Type of request to share contact info
type Button_Request int32

const (
	// Phone Number
	Button_phone Button_Request = 0
	// Email Address
	Button_email Button_Request = 1
	// General Form
	Button_contact Button_Request = 2
	// Current Location
	Button_location Button_Request = 3
)

// Enum value maps for Button_Request.
var (
	Button_Request_name = map[int32]string{
		0: "phone",
		1: "email",
		2: "contact",
		3: "location",
	}
	Button_Request_value = map[string]int32{
		"phone":    0,
		"email":    1,
		"contact":  2,
		"location": 3,
	}
)

func (x Button_Request) Enum() *Button_Request {
	p := new(Button_Request)
	*p = x
	return p
}

func (x Button_Request) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Button_Request) Descriptor() protoreflect.EnumDescriptor {
	return file_chat_messages_message_proto_enumTypes[0].Descriptor()
}

func (Button_Request) Type() protoreflect.EnumType {
	return &file_chat_messages_message_proto_enumTypes[0]
}

func (x Button_Request) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Button_Request.Descriptor instead.
func (Button_Request) EnumDescriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{4, 0}
}

// Chat Message.
type Message struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Unique message identifier inside this chat.
	Id int64 `protobuf:"varint,1,opt,name=id,proto3" json:"id,omitempty"`
	// Timestamp when this message was sent (published).
	Date int64 `protobuf:"varint,2,opt,name=date,proto3" json:"date,omitempty"`
	// Sender of the message.
	From *Peer `protobuf:"bytes,3,opt,name=from,proto3" json:"from,omitempty"`
	// Conversation the message belongs to ..
	Chat *Chat `protobuf:"bytes,4,opt,name=chat,proto3" json:"chat,omitempty"`
	// Chat Sender of the message, sent on behalf of a chat (member).
	Sender *Chat `protobuf:"bytes,5,opt,name=sender,proto3" json:"sender,omitempty"`
	// Timestamp when this message was last edited.
	Edit int64 `protobuf:"varint,6,opt,name=edit,proto3" json:"edit,omitempty"`
	// Message Text.
	Text string `protobuf:"bytes,7,opt,name=text,proto3" json:"text,omitempty"`
	// Message Media. Attachment.
	File *File `protobuf:"bytes,8,opt,name=file,proto3" json:"file,omitempty"`
	// Context. Variables. Environment.
	Context map[string]string `protobuf:"bytes,9,rep,name=context,proto3" json:"context,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	// Keyboard. Buttons. Quick Replies.
	Keyboard *ReplyMarkup `protobuf:"bytes,10,opt,name=keyboard,proto3" json:"keyboard,omitempty"`
	// Postback. Reply Button Click[ed].
	Postback *Postback `protobuf:"bytes,11,opt,name=postback,proto3" json:"postback,omitempty"`
}

func (x *Message) Reset() {
	*x = Message{}
	mi := &file_chat_messages_message_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Message) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Message) ProtoMessage() {}

func (x *Message) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Message.ProtoReflect.Descriptor instead.
func (*Message) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{0}
}

func (x *Message) GetId() int64 {
	if x != nil {
		return x.Id
	}
	return 0
}

func (x *Message) GetDate() int64 {
	if x != nil {
		return x.Date
	}
	return 0
}

func (x *Message) GetFrom() *Peer {
	if x != nil {
		return x.From
	}
	return nil
}

func (x *Message) GetChat() *Chat {
	if x != nil {
		return x.Chat
	}
	return nil
}

func (x *Message) GetSender() *Chat {
	if x != nil {
		return x.Sender
	}
	return nil
}

func (x *Message) GetEdit() int64 {
	if x != nil {
		return x.Edit
	}
	return 0
}

func (x *Message) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

func (x *Message) GetFile() *File {
	if x != nil {
		return x.File
	}
	return nil
}

func (x *Message) GetContext() map[string]string {
	if x != nil {
		return x.Context
	}
	return nil
}

func (x *Message) GetKeyboard() *ReplyMarkup {
	if x != nil {
		return x.Keyboard
	}
	return nil
}

func (x *Message) GetPostback() *Postback {
	if x != nil {
		return x.Postback
	}
	return nil
}

// Media File.
type File struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// File location
	Id string `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	// Size in bytes
	Size int64 `protobuf:"varint,3,opt,name=size,proto3" json:"size,omitempty"`
	// MIME media type
	Type string `protobuf:"bytes,4,opt,name=type,proto3" json:"type,omitempty"`
	// Filename
	Name string `protobuf:"bytes,5,opt,name=name,proto3" json:"name,omitempty"`
	// File url (optional)
	Url string `protobuf:"bytes,6,opt,name=url,proto3" json:"url,omitempty"`
}

func (x *File) Reset() {
	*x = File{}
	mi := &file_chat_messages_message_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *File) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*File) ProtoMessage() {}

func (x *File) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use File.ProtoReflect.Descriptor instead.
func (*File) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{1}
}

func (x *File) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *File) GetSize() int64 {
	if x != nil {
		return x.Size
	}
	return 0
}

func (x *File) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *File) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *File) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

type ReplyMarkup struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// An option used to block input to force
	// the user to respond with one of the buttons.
	NoInput bool `protobuf:"varint,2,opt,name=no_input,json=noInput,proto3" json:"no_input,omitempty"`
	// Markup of button(s)
	Buttons []*ButtonRow `protobuf:"bytes,1,rep,name=buttons,proto3" json:"buttons,omitempty"`
}

func (x *ReplyMarkup) Reset() {
	*x = ReplyMarkup{}
	mi := &file_chat_messages_message_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ReplyMarkup) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ReplyMarkup) ProtoMessage() {}

func (x *ReplyMarkup) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ReplyMarkup.ProtoReflect.Descriptor instead.
func (*ReplyMarkup) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{2}
}

func (x *ReplyMarkup) GetNoInput() bool {
	if x != nil {
		return x.NoInput
	}
	return false
}

func (x *ReplyMarkup) GetButtons() []*ButtonRow {
	if x != nil {
		return x.Buttons
	}
	return nil
}

type ButtonRow struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Button(s) in a row
	Row []*Button `protobuf:"bytes,1,rep,name=row,proto3" json:"row,omitempty"`
}

func (x *ButtonRow) Reset() {
	*x = ButtonRow{}
	mi := &file_chat_messages_message_proto_msgTypes[3]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *ButtonRow) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ButtonRow) ProtoMessage() {}

func (x *ButtonRow) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[3]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ButtonRow.ProtoReflect.Descriptor instead.
func (*ButtonRow) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{3}
}

func (x *ButtonRow) GetRow() []*Button {
	if x != nil {
		return x.Row
	}
	return nil
}

type Button struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Caption to display.
	Text string `protobuf:"bytes,1,opt,name=text,proto3" json:"text,omitempty"`
	// Type of the button.
	//
	// Types that are assignable to Type:
	//
	//	*Button_Url
	//	*Button_Code
	//	*Button_Share
	Type isButton_Type `protobuf_oneof:"type"`
}

func (x *Button) Reset() {
	*x = Button{}
	mi := &file_chat_messages_message_proto_msgTypes[4]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Button) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Button) ProtoMessage() {}

func (x *Button) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[4]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Button.ProtoReflect.Descriptor instead.
func (*Button) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{4}
}

func (x *Button) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

func (m *Button) GetType() isButton_Type {
	if m != nil {
		return m.Type
	}
	return nil
}

func (x *Button) GetUrl() string {
	if x, ok := x.GetType().(*Button_Url); ok {
		return x.Url
	}
	return ""
}

func (x *Button) GetCode() string {
	if x, ok := x.GetType().(*Button_Code); ok {
		return x.Code
	}
	return ""
}

func (x *Button) GetShare() Button_Request {
	if x, ok := x.GetType().(*Button_Share); ok {
		return x.Share
	}
	return Button_phone
}

type isButton_Type interface {
	isButton_Type()
}

type Button_Url struct {
	// URL to navigate to ..
	Url string `protobuf:"bytes,2,opt,name=url,proto3,oneof"`
}

type Button_Code struct {
	// Postback/Callback data.
	Code string `protobuf:"bytes,3,opt,name=code,proto3,oneof"`
}

type Button_Share struct {
	// Request to share contact info.
	Share Button_Request `protobuf:"varint,4,opt,name=share,proto3,enum=webitel.chat.Button_Request,oneof"`
}

func (*Button_Url) isButton_Type() {}

func (*Button_Code) isButton_Type() {}

func (*Button_Share) isButton_Type() {}

// Postback. Reply Button Click[ed].
type Postback struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Message ID of the button.
	Mid int64 `protobuf:"varint,1,opt,name=mid,proto3" json:"mid,omitempty"`
	// Data associated with the Button.
	Code string `protobuf:"bytes,2,opt,name=code,proto3" json:"code,omitempty"`
	// Button's display caption.
	Text string `protobuf:"bytes,3,opt,name=text,proto3" json:"text,omitempty"`
}

func (x *Postback) Reset() {
	*x = Postback{}
	mi := &file_chat_messages_message_proto_msgTypes[5]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Postback) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Postback) ProtoMessage() {}

func (x *Postback) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[5]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Postback.ProtoReflect.Descriptor instead.
func (*Postback) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{5}
}

func (x *Postback) GetMid() int64 {
	if x != nil {
		return x.Mid
	}
	return 0
}

func (x *Postback) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

func (x *Postback) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

type InputMessage struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Text     string         `protobuf:"bytes,1,opt,name=text,proto3" json:"text,omitempty"`
	File     *InputFile     `protobuf:"bytes,2,opt,name=file,proto3" json:"file,omitempty"`
	Keyboard *InputKeyboard `protobuf:"bytes,3,opt,name=keyboard,proto3" json:"keyboard,omitempty"`
}

func (x *InputMessage) Reset() {
	*x = InputMessage{}
	mi := &file_chat_messages_message_proto_msgTypes[6]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InputMessage) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputMessage) ProtoMessage() {}

func (x *InputMessage) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[6]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputMessage.ProtoReflect.Descriptor instead.
func (*InputMessage) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{6}
}

func (x *InputMessage) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

func (x *InputMessage) GetFile() *InputFile {
	if x != nil {
		return x.File
	}
	return nil
}

func (x *InputMessage) GetKeyboard() *InputKeyboard {
	if x != nil {
		return x.Keyboard
	}
	return nil
}

type InputFile struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to FileSource:
	//
	//	*InputFile_Id
	//	*InputFile_Url
	FileSource isInputFile_FileSource `protobuf_oneof:"file_source"`
}

func (x *InputFile) Reset() {
	*x = InputFile{}
	mi := &file_chat_messages_message_proto_msgTypes[7]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InputFile) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputFile) ProtoMessage() {}

func (x *InputFile) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[7]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputFile.ProtoReflect.Descriptor instead.
func (*InputFile) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{7}
}

func (m *InputFile) GetFileSource() isInputFile_FileSource {
	if m != nil {
		return m.FileSource
	}
	return nil
}

func (x *InputFile) GetId() string {
	if x, ok := x.GetFileSource().(*InputFile_Id); ok {
		return x.Id
	}
	return ""
}

func (x *InputFile) GetUrl() string {
	if x, ok := x.GetFileSource().(*InputFile_Url); ok {
		return x.Url
	}
	return ""
}

type isInputFile_FileSource interface {
	isInputFile_FileSource()
}

type InputFile_Id struct {
	Id string `protobuf:"bytes,1,opt,name=id,proto3,oneof"`
}

type InputFile_Url struct {
	Url string `protobuf:"bytes,2,opt,name=url,proto3,oneof"`
}

func (*InputFile_Id) isInputFile_FileSource() {}

func (*InputFile_Url) isInputFile_FileSource() {}

type InputKeyboard struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Rows []*InputButtonRow `protobuf:"bytes,1,rep,name=rows,proto3" json:"rows,omitempty"`
}

func (x *InputKeyboard) Reset() {
	*x = InputKeyboard{}
	mi := &file_chat_messages_message_proto_msgTypes[8]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InputKeyboard) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputKeyboard) ProtoMessage() {}

func (x *InputKeyboard) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[8]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputKeyboard.ProtoReflect.Descriptor instead.
func (*InputKeyboard) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{8}
}

func (x *InputKeyboard) GetRows() []*InputButtonRow {
	if x != nil {
		return x.Rows
	}
	return nil
}

type InputButtonRow struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Buttons []*InputButton `protobuf:"bytes,1,rep,name=buttons,proto3" json:"buttons,omitempty"`
}

func (x *InputButtonRow) Reset() {
	*x = InputButtonRow{}
	mi := &file_chat_messages_message_proto_msgTypes[9]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InputButtonRow) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputButtonRow) ProtoMessage() {}

func (x *InputButtonRow) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[9]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputButtonRow.ProtoReflect.Descriptor instead.
func (*InputButtonRow) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{9}
}

func (x *InputButtonRow) GetButtons() []*InputButton {
	if x != nil {
		return x.Buttons
	}
	return nil
}

type InputButton struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Caption string `protobuf:"bytes,1,opt,name=caption,proto3" json:"caption,omitempty"`
	Text    string `protobuf:"bytes,2,opt,name=text,proto3" json:"text,omitempty"`
	Type    string `protobuf:"bytes,3,opt,name=type,proto3" json:"type,omitempty"`
	Url     string `protobuf:"bytes,4,opt,name=url,proto3" json:"url,omitempty"`
	Code    string `protobuf:"bytes,5,opt,name=code,proto3" json:"code,omitempty"`
}

func (x *InputButton) Reset() {
	*x = InputButton{}
	mi := &file_chat_messages_message_proto_msgTypes[10]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *InputButton) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*InputButton) ProtoMessage() {}

func (x *InputButton) ProtoReflect() protoreflect.Message {
	mi := &file_chat_messages_message_proto_msgTypes[10]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use InputButton.ProtoReflect.Descriptor instead.
func (*InputButton) Descriptor() ([]byte, []int) {
	return file_chat_messages_message_proto_rawDescGZIP(), []int{10}
}

func (x *InputButton) GetCaption() string {
	if x != nil {
		return x.Caption
	}
	return ""
}

func (x *InputButton) GetText() string {
	if x != nil {
		return x.Text
	}
	return ""
}

func (x *InputButton) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

func (x *InputButton) GetUrl() string {
	if x != nil {
		return x.Url
	}
	return ""
}

func (x *InputButton) GetCode() string {
	if x != nil {
		return x.Code
	}
	return ""
}

var File_chat_messages_message_proto protoreflect.FileDescriptor

var file_chat_messages_message_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0c, 0x77,
	0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x1a, 0x18, 0x63, 0x68, 0x61,
	0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x2f, 0x70, 0x65, 0x65, 0x72, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x18, 0x63, 0x68, 0x61, 0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x73, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22,
	0xde, 0x03, 0x0a, 0x07, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69,
	0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x64, 0x61, 0x74, 0x65, 0x12,
	0x26, 0x0a, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e,
	0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x50, 0x65, 0x65,
	0x72, 0x52, 0x04, 0x66, 0x72, 0x6f, 0x6d, 0x12, 0x26, 0x0a, 0x04, 0x63, 0x68, 0x61, 0x74, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e,
	0x63, 0x68, 0x61, 0x74, 0x2e, 0x43, 0x68, 0x61, 0x74, 0x52, 0x04, 0x63, 0x68, 0x61, 0x74, 0x12,
	0x2a, 0x0a, 0x06, 0x73, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x43,
	0x68, 0x61, 0x74, 0x52, 0x06, 0x73, 0x65, 0x6e, 0x64, 0x65, 0x72, 0x12, 0x12, 0x0a, 0x04, 0x65,
	0x64, 0x69, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x65, 0x64, 0x69, 0x74, 0x12,
	0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74,
	0x65, 0x78, 0x74, 0x12, 0x26, 0x0a, 0x04, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x12, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74,
	0x2e, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x04, 0x66, 0x69, 0x6c, 0x65, 0x12, 0x3c, 0x0a, 0x07, 0x63,
	0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x18, 0x09, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x22, 0x2e, 0x77,
	0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x4d, 0x65, 0x73, 0x73,
	0x61, 0x67, 0x65, 0x2e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79,
	0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x12, 0x35, 0x0a, 0x08, 0x6b, 0x65, 0x79,
	0x62, 0x6f, 0x61, 0x72, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x77, 0x65,
	0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x52, 0x65, 0x70, 0x6c, 0x79,
	0x4d, 0x61, 0x72, 0x6b, 0x75, 0x70, 0x52, 0x08, 0x6b, 0x65, 0x79, 0x62, 0x6f, 0x61, 0x72, 0x64,
	0x12, 0x32, 0x0a, 0x08, 0x70, 0x6f, 0x73, 0x74, 0x62, 0x61, 0x63, 0x6b, 0x18, 0x0b, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x16, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61,
	0x74, 0x2e, 0x50, 0x6f, 0x73, 0x74, 0x62, 0x61, 0x63, 0x6b, 0x52, 0x08, 0x70, 0x6f, 0x73, 0x74,
	0x62, 0x61, 0x63, 0x6b, 0x1a, 0x3a, 0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01,
	0x22, 0x64, 0x0a, 0x04, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x73, 0x69, 0x7a, 0x65,
	0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x12, 0x0a, 0x04,
	0x74, 0x79, 0x70, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x06, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x22, 0x5b, 0x0a, 0x0b, 0x52, 0x65, 0x70, 0x6c, 0x79, 0x4d,
	0x61, 0x72, 0x6b, 0x75, 0x70, 0x12, 0x19, 0x0a, 0x08, 0x6e, 0x6f, 0x5f, 0x69, 0x6e, 0x70, 0x75,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x6e, 0x6f, 0x49, 0x6e, 0x70, 0x75, 0x74,
	0x12, 0x31, 0x0a, 0x07, 0x62, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x17, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74,
	0x2e, 0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x52, 0x6f, 0x77, 0x52, 0x07, 0x62, 0x75, 0x74, 0x74,
	0x6f, 0x6e, 0x73, 0x22, 0x33, 0x0a, 0x09, 0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x52, 0x6f, 0x77,
	0x12, 0x26, 0x0a, 0x03, 0x72, 0x6f, 0x77, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x14, 0x2e,
	0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x42, 0x75, 0x74,
	0x74, 0x6f, 0x6e, 0x52, 0x03, 0x72, 0x6f, 0x77, 0x22, 0xc0, 0x01, 0x0a, 0x06, 0x42, 0x75, 0x74,
	0x74, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x04, 0x74, 0x65, 0x78, 0x74, 0x12, 0x12, 0x0a, 0x03, 0x75, 0x72, 0x6c, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x12, 0x14, 0x0a, 0x04, 0x63,
	0x6f, 0x64, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x04, 0x63, 0x6f, 0x64,
	0x65, 0x12, 0x34, 0x0a, 0x05, 0x73, 0x68, 0x61, 0x72, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x1c, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e,
	0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x48, 0x00,
	0x52, 0x05, 0x73, 0x68, 0x61, 0x72, 0x65, 0x22, 0x3a, 0x0a, 0x07, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x12, 0x09, 0x0a, 0x05, 0x70, 0x68, 0x6f, 0x6e, 0x65, 0x10, 0x00, 0x12, 0x09, 0x0a,
	0x05, 0x65, 0x6d, 0x61, 0x69, 0x6c, 0x10, 0x01, 0x12, 0x0b, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x74,
	0x61, 0x63, 0x74, 0x10, 0x02, 0x12, 0x0c, 0x0a, 0x08, 0x6c, 0x6f, 0x63, 0x61, 0x74, 0x69, 0x6f,
	0x6e, 0x10, 0x03, 0x42, 0x06, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0x44, 0x0a, 0x08, 0x50,
	0x6f, 0x73, 0x74, 0x62, 0x61, 0x63, 0x6b, 0x12, 0x10, 0x0a, 0x03, 0x6d, 0x69, 0x64, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x03, 0x6d, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x63, 0x6f, 0x64,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x12, 0x12, 0x0a,
	0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x65, 0x78,
	0x74, 0x22, 0x88, 0x01, 0x0a, 0x0c, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x4d, 0x65, 0x73, 0x73, 0x61,
	0x67, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x04, 0x74, 0x65, 0x78, 0x74, 0x12, 0x2b, 0x0a, 0x04, 0x66, 0x69, 0x6c, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x17, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63,
	0x68, 0x61, 0x74, 0x2e, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x46, 0x69, 0x6c, 0x65, 0x52, 0x04, 0x66,
	0x69, 0x6c, 0x65, 0x12, 0x37, 0x0a, 0x08, 0x6b, 0x65, 0x79, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1b, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e,
	0x63, 0x68, 0x61, 0x74, 0x2e, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x4b, 0x65, 0x79, 0x62, 0x6f, 0x61,
	0x72, 0x64, 0x52, 0x08, 0x6b, 0x65, 0x79, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x22, 0x40, 0x0a, 0x09,
	0x49, 0x6e, 0x70, 0x75, 0x74, 0x46, 0x69, 0x6c, 0x65, 0x12, 0x10, 0x0a, 0x02, 0x69, 0x64, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x02, 0x69, 0x64, 0x12, 0x12, 0x0a, 0x03, 0x75,
	0x72, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x42,
	0x0d, 0x0a, 0x0b, 0x66, 0x69, 0x6c, 0x65, 0x5f, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65, 0x22, 0x41,
	0x0a, 0x0d, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x4b, 0x65, 0x79, 0x62, 0x6f, 0x61, 0x72, 0x64, 0x12,
	0x30, 0x0a, 0x04, 0x72, 0x6f, 0x77, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1c, 0x2e,
	0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63, 0x68, 0x61, 0x74, 0x2e, 0x49, 0x6e, 0x70,
	0x75, 0x74, 0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x52, 0x6f, 0x77, 0x52, 0x04, 0x72, 0x6f, 0x77,
	0x73, 0x22, 0x45, 0x0a, 0x0e, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e,
	0x52, 0x6f, 0x77, 0x12, 0x33, 0x0a, 0x07, 0x62, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x77, 0x65, 0x62, 0x69, 0x74, 0x65, 0x6c, 0x2e, 0x63,
	0x68, 0x61, 0x74, 0x2e, 0x49, 0x6e, 0x70, 0x75, 0x74, 0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x52,
	0x07, 0x62, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x73, 0x22, 0x75, 0x0a, 0x0b, 0x49, 0x6e, 0x70, 0x75,
	0x74, 0x42, 0x75, 0x74, 0x74, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x63, 0x61, 0x70, 0x74, 0x69,
	0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x63, 0x61, 0x70, 0x74, 0x69, 0x6f,
	0x6e, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x04, 0x74, 0x65, 0x78, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x75, 0x72, 0x6c,
	0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x75, 0x72, 0x6c, 0x12, 0x12, 0x0a, 0x04, 0x63,
	0x6f, 0x64, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x63, 0x6f, 0x64, 0x65, 0x42,
	0x39, 0x5a, 0x37, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x77, 0x65,
	0x62, 0x69, 0x74, 0x65, 0x6c, 0x2f, 0x63, 0x68, 0x61, 0x74, 0x5f, 0x6d, 0x61, 0x6e, 0x61, 0x67,
	0x65, 0x72, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x63, 0x68, 0x61,
	0x74, 0x2f, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_chat_messages_message_proto_rawDescOnce sync.Once
	file_chat_messages_message_proto_rawDescData = file_chat_messages_message_proto_rawDesc
)

func file_chat_messages_message_proto_rawDescGZIP() []byte {
	file_chat_messages_message_proto_rawDescOnce.Do(func() {
		file_chat_messages_message_proto_rawDescData = protoimpl.X.CompressGZIP(file_chat_messages_message_proto_rawDescData)
	})
	return file_chat_messages_message_proto_rawDescData
}

var file_chat_messages_message_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_chat_messages_message_proto_msgTypes = make([]protoimpl.MessageInfo, 12)
var file_chat_messages_message_proto_goTypes = []any{
	(Button_Request)(0),    // 0: webitel.chat.Button.Request
	(*Message)(nil),        // 1: webitel.chat.Message
	(*File)(nil),           // 2: webitel.chat.File
	(*ReplyMarkup)(nil),    // 3: webitel.chat.ReplyMarkup
	(*ButtonRow)(nil),      // 4: webitel.chat.ButtonRow
	(*Button)(nil),         // 5: webitel.chat.Button
	(*Postback)(nil),       // 6: webitel.chat.Postback
	(*InputMessage)(nil),   // 7: webitel.chat.InputMessage
	(*InputFile)(nil),      // 8: webitel.chat.InputFile
	(*InputKeyboard)(nil),  // 9: webitel.chat.InputKeyboard
	(*InputButtonRow)(nil), // 10: webitel.chat.InputButtonRow
	(*InputButton)(nil),    // 11: webitel.chat.InputButton
	nil,                    // 12: webitel.chat.Message.ContextEntry
	(*Peer)(nil),           // 13: webitel.chat.Peer
	(*Chat)(nil),           // 14: webitel.chat.Chat
}
var file_chat_messages_message_proto_depIdxs = []int32{
	13, // 0: webitel.chat.Message.from:type_name -> webitel.chat.Peer
	14, // 1: webitel.chat.Message.chat:type_name -> webitel.chat.Chat
	14, // 2: webitel.chat.Message.sender:type_name -> webitel.chat.Chat
	2,  // 3: webitel.chat.Message.file:type_name -> webitel.chat.File
	12, // 4: webitel.chat.Message.context:type_name -> webitel.chat.Message.ContextEntry
	3,  // 5: webitel.chat.Message.keyboard:type_name -> webitel.chat.ReplyMarkup
	6,  // 6: webitel.chat.Message.postback:type_name -> webitel.chat.Postback
	4,  // 7: webitel.chat.ReplyMarkup.buttons:type_name -> webitel.chat.ButtonRow
	5,  // 8: webitel.chat.ButtonRow.row:type_name -> webitel.chat.Button
	0,  // 9: webitel.chat.Button.share:type_name -> webitel.chat.Button.Request
	8,  // 10: webitel.chat.InputMessage.file:type_name -> webitel.chat.InputFile
	9,  // 11: webitel.chat.InputMessage.keyboard:type_name -> webitel.chat.InputKeyboard
	10, // 12: webitel.chat.InputKeyboard.rows:type_name -> webitel.chat.InputButtonRow
	11, // 13: webitel.chat.InputButtonRow.buttons:type_name -> webitel.chat.InputButton
	14, // [14:14] is the sub-list for method output_type
	14, // [14:14] is the sub-list for method input_type
	14, // [14:14] is the sub-list for extension type_name
	14, // [14:14] is the sub-list for extension extendee
	0,  // [0:14] is the sub-list for field type_name
}

func init() { file_chat_messages_message_proto_init() }
func file_chat_messages_message_proto_init() {
	if File_chat_messages_message_proto != nil {
		return
	}
	file_chat_messages_peer_proto_init()
	file_chat_messages_chat_proto_init()
	file_chat_messages_message_proto_msgTypes[4].OneofWrappers = []any{
		(*Button_Url)(nil),
		(*Button_Code)(nil),
		(*Button_Share)(nil),
	}
	file_chat_messages_message_proto_msgTypes[7].OneofWrappers = []any{
		(*InputFile_Id)(nil),
		(*InputFile_Url)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_chat_messages_message_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   12,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_chat_messages_message_proto_goTypes,
		DependencyIndexes: file_chat_messages_message_proto_depIdxs,
		EnumInfos:         file_chat_messages_message_proto_enumTypes,
		MessageInfos:      file_chat_messages_message_proto_msgTypes,
	}.Build()
	File_chat_messages_message_proto = out.File
	file_chat_messages_message_proto_rawDesc = nil
	file_chat_messages_message_proto_goTypes = nil
	file_chat_messages_message_proto_depIdxs = nil
}
