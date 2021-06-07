module github.com/kubernetes-sigs/cluster-api-provider-ibmcloud

go 1.13

require (
	github.com/IBM/go-sdk-core v1.1.0
	github.com/IBM/go-sdk-core/v4 v4.5.1 // indirect
	github.com/IBM/vpc-go-sdk v0.1.1
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.1
	github.com/onsi/gomega v1.11.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.15.0
	k8s.io/api v0.21.1
	k8s.io/apimachinery v0.21.1
	k8s.io/client-go v0.21.1
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210305010621-2afb4311ab10
	sigs.k8s.io/cluster-api v0.0.0-20210526191338-0e1629b75111
	sigs.k8s.io/controller-runtime v0.9.0-beta.5
)
