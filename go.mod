module github.com/kubernetes-sigs/cluster-api-provider-ibmcloud

go 1.13

require (
	github.com/IBM/go-sdk-core v1.1.0
	github.com/IBM/go-sdk-core/v4 v4.5.1 // indirect
	github.com/IBM/vpc-go-sdk v0.1.1
	github.com/go-logr/logr v0.1.0
	github.com/onsi/ginkgo v1.13.0
	github.com/onsi/gomega v1.10.1
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.9.1
	k8s.io/api v0.17.9
	k8s.io/apimachinery v0.17.9
	k8s.io/client-go v0.17.9
	k8s.io/klog/v2 v2.0.0
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19
	sigs.k8s.io/cluster-api v0.3.9
	sigs.k8s.io/controller-runtime v0.5.10
)
