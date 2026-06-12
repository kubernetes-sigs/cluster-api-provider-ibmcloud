# Migration Guides

This section contains migration guides for upgrading between different API versions of the Cluster API Provider for IBM Cloud.

## Available Migration Guides

### PowerVS API Migrations

- **[v1beta2 to v1beta3](./powervs-v1beta2-to-v1beta3.md)** - Migration guide for PowerVS clusters from v1beta2 to v1beta3 API, covering:
  - Topology specification improvements (VirtualIP vs. LoadBalancer)
  - Data type enhancements (pointer removal for safer, predictable API behavior)
  - Workspace configuration changes (Reference vs. Provision)
  - Network configuration enhancements (explicit type declaration)
  - IBMPowerVSMachine workspace and network reference updates
  - IBMPowerVSImage workspace reference updates
  - Conversion webhook details for automatic migration

> **Note:** This guide covers the currently implemented v1beta3 changes. Additional API improvements will be documented as they are completed.

## Why Migrate?

Each new API version brings improvements in:

- **Type Safety**: Better validation, fewer runtime errors, and pointer-free data structures.
- **Discoverability**: All configuration options are documented and explorable.
- **Maintainability**: Clearer intent and simpler controller logic.
- **GitOps Compatibility**: Better support for modern deployment tools.
- **Production Readiness**: Enhanced features for enterprise deployments.

## Migration Support

- **Automatic Conversion**: Conversion webhooks provide automatic translation between API versions
- **Backward Compatibility**: Older API versions continue to work during the migration period
- **Validation**: CEL validation rules catch configuration errors early
- **Documentation**: Comprehensive guides with examples for common scenarios

## Getting Help

If you encounter issues during migration:

1. Review the specific migration guide for your version
2. Check the [Troubleshooting](../user/troubleshooting.md) section
3. Consult the [API References](../reference/api-references.md)
4. Open an issue on [GitHub](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/issues)
5. Ask questions in the #cluster-api-ibmcloud channel on Kubernetes Slack

## Best Practices

When migrating between API versions:

1. **Test First**: Always test migrations in a non-production environment
2. **Read the Guide**: Review the complete migration guide before starting
3. **Backup Configurations**: Keep copies of your current configurations
4. **Incremental Migration**: Migrate one cluster at a time
5. **Monitor**: Watch cluster status during and after migration
6. **Update Automation**: Update any scripts or automation that reference the old API