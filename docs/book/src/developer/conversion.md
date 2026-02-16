# Guide for API conversions

## Introduction
The purpose of this document is to help/assist contributors with future API conversions using conversion-gen tool.

## Prerequisites
1. Create a new API version.
```shell
kubebuilder create api --group <group> --version <version> --kind <kind>
```
2. Copy over existing types, and make the required changes.
3. Mark a storage version, add marker `+kubebuilder:storageversion` to concerned version package.

**_NOTE:_** [Refer for more detailed information about prerequisites.](https://kubebuilder.io/multiversion-tutorial/api-changes.html#changing-things-up)

## Conversion flow
1. In each “spoke” version package, add marker `+k8s:conversion-gen` directive pointing to the “hub” version package. It must be in `doc.go`. [Refer](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/api/powervs/v1beta2/doc.go)
2. In “hub” version package, create `doc.go` file without any marker. [Refer](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/api/powervs/v1beta2/doc.go)
3. In “spoke” version package, add a var `localSchemeBuilder = &SchemeBuilder.SchemeBuilder` in `groupversion_info.go` so the auto-generated code would compile. [Refer](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/api/powervs/v1beta2/groupversion_info.go)
4. In “hub” version package, create a `conversion.go` to implement the “hub” methods. [Refer](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/api/powervs/v1beta3/conversion.go)
5. Run target `make generate-go-conversions-core`, this will generate `zz_generated.conversion.go` in the spoke version package.
6. In "spoke" version package, update `{kind}_conversion.go` to implement Convertible for each type. When `conversion-gen` stops generating methods because of incompatibilities or we need to override the behavior, we stick them in this source file. Our “spoke” versions need to implement the Convertible interface. Namely, they’ll need ConvertTo and ConvertFrom methods to convert to/from the hub version. [Refer](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/main/api/powervs/v1beta2/conversion.go)

## References
- [What are hubs and spokes?](https://book.kubebuilder.io/multiversion-tutorial/conversion-concepts.html)
- [Sample Tutorial](https://book.kubebuilder.io/multiversion-tutorial/tutorial.html)
