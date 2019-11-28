module sigs.k8s.io/cluster-api-provider-ibmcloud

go 1.12

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
	cloud.google.com/go v0.40.0
	contrib.go.opencensus.io/exporter/ocagent v0.4.12
	github.com/Azure/go-autorest v12.2.0+incompatible
	github.com/appscode/jsonpatch v0.0.0-20190108182946-7c0e3b262f30
	github.com/beorn7/perks v1.0.0
	github.com/census-instrumentation/opencensus-proto v0.2.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/emicklei/go-restful v2.9.6+incompatible
	github.com/evanphx/json-patch v4.5.0+incompatible
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1
	github.com/gobuffalo/envy v1.7.0
	github.com/gogo/protobuf v1.2.1
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef
	github.com/golang/protobuf v1.3.1
	github.com/google/btree v1.0.0
	github.com/google/gofuzz v1.0.0
	github.com/google/uuid v1.1.1
	github.com/googleapis/gnostic v0.3.0
	github.com/gophercloud/gophercloud v0.2.0
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79
	github.com/grpc-ecosystem/grpc-gateway v1.9.2
	github.com/hashicorp/golang-lru v0.5.1
	github.com/hpcloud/tail v1.0.0
	github.com/imdario/mergo v0.3.7
	github.com/inconshreveable/mousetrap v1.0.0
	github.com/joho/godotenv v1.3.0
	github.com/json-iterator/go v1.1.6
	github.com/markbates/inflect v1.0.4
	github.com/matttproud/golang_protobuf_extensions v1.0.1
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709
	github.com/petar/GoLLRB v0.0.0-20190514000832-33fb24c13b99
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.4
	github.com/prometheus/client_model v0.0.0-20190129233127-fd36f4220a90
	github.com/prometheus/common v0.6.0
	github.com/prometheus/procfs v0.0.3
	github.com/renier/xmlrpc v0.0.0-20170708154548-ce4a1a486c03
	github.com/rogpeppe/go-internal v1.3.0
	github.com/softlayer/softlayer-go v0.0.0-20190615201252-ba6e7f295217
	github.com/spf13/afero v1.2.2
	github.com/spf13/cobra v0.0.5
	github.com/spf13/pflag v1.0.3
	go.opencensus.io v0.19.3
	go.uber.org/atomic v1.4.0
	go.uber.org/multierr v1.1.0
	go.uber.org/zap v1.10.0
	golang.org/x/crypto v0.0.0-20190621222207-cc06ce4a13d4
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sync v0.0.0-20190423024810-112230192c58
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb
	golang.org/x/text v0.3.2
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4
	golang.org/x/tools v0.0.0-20190627033414-4874f863e654
	google.golang.org/api v0.7.0
	google.golang.org/appengine v1.6.1
	google.golang.org/genproto v0.0.0-20190626174449-989357319d63
	google.golang.org/grpc v1.21.1
	gopkg.in/fsnotify.v1 v1.4.7
	gopkg.in/inf.v0 v0.9.1
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7
	gopkg.in/yaml.v2 v2.2.2
	k8s.io/api v0.0.0-20190222213804-5cb15d344471
	k8s.io/apiextensions-apiserver v0.0.0-20190228180357-d002e88f6236
	k8s.io/apimachinery v0.0.0-20190221213512-86fb29eff628
	k8s.io/client-go v0.0.0-20190228174230-b40b2a5939e4
	k8s.io/cluster-bootstrap v0.0.0-20190612212613-c76815829c2e
	k8s.io/code-generator v0.0.0-20190620073620-d55040311883
	k8s.io/component-base v0.0.0-20190626045757-ca439aa083f5
	k8s.io/gengo v0.0.0-20190327210449-e17681d19d3a
	k8s.io/klog v0.3.3
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208
	sigs.k8s.io/cluster-api v0.0.0-20190625161037-d1b07f40847c
	sigs.k8s.io/controller-runtime v0.1.12
	sigs.k8s.io/controller-tools v0.1.11
	sigs.k8s.io/testing_frameworks v0.1.1
	sigs.k8s.io/yaml v1.1.0
)
