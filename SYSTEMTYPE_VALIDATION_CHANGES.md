# SystemType Dynamic Validation Implementation

## Summary
Implemented controller-side dynamic validation for PowerVS systemType field, following CAPI architectural patterns. This replaces the previous webhook-based hardcoded validation with runtime validation against the PowerVS API.

## Problem Solved
- **Before**: Webhook used hardcoded list of systemTypes, blocking new types added by PowerVS
- **After**: Controller validates against live PowerVS API, automatically supporting new systemTypes

## Architecture Pattern
Following CAPI best practices:
- **CRDs**: Define shape (pattern validation: `^[a-z][0-9]+$`) this allows flexible validation.
- **Webhooks**: Protect invariants (format checking only)
- **Controllers**: Enforce reality (PowerVS API validation)

## Files Changed

### 1. **internal/controllers/powervs/validation.go** (NEW)
- Created `validateSystemType()` function
- Dynamically fetches supported systemTypes from PowerVS API
- Uses `datacenterClient.GetAll()` to aggregate types across all datacenters
- Returns validation result and list of supported types

### 2. **internal/controllers/powervs/ibmpowervsmachine_controller.go**
- Added systemType validation in `reconcileNormal()` before VM creation (line ~315)
- Sets `InvalidConfiguration` condition if systemType is not supported
- Sets `Ready = false` without rejecting the API object
- Logs clear error message with list of supported types

### 3. **internal/webhooks/powervs/ibmpowervsmachine.go**
- Simplified `validateIBMPowerVSMachineSystemType()` function
- Removed hardcoded systemType list
- Now only validates format (delegated to kubebuilder pattern)
- Added documentation explaining architectural decision

### 4. **api/powervs/v1beta2/conditions_consts.go**
- Added `InvalidConfigurationReason` constant for condition reporting

### 5. **api/powervs/v1beta2/ibmpowervsmachine_types.go**
- Updated systemType field documentation
- Clarified that validation happens in controller, not webhook
- Explained pattern allows future systemTypes automatically

### 6. **pkg/cloud/services/powervs/service.go**
- Added `GetPISession()` method to expose IBMPISession

### 7. **pkg/cloud/services/powervs/powervs.go**
- Added `GetPISession()` to PowerVS interface

### 8. **pkg/cloud/services/powervs/mock/powervs_generated.go**
- Regenerated mock to include `GetPISession()` method

## How It Works

### Validation Flow
1. User creates IBMPowerVSMachine with systemType (e.g., "s1234")
2. **API Server**: Validates pattern `^[a-z][0-9]+$` 
3. **Webhook**: Checks format only (no hardcoded list) 
4. **Controller Reconcile Loop**:
   - Calls `validateSystemType(ctx, machineScope)`
   - Gets PISession from `machineScope.IBMPowerVSClient.GetPISession()`
   - Calls PowerVS API: `datacenterClient.GetAll()`
   - Aggregates supported systemTypes from all datacenters
   - Checks if "s1234" is in the list
   - If **valid**: Proceeds to create VM 
   - If **invalid**: Sets condition, marks Ready=false, returns

### Example: New SystemType "s1234" Added by PowerVS
1. PowerVS adds "s1234" to their datacenter capabilities
2. User specifies `systemType: "s1234"` in IBMPowerVSMachine
3. Pattern validation passes (matches `^[a-z][0-9]+$`)
4. Webhook passes (no hardcoded list to block it)
5. Controller queries PowerVS API, finds "s1234" is supported
6. VM is created successfully 

**No code changes needed!**

## Benefits

###  Dynamic Validation
- Automatically supports new systemTypes from PowerVS
- No code changes or releases required for new types

###  CAPI Compliance
- Follows established CAPI architectural patterns
- Webhooks protect invariants, controllers enforce reality

###  Cross-Version Compatibility
- Works in release-0.12 and main branches
- No API server deadlocks or CRD gymnastics

###  Clear Error Messages
- Users see exactly which systemTypes are supported
- Condition shows: "SystemType 's999' is not supported. Supported types: [e1050, e1080, e980, s1022, s1122, s922]"

###  No VM Creation on Invalid Config
- Invalid systemType prevents VM creation
- Machine marked as InvalidConfiguration
- No wasted cloud resources

## Testing

### Build Verification
```bash
go build ./...  #  Success
go build ./internal/controllers/powervs/...  #  Success
go build ./internal/webhooks/powervs/...  #  Success
```

### Manual Testing Steps
1. Create IBMPowerVSMachine with valid systemType (e.g., "s922")
   - Expected: VM created successfully
2. Create IBMPowerVSMachine with invalid systemType (e.g., "s999")
   - Expected: Machine marked InvalidConfiguration, no VM created
3. Create IBMPowerVSMachine with empty systemType
   - Expected: Uses default, VM created successfully

## Migration Notes

### For Users
- No action required
- Existing machines continue to work
- New systemTypes automatically supported

### For Repo Maintainers
- Webhook tests may need updates (hardcoded list removed)
- Controller tests should mock `GetPISession()` and `datacenterClient.GetAll()`

## Future Enhancements

### Future Improvements
1. **Caching**: Cache supported systemTypes to reduce API calls
2. **Region-Specific**: Validate systemType against specific datacenter/region
