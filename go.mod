module github.com/kubernetes-sigs/cluster-api-provider-ibmcloud

go 1.16

require (
	github.com/IBM/go-sdk-core v1.1.0
	github.com/IBM/go-sdk-core/v4 v4.5.1 // indirect
	github.com/IBM/vpc-go-sdk v0.1.1
	github.com/go-logr/logr v0.4.0
	github.com/onsi/ginkgo v1.16.4
	github.com/onsi/gomega v1.13.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.26.0
	k8s.io/api v0.21.2
	k8s.io/apimachinery v0.21.2
	k8s.io/client-go v0.21.2
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210527160623-6fdb442a123b
	sigs.k8s.io/cluster-api v0.0.0-20210526191338-0e1629b75111
	sigs.k8s.io/controller-runtime v0.9.3
)

replace sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.4.0
