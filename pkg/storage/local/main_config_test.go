package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/thetechnick/nginx-ingress/pkg/storage"
	"github.com/thetechnick/nginx-ingress/pkg/storage/pb"
)

func TestConfigManager(t *testing.T) {
	cm := NewMainConfigStorage(nil).(*localMainConfigStorage)
	assert := assert.New(t)
	nginxMock := &NginxMock{}
	transactionMock := &TransactionMock{}
	cm.createTransaction = func() Transaction {
		return transactionMock
	}
	cm.nginx = nginxMock

	dhparam := "dhparam content"

	t.Run("Put", func(t *testing.T) {
		nginxMock.On("Reload").Return(nil)
		transactionMock.On("Update", mock.Anything, mock.Anything).Return(nil)
		transactionMock.On("Apply")

		err := cm.Put(&pb.MainConfig{
			Dhparam: []byte(dhparam),
			Files: []*pb.File{
				&pb.File{
					Name:    "/etc/nginx/ssl/cert.pem",
					Content: []byte("cert"),
				},
			},
		})
		if assert.NoError(err) {
			nginxMock.AssertCalled(t, "Reload")
			transactionMock.AssertCalled(t, "Update", "/etc/nginx/nginx.conf", "")
			transactionMock.AssertCalled(t, "Update", "/etc/nginx/ssl/cert.pem", "cert")
			transactionMock.AssertCalled(t, "Update", storage.DHParamFile, dhparam)
			transactionMock.AssertCalled(t, "Apply")
		}
	})
}
