module sigs.k8s.io/cluster-api-provider-ibmcloud

go 1.13

replace (
	k8s.io/api => k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery => k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go => k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.0.0-20190612212613-c76815829c2e
	k8s.io/code-generator => k8s.io/code-generator v0.0.0-20190620073620-d55040311883
	k8s.io/component-base => k8s.io/component-base v0.0.0-20190626045757-ca439aa083f5
	k8s.io/gengo => k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
	k8s.io/klog => k8s.io/klog v0.3.3
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208
	sigs.k8s.io/cluster-api => sigs.k8s.io/cluster-api v0.0.0-20190625161037-d1b07f40847c
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools => sigs.k8s.io/controller-tools v0.1.11
	sigs.k8s.io/testing_frameworks => sigs.k8s.io/testing_frameworks v0.1.1
	sigs.k8s.io/yaml => sigs.k8s.io/yaml v1.1.0
)

require (
	cloud.google.com/go v0.40.0 // indirect
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30 // indirect
	github.com/go-logr/logr v0.1.0 // indirect
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/golang/mock v1.2.0
	github.com/golangci/golangci-lint v1.23.7
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/jarcoal/httpmock v1.0.4 // indirect
	github.com/markbates/inflect v1.0.4 // indirect
	github.com/onsi/gomega v1.8.1
	github.com/pkg/errors v0.8.1
	github.com/prometheus/common v0.6.0 // indirect
	github.com/prometheus/procfs v0.0.3 // indirect
	github.com/renier/xmlrpc v0.0.0-20170708154548-ce4a1a486c03 // indirect
	github.com/softlayer/softlayer-go v0.0.0-20190615201252-ba6e7f295217
	golang.org/x/net v0.0.0-20190923162816-aa69164e4478
	google.golang.org/appengine v1.6.1 // indirect
	gopkg.in/yaml.v2 v2.2.7
	k8s.io/api v0.0.0-20190612210016-7525909cc6da
	k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236 // indirect
	k8s.io/apimachinery v0.0.0-20190624085041-961b39a1baa0
	k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/cluster-bootstrap v0.0.0-20190612212613-c76815829c2e
	k8s.io/code-generator v0.18.0-alpha.2.0.20200130061103-7dfd5e9157ef
	k8s.io/component-base v0.0.0-20190626045757-ca439aa083f5 // indirect
	k8s.io/klog v1.0.0
	sigs.k8s.io/cluster-api v0.0.0-20190625161037-d1b07f40847c
	sigs.k8s.io/cluster-api/hack/tools v0.0.0-20200228224239-308f931d5f0c
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/controller-tools v0.2.5
	sigs.k8s.io/testing_frameworks v0.1.1
	sigs.k8s.io/yaml v1.1.0
)
