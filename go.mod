module sigs.k8s.io/cluster-api-provider-ibmcloud

go 1.16

require (
	github.com/IBM-Cloud/bluemix-go v0.0.0-20200921095234-26d1d0148c62
	github.com/IBM-Cloud/power-go-client v1.0.85
	github.com/IBM/go-sdk-core/v5 v5.9.1
	github.com/IBM/vpc-go-sdk v0.14.0
	github.com/go-logr/logr v1.2.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/pkg/errors v0.9.1
	github.com/ppc64le-cloud/powervs-utils v0.0.0-20210106101518-5d3f965b0344
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.23.0
	k8s.io/apimachinery v0.23.0
	k8s.io/client-go v0.23.0
	k8s.io/klog/v2 v2.30.0
	k8s.io/utils v0.0.0-20210930125809-cb0fa318a74b
	sigs.k8s.io/cluster-api v1.0.2
	sigs.k8s.io/cluster-api/test v1.0.2
	sigs.k8s.io/controller-runtime v0.11.0
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v1.0.2
