package collision

import (
	"time"

	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/config"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

var (
	//
	// Ingress 1
	//
	ingress1Upstream1 = config.Upstream{
		Name: "default-ing1-one.example.com-svc1",
	}
	ingress1Location1 = config.Location{
		Path:     "/1",
		Upstream: ingress1Upstream1,
	}
	ingress1Server1 = config.Server{
		Name:      "one.example.com",
		Locations: []config.Location{ingress1Location1},
		Upstreams: []config.Upstream{ingress1Upstream1},
	}
	ingress1 = v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ing1",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Hour)),
		},
	}

	//
	// Ingress 2
	//
	ingress2Upstream1 = config.Upstream{
		Name: "default-ing2-one.example.com-svc1",
	}
	ingress2Upstream2 = config.Upstream{
		Name: "default-ing2-two.example.com-svc1",
	}
	ingress2Location1 = config.Location{
		Path:     "/1",
		Upstream: ingress2Upstream1,
	}
	ingress2Location2 = config.Location{
		Path:     "/2",
		Upstream: ingress2Upstream1,
	}
	ingress2Location3 = config.Location{
		Path:     "/3",
		Upstream: ingress2Upstream2,
	}
	ingress2Server1 = config.Server{
		Name:      "one.example.com",
		Upstreams: []config.Upstream{ingress2Upstream1},
		Locations: []config.Location{ingress2Location1, ingress2Location2},
	}
	ingress2Server2 = config.Server{
		Name:      "two.example.com",
		Upstreams: []config.Upstream{ingress2Upstream2},
		Locations: []config.Location{ingress2Location3},
	}
	ingress2 = v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ing2",
			Namespace: "default",
			// this ingress is older than ing1, so it shoud be the base of the merge,
			// which means that ing2 will override conflicting locations, but not server settings
			CreationTimestamp: metav1.NewTime(time.Now().Add(-4 * time.Hour)),
		},
	}

	//
	// Ingress 3
	//
	ingress3Upstream1 = config.Upstream{
		Name: "default-ing3-one.example.com-svc1",
	}
	ingress3Location1 = config.Location{
		Path:     "/1",
		Upstream: ingress3Upstream1,
	}
	ingress3Server1 = config.Server{
		Name:      "one.example.com",
		Locations: []config.Location{ingress3Location1},
		// this server introduces additional settings,
		// because the merging process is additive they will be
		// used regardless of age
		SSL:                   true,
		SSLCertificate:        "cert.pem",
		SSLCertificateKey:     "cert.pem",
		HTTP2:                 true,
		HSTS:                  true,
		HSTSMaxAge:            2000,
		HSTSIncludeSubdomains: true,
	}
	ingress3 = v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "ing3",
			Namespace:         "default",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
	}
)
