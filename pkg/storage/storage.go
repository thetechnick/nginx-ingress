package storage

import "github.com/thetechnick/nginx-ingress/pkg/storage/pb"

const (
	// MainConfigDir is the directory containing all configs
	MainConfigDir = "/etc/nginx/"
	// ServerConfigDir contains the server/host configs
	ServerConfigDir = "/etc/nginx/conf.d/"
	// CertificatesDir contains the tls/ssl certificates and the dhparam file
	CertificatesDir = "/etc/nginx/ssl/"
	// AuthDir contains basic auth files
	AuthDir = "/etc/nginx/auth/"
	// DHParamFile path to the DHParamsFile
	DHParamFile = "/etc/nginx/ssl/dhparam.pem"
)

// ServerConfigStorage stores ServerConfigs
type ServerConfigStorage interface {
	Put(serverConfig *pb.ServerConfig) error
	Delete(serverConfig *pb.ServerConfig) error
	List() ([]*pb.ServerConfig, error)
	ByIngressKey(ingressKey string) ([]*pb.ServerConfig, error)
	Get(name string) (*pb.ServerConfig, error)
}

// MainConfigStorage stores the MainConfig
type MainConfigStorage interface {
	Put(cfg *pb.MainConfig) error
	Get() (*pb.MainConfig, error)
}
