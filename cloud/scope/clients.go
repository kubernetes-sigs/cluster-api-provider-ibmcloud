package scope

import (
	"github.com/IBM/go-sdk-core/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

type IBMVPCClients struct {
	VPCService *vpcv1.VpcV1
	//APIKey          string
	//IAMEndpoint     string
	//ServiceEndPoint string
}

func (c *IBMVPCClients) setIBMVPCService(iamEndpoint string, svcEndpoint string, apiKey string) error {
	var err error
	c.VPCService, err = vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: &core.IamAuthenticator{
			ApiKey: apiKey,
			URL:    iamEndpoint,
		},
		URL: svcEndpoint,
	})

	return err
}
