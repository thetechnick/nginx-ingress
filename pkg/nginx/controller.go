package nginx

import (
	"fmt"
	"os"
	"path"
	"text/template"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/shell"

	log "github.com/sirupsen/logrus"
)

const dhparamFilename = "dhparam.pem"

// Controller Updates NGINX configuration, starts and reloads NGINX
type Controller struct {
	nginxConfdPath string
	nginxCertsPath string
	local          bool
	healthStatus   bool
	nginx          Nginx
}

// NewUpstreamWithDefaultServer creates an upstream with the default server.
// proxy_pass to an upstream with the default server returns 502.
// We use it for services that have no endpoints
func NewUpstreamWithDefaultServer(name string) config.Upstream {
	return config.Upstream{
		Name:            name,
		UpstreamServers: []config.UpstreamServer{config.UpstreamServer{Address: "127.0.0.1", Port: "8181"}},
	}
}

// NewController creates a NGINX controller
func NewController(nginxConfPath string, local bool, healthStatus bool) (*Controller, error) {
	ngxc := Controller{
		nginxConfdPath: path.Join(nginxConfPath, "conf.d"),
		nginxCertsPath: path.Join(nginxConfPath, "ssl"),
		local:          local,
		healthStatus:   healthStatus,
		nginx:          NewNginx(shell.NewShellExecutor()),
	}

	cfg := &config.MainConfig{ServerNamesHashMaxSize: config.NewDefaultConfig().MainServerNamesHashMaxSize}
	ngxc.UpdateMainConfigFile(cfg)

	return &ngxc, nil
}

// DeleteIngress deletes the configuration file, which corresponds for the
// specified ingress from NGINX conf directory
func (nginx *Controller) DeleteIngress(name string) {
	filename := nginx.getIngressNginxConfigFileName(name)
	log.Infof("deleting %v", filename)

	if !nginx.local {
		if err := os.Remove(filename); err != nil {
			log.Warningf("Failed to delete %v: %v", filename, err)
		}
	}
}

// AddOrUpdateConfig creates or updates a file with
// the specified configuration for the specified ingress
func (nginx *Controller) AddOrUpdateConfig(name string, config config.Server) {
	log.Infof("Updating NGINX configuration")
	filename := nginx.getIngressNginxConfigFileName(name)
	nginx.templateIt(config, filename)
}

// AddOrUpdateDHParam creates the servers dhparam.pem file
func (nginx *Controller) AddOrUpdateDHParam(dhparam string) (string, error) {
	fileName := nginx.nginxCertsPath + "/" + dhparamFilename
	if !nginx.local {
		pem, err := os.Create(fileName)
		if err != nil {
			return fileName, fmt.Errorf("Couldn't create file %v: %v", fileName, err)
		}
		defer pem.Close()

		_, err = pem.WriteString(dhparam)
		if err != nil {
			return fileName, fmt.Errorf("Couldn't write to pem file %v: %v", fileName, err)
		}
	}
	return fileName, nil
}

// AddOrUpdateCertAndKey creates a .pem file wth the cert and the key with the
// specified name
func (nginx *Controller) AddOrUpdateCertAndKey(name string, cert string, key string) string {
	pemFileName := nginx.nginxCertsPath + "/" + name + ".pem"

	if !nginx.local {
		pem, err := os.Create(pemFileName)
		if err != nil {
			log.Fatalf("Couldn't create pem file %v: %v", pemFileName, err)
		}
		defer pem.Close()

		_, err = pem.WriteString(key)
		if err != nil {
			log.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
		}

		_, err = pem.WriteString("\n")
		if err != nil {
			log.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
		}

		_, err = pem.WriteString(cert)
		if err != nil {
			log.Fatalf("Couldn't write to pem file %v: %v", pemFileName, err)
		}
	}

	return pemFileName
}

func (nginx *Controller) getIngressNginxConfigFileName(name string) string {
	if name == emptyHost {
		name = "default"
	}
	return path.Join(nginx.nginxConfdPath, name+".conf")
}

func (nginx *Controller) templateIt(config config.Server, filename string) {
	tmpl, err := template.New("ingress.tmpl").ParseFiles("ingress.tmpl")
	if err != nil {
		log.Fatal("Failed to parse template file")
	}

	log.Infof("Writing NGINX conf to %v", filename)

	if !nginx.local {
		w, err := os.Create(filename)
		if err != nil {
			log.Fatalf("Failed to open %v: %v", filename, err)
		}
		defer w.Close()

		if err := tmpl.Execute(w, config); err != nil {
			log.Fatalf("Failed to write template %v", err)
		}
	} else {
		// print conf to stdout here
	}

	log.Infof("NGINX configuration file had been updated")
}

// Reload reloads NGINX
func (nginx *Controller) Reload() error {
	if !nginx.local {
		if err := nginx.nginx.TestConfig(); err != nil {
			return fmt.Errorf("Invalid nginx configuration detected, not reloading: %s", err)
		}
		if err := nginx.nginx.Reload(); err != nil {
			return fmt.Errorf("Reloading NGINX failed: %s", err)
		}
	} else {
		log.Info("Reloading nginx")
	}
	return nil
}

// Start starts NGINX
func (nginx *Controller) Start() {
	if !nginx.local {
		if err := nginx.nginx.Start(); err != nil {
			log.Fatalf("Failed to start nginx: %v", err)
		}
	} else {
		log.Info("Starting nginx")
	}
}

func createDir(path string) {
	if err := os.Mkdir(path, os.ModeDir); err != nil {
		log.Fatalf("Couldn't create directory %v: %v", path, err)
	}
}

// UpdateMainConfigFile update the main NGINX configuration file
func (nginx *Controller) UpdateMainConfigFile(cfg *config.MainConfig) {
	cfg.HealthStatus = nginx.healthStatus

	tmpl, err := template.New("nginx.conf.tmpl").ParseFiles("nginx.conf.tmpl")
	if err != nil {
		log.Fatalf("Failed to parse the main config template file: %v", err)
	}

	filename := "/etc/nginx/nginx.conf"
	log.Infof("Writing NGINX conf to %v", filename)

	if !nginx.local {
		w, err := os.Create(filename)
		if err != nil {
			log.Fatalf("Failed to open %v: %v", filename, err)
		}
		defer w.Close()

		if err := tmpl.Execute(w, cfg); err != nil {
			log.Fatalf("Failed to write template %v", err)
		}
	}

	log.Infof("The main NGINX configuration file had been updated")
}
