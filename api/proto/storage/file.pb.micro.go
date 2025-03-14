// Code generated by protoc-gen-micro. DO NOT EDIT.
// source: file.proto

package storage

import (
	fmt "fmt"
	proto "github.com/golang/protobuf/proto"
	math "math"
)

import (
	context "context"
	api "github.com/micro/micro/v3/service/api"
	client "github.com/micro/micro/v3/service/client"
	server "github.com/micro/micro/v3/service/server"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion3 // please upgrade the proto package

// Reference imports to suppress errors if they are not otherwise used.
var _ api.Endpoint
var _ context.Context
var _ client.Option
var _ server.Option

// Api Endpoints for FileService service

func NewFileServiceEndpoints() []*api.Endpoint {
	return []*api.Endpoint{}
}

// Client API for FileService service

type FileService interface {
	UploadFile(ctx context.Context, opts ...client.CallOption) (FileService_UploadFileService, error)
	UploadFileUrl(ctx context.Context, in *UploadFileUrlRequest, opts ...client.CallOption) (*UploadFileUrlResponse, error)
	GenerateFileLink(ctx context.Context, in *GenerateFileLinkRequest, opts ...client.CallOption) (*GenerateFileLinkResponse, error)
}

type fileService struct {
	c    client.Client
	name string
}

func NewFileService(name string, c client.Client) FileService {
	return &fileService{
		c:    c,
		name: name,
	}
}

func (c *fileService) UploadFile(ctx context.Context, opts ...client.CallOption) (FileService_UploadFileService, error) {
	req := c.c.NewRequest(c.name, "FileService.UploadFile", &UploadFileRequest{})
	stream, err := c.c.Stream(ctx, req, opts...)
	if err != nil {
		return nil, err
	}
	return &fileServiceUploadFile{stream}, nil
}

type FileService_UploadFileService interface {
	Context() context.Context
	SendMsg(interface{}) error
	RecvMsg(interface{}) error
	CloseAndRecv() (*UploadFileResponse, error)
	Send(*UploadFileRequest) error
}

type fileServiceUploadFile struct {
	stream client.Stream
}

func (x *fileServiceUploadFile) CloseAndRecv() (*UploadFileResponse, error) {
	if err := x.stream.Close(); err != nil {
		return nil, err
	}
	r := new(UploadFileResponse)
	err := x.RecvMsg(r)
	return r, err
}

func (x *fileServiceUploadFile) Context() context.Context {
	return x.stream.Context()
}

func (x *fileServiceUploadFile) SendMsg(m interface{}) error {
	return x.stream.Send(m)
}

func (x *fileServiceUploadFile) RecvMsg(m interface{}) error {
	return x.stream.Recv(m)
}

func (x *fileServiceUploadFile) Send(m *UploadFileRequest) error {
	return x.stream.Send(m)
}

func (c *fileService) UploadFileUrl(ctx context.Context, in *UploadFileUrlRequest, opts ...client.CallOption) (*UploadFileUrlResponse, error) {
	req := c.c.NewRequest(c.name, "FileService.UploadFileUrl", in)
	out := new(UploadFileUrlResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *fileService) GenerateFileLink(ctx context.Context, in *GenerateFileLinkRequest, opts ...client.CallOption) (*GenerateFileLinkResponse, error) {
	req := c.c.NewRequest(c.name, "FileService.GenerateFileLink", in)
	out := new(GenerateFileLinkResponse)
	err := c.c.Call(ctx, req, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Server API for FileService service

type FileServiceHandler interface {
	UploadFile(context.Context, FileService_UploadFileStream) error
	UploadFileUrl(context.Context, *UploadFileUrlRequest, *UploadFileUrlResponse) error
	GenerateFileLink(context.Context, *GenerateFileLinkRequest, *GenerateFileLinkResponse) error
}

func RegisterFileServiceHandler(s server.Server, hdlr FileServiceHandler, opts ...server.HandlerOption) error {
	type fileService interface {
		UploadFile(ctx context.Context, stream server.Stream) error
		UploadFileUrl(ctx context.Context, in *UploadFileUrlRequest, out *UploadFileUrlResponse) error
		GenerateFileLink(ctx context.Context, in *GenerateFileLinkRequest, out *GenerateFileLinkResponse) error
	}
	type FileService struct {
		fileService
	}
	h := &fileServiceHandler{hdlr}
	return s.Handle(s.NewHandler(&FileService{h}, opts...))
}

type fileServiceHandler struct {
	FileServiceHandler
}

func (h *fileServiceHandler) UploadFile(ctx context.Context, stream server.Stream) error {
	return h.FileServiceHandler.UploadFile(ctx, &fileServiceUploadFileStream{stream})
}

type FileService_UploadFileStream interface {
	Context() context.Context
	SendMsg(interface{}) error
	RecvMsg(interface{}) error
	SendAndClose(*UploadFileResponse) error
	Recv() (*UploadFileRequest, error)
}

type fileServiceUploadFileStream struct {
	stream server.Stream
}

func (x *fileServiceUploadFileStream) SendAndClose(in *UploadFileResponse) error {
	if err := x.SendMsg(in); err != nil {
		return err
	}
	return x.stream.Close()
}

func (x *fileServiceUploadFileStream) Context() context.Context {
	return x.stream.Context()
}

func (x *fileServiceUploadFileStream) SendMsg(m interface{}) error {
	return x.stream.Send(m)
}

func (x *fileServiceUploadFileStream) RecvMsg(m interface{}) error {
	return x.stream.Recv(m)
}

func (x *fileServiceUploadFileStream) Recv() (*UploadFileRequest, error) {
	m := new(UploadFileRequest)
	if err := x.stream.Recv(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (h *fileServiceHandler) UploadFileUrl(ctx context.Context, in *UploadFileUrlRequest, out *UploadFileUrlResponse) error {
	return h.FileServiceHandler.UploadFileUrl(ctx, in, out)
}

func (h *fileServiceHandler) GenerateFileLink(ctx context.Context, in *GenerateFileLinkRequest, out *GenerateFileLinkResponse) error {
	return h.FileServiceHandler.GenerateFileLink(ctx, in, out)
}
