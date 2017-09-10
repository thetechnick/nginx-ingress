// Code generated by protoc-gen-go.
// source: storage.proto
// DO NOT EDIT!

/*
Package pb is a generated protocol buffer package.

It is generated from these files:
	storage.proto

It has these top-level messages:
	ServerConfig
	TLSCertificate
	MainConfig
*/
package pb

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type ServerConfig struct {
	Name   string            `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	Config []byte            `protobuf:"bytes,2,opt,name=config,proto3" json:"config,omitempty"`
	Tls    *TLSCertificate   `protobuf:"bytes,3,opt,name=tls" json:"tls,omitempty"`
	Meta   map[string]string `protobuf:"bytes,4,rep,name=meta" json:"meta,omitempty" protobuf_key:"bytes,1,opt,name=key" protobuf_val:"bytes,2,opt,name=value"`
}

func (m *ServerConfig) Reset()                    { *m = ServerConfig{} }
func (m *ServerConfig) String() string            { return proto.CompactTextString(m) }
func (*ServerConfig) ProtoMessage()               {}
func (*ServerConfig) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *ServerConfig) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *ServerConfig) GetConfig() []byte {
	if m != nil {
		return m.Config
	}
	return nil
}

func (m *ServerConfig) GetTls() *TLSCertificate {
	if m != nil {
		return m.Tls
	}
	return nil
}

func (m *ServerConfig) GetMeta() map[string]string {
	if m != nil {
		return m.Meta
	}
	return nil
}

type TLSCertificate struct {
	Name    string `protobuf:"bytes,1,opt,name=name" json:"name,omitempty"`
	Content []byte `protobuf:"bytes,2,opt,name=content,proto3" json:"content,omitempty"`
}

func (m *TLSCertificate) Reset()                    { *m = TLSCertificate{} }
func (m *TLSCertificate) String() string            { return proto.CompactTextString(m) }
func (*TLSCertificate) ProtoMessage()               {}
func (*TLSCertificate) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *TLSCertificate) GetName() string {
	if m != nil {
		return m.Name
	}
	return ""
}

func (m *TLSCertificate) GetContent() []byte {
	if m != nil {
		return m.Content
	}
	return nil
}

type MainConfig struct {
	Config  []byte `protobuf:"bytes,1,opt,name=config,proto3" json:"config,omitempty"`
	Dhparam []byte `protobuf:"bytes,2,opt,name=dhparam,proto3" json:"dhparam,omitempty"`
}

func (m *MainConfig) Reset()                    { *m = MainConfig{} }
func (m *MainConfig) String() string            { return proto.CompactTextString(m) }
func (*MainConfig) ProtoMessage()               {}
func (*MainConfig) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *MainConfig) GetConfig() []byte {
	if m != nil {
		return m.Config
	}
	return nil
}

func (m *MainConfig) GetDhparam() []byte {
	if m != nil {
		return m.Dhparam
	}
	return nil
}

func init() {
	proto.RegisterType((*ServerConfig)(nil), "pb.ServerConfig")
	proto.RegisterType((*TLSCertificate)(nil), "pb.TLSCertificate")
	proto.RegisterType((*MainConfig)(nil), "pb.MainConfig")
}

func init() { proto.RegisterFile("storage.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 251 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0x90, 0xb1, 0x4e, 0xf3, 0x30,
	0x10, 0xc7, 0xe5, 0x24, 0x5f, 0xab, 0x5c, 0xfb, 0x21, 0x64, 0x21, 0x64, 0x75, 0x8a, 0x22, 0x86,
	0x4c, 0x1e, 0xca, 0x00, 0x62, 0xe8, 0x52, 0xb1, 0xd1, 0xc5, 0xe5, 0x05, 0x2e, 0xe1, 0x5a, 0x22,
	0x1a, 0x3b, 0x72, 0x8f, 0x4a, 0x7d, 0x44, 0xde, 0x0a, 0xc5, 0xb8, 0x40, 0x25, 0xb6, 0xfb, 0xf9,
	0xce, 0xba, 0xfb, 0xfd, 0xe1, 0xff, 0x9e, 0x9d, 0xc7, 0x2d, 0xe9, 0xde, 0x3b, 0x76, 0x32, 0xe9,
	0xeb, 0xf2, 0x43, 0xc0, 0x74, 0x4d, 0xfe, 0x40, 0x7e, 0xe9, 0xec, 0xa6, 0xdd, 0x4a, 0x09, 0x99,
	0xc5, 0x8e, 0x94, 0x28, 0x44, 0x95, 0x9b, 0x50, 0xcb, 0x6b, 0x18, 0x35, 0xa1, 0xab, 0x92, 0x42,
	0x54, 0x53, 0x13, 0x49, 0xde, 0x40, 0xca, 0xbb, 0xbd, 0x4a, 0x0b, 0x51, 0x4d, 0xe6, 0x52, 0xf7,
	0xb5, 0x7e, 0x7e, 0x5a, 0x2f, 0xc9, 0x73, 0xbb, 0x69, 0x1b, 0x64, 0x32, 0x43, 0x5b, 0x6a, 0xc8,
	0x3a, 0x62, 0x54, 0x59, 0x91, 0x56, 0x93, 0xf9, 0x6c, 0x18, 0xfb, 0xbd, 0x51, 0xaf, 0x88, 0xf1,
	0xd1, 0xb2, 0x3f, 0x9a, 0x30, 0x37, 0xbb, 0x83, 0xfc, 0xfb, 0x49, 0x5e, 0x42, 0xfa, 0x46, 0xc7,
	0x78, 0xcd, 0x50, 0xca, 0x2b, 0xf8, 0x77, 0xc0, 0xdd, 0x3b, 0x85, 0x5b, 0x72, 0xf3, 0x05, 0x0f,
	0xc9, 0xbd, 0x28, 0x17, 0x70, 0x71, 0xbe, 0xff, 0x4f, 0x19, 0x05, 0xe3, 0xc6, 0x59, 0x26, 0xcb,
	0xd1, 0xe6, 0x84, 0xe5, 0x02, 0x60, 0x85, 0xad, 0x8d, 0x41, 0xfc, 0x48, 0x8b, 0x33, 0x69, 0x05,
	0xe3, 0x97, 0xd7, 0x1e, 0x3d, 0x76, 0xa7, 0xff, 0x11, 0xeb, 0x51, 0x88, 0xf5, 0xf6, 0x33, 0x00,
	0x00, 0xff, 0xff, 0x6c, 0xfd, 0x5e, 0x6a, 0x67, 0x01, 0x00, 0x00,
}