package local

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"gitlab.thetechnick.ninja/thetechnick/nginx-ingress/pkg/storage/pb"
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
		transactionMock.On("Update", "/etc/nginx/nginx.conf", "").Return(nil)
		transactionMock.On("Update", "/etc/nginx/ssl/"+dhparamFilename, dhparam).Return(nil)
		transactionMock.On("Apply")

		err := cm.Put(&pb.MainConfig{
			Dhparam: []byte(dhparam),
		})
		if assert.NoError(err) {
			nginxMock.AssertCalled(t, "Reload")
			transactionMock.AssertCalled(t, "Update", "/etc/nginx/nginx.conf", "")
			transactionMock.AssertCalled(t, "Update", "/etc/nginx/ssl/"+dhparamFilename, dhparam)
			transactionMock.AssertCalled(t, "Apply")
		}
	})
}
