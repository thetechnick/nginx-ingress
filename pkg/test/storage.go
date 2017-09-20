package test

import (
	"github.com/stretchr/testify/mock"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
)

type ServerConfigStorageMock struct {
	mock.Mock
}

func (m *ServerConfigStorageMock) Put(serverConfig *pb.ServerConfig) error {
	args := m.Called(serverConfig)
	return args.Error(0)
}

func (m *ServerConfigStorageMock) Delete(serverConfig *pb.ServerConfig) error {
	args := m.Called(serverConfig)
	return args.Error(0)
}

func (m *ServerConfigStorageMock) List() ([]*pb.ServerConfig, error) {
	args := m.Called()
	return args.Get(0).([]*pb.ServerConfig), args.Error(1)
}

func (m *ServerConfigStorageMock) ByIngressKey(ingressKey string) ([]*pb.ServerConfig, error) {
	args := m.Called(ingressKey)
	return args.Get(0).([]*pb.ServerConfig), args.Error(1)
}

func (m *ServerConfigStorageMock) Get(name string) (*pb.ServerConfig, error) {
	args := m.Called(name)
	return args.Get(0).(*pb.ServerConfig), args.Error(1)
}

type MainConfigStorageMock struct {
	mock.Mock
}

func (m *MainConfigStorageMock) Put(cfg *pb.MainConfig) error {
	args := m.Called(cfg)
	return args.Error(0)
}

func (m *MainConfigStorageMock) Get() (*pb.MainConfig, error) {
	args := m.Called()
	return args.Get(0).(*pb.MainConfig), args.Error(1)
}
