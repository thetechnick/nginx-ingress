# Ingress Controller

This repository provides an implementation of an Ingress controller for NGINX. It is based on the work done by the NGINX team [nginxinc/kubernetes-ingress repo](https://github.com/nginxinc/kubernetes-ingress), different from the NGINX Ingress controller in [kubernetes/ingress repo](https://github.com/kubernetes/ingress) and provides more features.

## What is Ingress?

An Ingress is a Kubernetes resource that lets you configure an HTTP load balancer for your Kubernetes services. Such a load balancer usually exposes your services to clients outside of your Kubernetes cluster. An Ingress resource supports:
* Exposing services:
    * Via custom URLs (for example, service A at the URL `/serviceA` and service B at the URL `/serviceB`).
    * Via multiple host names (for example, `foo.example.com` for one group of services and `bar.example.com` for another group).
* Configuring SSL termination for each exposed host name.

See the [Ingress User Guide](http://kubernetes.io/docs/user-guide/ingress/) to learn more.

## What is an Ingress Controller?

An Ingress controller is an application that monitors Ingress resources via the Kubernetes API and updates the configuration of a load balancer in case of any changes. Different load balancers require different Ingress controller implementations. Typically, an Ingress controller is deployed as a pod in a cluster. In the case of software load balancers, such as NGINX, an Ingress controller is deployed in a pod along with a load balancer.

See https://github.com/kubernetes/contrib/tree/master/ingress/controllers/ to learn more about Ingress controllers and find out about different implementations.

## NGINX Ingress Controller

We provide an Ingress controller for NGINX that supports the following Ingress features:
* SSL termination
* Path-based rules
* Multiple host names

These extensions are provided:
* [Websocket](docs/websocket), which allows you to load balance Websocket applications.
* [SSL Services](docs/ssl-services), which allows you to load balance HTTPS applications.
* [Rewrites](docs/rewrites), which allows you to rewrite the URI of a request before sending it to the application.

Additional extensions as well as a mechanism to customize NGINX configuration are available.
See [docs/customization](docs/customization).

## Components

### LBC - Load balancer controller

The lbc [quay.io/nico_schieder/ingress-lbc](https://quay.io/repository/nico_schieder/ingress-lbc) runs the main control loop that watches the Kubernetes API, validates and generates NGINX configuration files and reports these errors as events on the affected resources. It can be deployed in a server-agent mode using etcd v3 as persistent backend or standalone.

### Agent

The agent [quay.io/nico_schieder/ingress-agent](https://quay.io/repository/nico_schieder/ingress-agent) takes rendered NGINX configuration files from etcd and reconfigures NGINX to use this configuration. If etcd is unavailable the agent will keep using the last configuration and resync automatically after etcd is available again.

## Deployment Modes

### Server-Agent Mode

When using the agent deployment you only run a single instance of the load balancer controller (lbc) which generates NGINX configuration and stores them in etcd. Multiple agent instances are watching the configuration in etcd and apply these configuration files to NGINX.

In this mode you can freely scale the number of agent instances and not apply more pressure on the Kubernetes API.
Because the configuration is persistent in etcd you are free to update/restart the lbc deployment, of cause while the lbc is down service endpoints and changes to the ingress configuration will not be updated.

Only etcd version 3 and above is supported.
You find a example deployment here: [agent-deployment](docs/agent-deployment.yml)

### Standalone

In this mode the lbc also runs an embedded agent, so a etcd deployment is not necessary. If you just want to test this ingress controller, this would be the simpler way.

**Drawbacks**
- each instance watches the Kubernetes API, applying pressure to the masters
- each instance reports the same errors and warnings

You find a example deployment here: [standalone-deployment](docs/standalone-deployment.yml)

### Using Multiple  Ingress Controllers

#### Using different implementations

You can run multiple different Ingress controllers at the same time. For example, if your Kubernetes cluster is deployed in cloud, you can run the NGINX controller and the corresponding cloud HTTP load balancing controller. Refer to the [example](docs/multiple-ingress-controllers) to learn more.

#### Using multiple separated instances of this ingress controller

If you want to run multiple instances of this ingress controller you can use the flag `-selector` to filter ingresses by its labels. This feature can be used to separate restricted services from public ones by binding to different IP addresses, so you can use normal firewall rules to restrict traffic.
The flag supports the same format as `kubectl get ing --selector`.
