package command

import (
	auditv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/audit/v1/auditv1connect"
	baremetalv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/baremetal/v1/baremetalv1connect"
	catalogv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/catalog/v1/catalogv1connect"
	computev1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/compute/v1/computev1connect"
	healthv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/health/v1/healthv1connect"
	networkv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/network/v1/networkv1connect"
	objectstorev1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/objectstore/v1/objectstorev1connect"
	opsv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/ops/v1/opsv1connect"
	projectv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/project/v1/projectv1connect"
	quotav1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/quota/v1/quotav1connect"
	storagev1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/storage/v1/storagev1connect"
	supportv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/support/v1/supportv1connect"
	walletv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/wallet/v1/walletv1connect"
	webhooksv1connect "github.com/AES-Services/metalhost-sdk/gen/go/aes/webhooks/v1/webhooksv1connect"
)

func (c *commandContext) catalogClient() (catalogv1connect.CatalogServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return catalogv1connect.NewCatalogServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) healthClient() (healthv1connect.HealthServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return healthv1connect.NewHealthServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) projectClient() (projectv1connect.ProjectServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return projectv1connect.NewProjectServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) opsClient() (opsv1connect.OperationsServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return opsv1connect.NewOperationsServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) computeClient() (computev1connect.ComputeServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return computev1connect.NewComputeServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) sshKeyClient() (computev1connect.SSHKeysServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return computev1connect.NewSSHKeysServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) userDataSnippetClient() (computev1connect.UserDataSnippetsServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return computev1connect.NewUserDataSnippetsServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) storageClient() (storagev1connect.StorageServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return storagev1connect.NewStorageServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) networkClient() (networkv1connect.NetworkServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return networkv1connect.NewNetworkServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) objectStoreClient() (objectstorev1connect.ObjectStoreServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return objectstorev1connect.NewObjectStoreServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) walletClient() (walletv1connect.WalletServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return walletv1connect.NewWalletServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) quotaClient() (quotav1connect.QuotaServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return quotav1connect.NewQuotaServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) auditClient() (auditv1connect.AuditServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return auditv1connect.NewAuditServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) bareMetalClient() (baremetalv1connect.BareMetalServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return baremetalv1connect.NewBareMetalServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) webhooksClient() (webhooksv1connect.WebhooksServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return webhooksv1connect.NewWebhooksServiceClient(cfg.Client(), cfg.BaseURL()), nil
}

func (c *commandContext) supportClient() (supportv1connect.SupportServiceClient, error) {
	cfg, err := c.sdkConfig()
	if err != nil {
		return nil, err
	}
	return supportv1connect.NewSupportServiceClient(cfg.Client(), cfg.BaseURL()), nil
}
