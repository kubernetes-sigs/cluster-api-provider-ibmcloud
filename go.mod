module github.com/kubernetes-sigs/cluster-api-provider-ibmcloud

go 1.16

require (
	github.com/IBM-Cloud/bluemix-go v0.0.0-20200921095234-26d1d0148c62
	github.com/IBM-Cloud/power-go-client v1.0.75
	github.com/IBM/go-sdk-core/v5 v5.7.0
	github.com/IBM/vpc-go-sdk v0.1.1
	github.com/go-logr/logr v0.4.0
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.15.0
	github.com/pkg/errors v0.9.1
	github.com/ppc64le-cloud/powervs-utils v0.0.0-20210106101518-5d3f965b0344
	github.com/prometheus/common v0.26.0
	k8s.io/api v0.21.4
	k8s.io/apimachinery v0.21.4
	k8s.io/client-go v0.21.4
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176
	sigs.k8s.io/cluster-api v0.0.0-20210526191338-0e1629b75111
	sigs.k8s.io/controller-runtime v0.9.7
)

replace (
	//TODO(mkumatag): remove this when github.com/IBM-Cloud/power-go-client adopts the recent version of github.com/go-openapi/errors package
	github.com/go-openapi/errors => github.com/go-openapi/errors v0.19.3
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.4
)
