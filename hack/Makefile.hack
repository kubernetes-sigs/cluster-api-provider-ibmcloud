CONTROL_PLANE_SSH_PRIVATE_KEY = ${HOME}/.ssh/id_ibmcloud
NODE_SSH_PRIVATE_KEY = $(CONTROL_PLANE_SSH_PRIVATE_KEY)

build-clusterctl:
	docker run --rm --network host -e CGO_ENABLED=0 \
		-w /go/src/sigs.k8s.io/cluster-api-provider-ibmcloud \
		-v $(shell pwd):/go/src/sigs.k8s.io/cluster-api-provider-ibmcloud \
		golang:1.10.3 go build -o bin/clusterctl cmd/clusterctl/main.go

check-env:
ifndef IBMCLOUD_APIUSERNAME
	$(error IBMCLOUD_APIUSERNAME is undefined)
endif
ifndef IBMCLOUD_AUTHENTICATION_KEY
	$(error IBMCLOUD_AUTHENTICATION_KEY is undefined)
endif
ifndef CONTROL_PLANE_SSH_KEYNAME
	$(error CONTROL_PLANE_SSH_KEYNAME is undefined)
endif

ssh-key:
	@test -e ${CONTROL_PLANE_SSH_PRIVATE_KEY} || (ssh-keygen -t rsa -N "" -f ${CONTROL_PLANE_SSH_PRIVATE_KEY} && cat ${CONTROL_PLANE_SSH_PRIVATE_KEY}.pub)
	@test -e ${NODE_SSH_PRIVATE_KEY} || ssh-keygen -t rsa -N "" -f ${NODE_SSH_PRIVATE_KEY}

output-clouds-yaml: check-env
	@mkdir -p _output
	@sed -e "s#IBMCLOUD_APIUSERNAME#${IBMCLOUD_APIUSERNAME}#g" \
		-e "s#IBMCLOUD_AUTHENTICATION_KEY#${IBMCLOUD_AUTHENTICATION_KEY}#g" \
		examples/ibmcloud/clouds.yaml.template > _output/clouds.yaml
	@echo "Generate _output/clouds.yaml success"

output-cluster-yaml: check-env
	@mkdir -p _output
	@sed -e "s#CLUSTER_NAME#${CLUSTER_NAME}#g" \
		-e "s#CLUSTER_SERVICES_CIDR_BLOCKS#${CLUSTER_SERVICES_CIDR_BLOCKS}#g" \
		-e "s#CLUSTER_PODS_CIDR_BLOCKS#${CLUSTER_PODS_CIDR_BLOCKS}#g" \
		-e "s#CLUSTER_SERVICE_DOMAIN#${CLUSTER_SERVICE_DOMAIN}#g" \
		examples/ibmcloud/cluster.yaml.template > _output/cluster.yaml
	@echo "Generate _output/cluster.yaml success"

output-machines-yaml: check-env
	@mkdir -p _output
	@test -e _output/machines.yaml || \
	sed -e "s#CLUSTER_NAME#${CLUSTER_NAME}#g" \
		-e "s#CONTROL_PLANE_DOMAIN#${CONTROL_PLANE_DOMAIN}#g" \
		-e "s#CONTROL_PLANE_OS_MEMORY#${CONTROL_PLANE_OS_MEMORY}#g" \
		-e "s#CONTROL_PLANE_OS_CPU#${CONTROL_PLANE_OS_CPU}#g" \
		-e "s#CONTROL_PLANE_DATACENTER#${CONTROL_PLANE_DATACENTER}#g" \
		-e "s#CONTROL_PLANE_OS_CODE#${CONTROL_PLANE_OS_CODE}#g" \
		-e "s#CONTROL_PLANE_SSH_KEYNAME#${CONTROL_PLANE_SSH_KEYNAME}#g" \
		-e "s#CONTROL_PLANE_SSH_USERNAME#${CONTROL_PLANE_SSH_USERNAME}#g" \
		-e "s#CONTROL_PLANE_KUBELET_VERSION#${CONTROL_PLANE_KUBELET_VERSION}#g" \
		-e "s#CONTROL_PLANE_VERSION#${CONTROL_PLANE_VERSION}#g" \
		-e "s#NODE_DOMAIN#${NODE_DOMAIN}#g" \
		-e "s#NODE_OS_MEMORY#${NODE_OS_MEMORY}#g" \
		-e "s#NODE_OS_CPU#${NODE_OS_CPU}#g" \
		-e "s#NODE_DATACENTER#${NODE_DATACENTER}#g" \
		-e "s#NODE_OS_CODE#${NODE_OS_CODE}#g" \
		-e "s#NODE_SSH_KEYNAME#${NODE_SSH_KEYNAME}#g" \
		-e "s#NODE_SSH_USERNAME#${NODE_SSH_USERNAME}#g" \
		-e "s#NODE_KUBELET_VERSION#${NODE_KUBELET_VERSION}#g" \
		examples/ibmcloud/machines.yaml.template > _output/machines.yaml && echo "Generate _output/machines.yaml success"

output-provider-components-yaml: output-clouds-yaml
	@mkdir -p _output/provider-component/
	@cat examples/ibmcloud/kustomization.yaml > _output/kustomization.yaml
	@\cp -r examples/ibmcloud/provider-component/user-data _output/provider-component/
	@\cp -r examples/ibmcloud/provider-component/cluster-api _output/provider-component/
	@\cp -r vendor/sigs.k8s.io/cluster-api/config _output/provider-component/cluster-api/
	@kubectl kustomize _output > _output/provider-components.yaml
	@echo "Generate _output/provider-components.yaml success"

output-addons-yaml:
	@mkdir -p _output
	@test -e _output/addons.yaml || echo '---' > _output/addons.yaml

generate-yaml: output-clouds-yaml output-cluster-yaml output-machines-yaml output-provider-components-yaml
	@echo "Generate all required yaml files success"

create-with-kind:
	bin/clusterctl create cluster --bootstrap-type kind --provider ibmcloud \
		-c _output/cluster.yaml \
		-m _output/machines.yaml \
		-p _output/provider-components.yaml
