# Secure COS Ignition Bootstrap via HMAC Pre-signed URLs

## Metadata
* **Authors:** Karthik-K-N
* **Status:** Implemented
* **Creation Date:** 2026-08-14
* **Target Version:** v1beta3

## Summary
This proposal describes how the PowerVS machine bootstrap workflow was changed to
eliminate an IAM Bearer token that was being embedded in Ignition user-data. The fix
provisions a per-cluster COS HMAC service credential during cluster reconciliation,
stores it in a Kubernetes Secret, and uses it to generate a short-lived SigV4
pre-signed URL at machine-create time. The bootstrapping node fetches its Ignition
config with a plain HTTP GET — no credential of any kind is present in user-data.

## Motivation
During machine bootstrap, `ignitionUserData()` fetched a raw IAM Bearer token and
embedded it directly in the Ignition JSON config passed to every new node as
cloud-init user-data:

```go
auth, err := authenticator.GetIAMAuthenticator()
iamtoken, err := auth.GetToken()
token := "Bearer " + iamtoken

// Embedded in Ignition v2/v3 config:
HTTPHeaders: ignV2Types.HTTPHeaders{{Name: "Authorization", Value: token}}
```

This approach has three compounding problems:

1. **Long-lived credential in user-data:** The IAM Bearer token is serialised into the
   Ignition JSON that lives as cloud-init user-data on every new instance. On many
   platforms, user-data is accessible from the instance metadata endpoint without
   authentication. Any process on the node, or anyone who could read the bootstrap
   Secret on the management cluster, obtained a valid IAM Bearer token.

2. **Overly broad scope:** The token is derived from `IBMCLOUD_API_KEY`. It carries the
   full IAM permissions of that key — far beyond reading a single COS object.

3. **No tight expiry:** IAM tokens live approximately one hour and are renewable. The
   exposure window is not bounded to the single bootstrap GET.

### Why a Naïve Pre-signed URL Does Not Work With IAM Credentials

The standard S3-compatible fix — calling `req.Presign(1 * time.Hour)` on a
`GetObjectRequest` — does not work with IBM COS when the client uses IAM
(`IBMCLOUD_API_KEY`) credentials.

IBM COS SDK v1.x routes signing on `ProviderType`:

| ProviderType | Signer | Mechanism |
|---|---|---|
| `""` / `"v4"` | `v4.SignRequestHandler` | HMAC-SHA256 query-string parameters |
| **`"oauth"`** | `ibmiam.SignRequestHandler` | Sets `Authorization: Bearer <token>` header |

`ibmiam.NewStaticCredentials(...)` sets `ProviderType = "oauth"`. The IBM OAuth signer
sets an HTTP **header**, not query parameters. `req.Presign()` with an OAuth credential
produces a URL with no embedded authentication — IBM COS returns 403 when a node makes
a plain GET without the `Authorization` header.

Pre-signed URLs only work with HMAC credentials (SigV4 signer, `ProviderType = ""`).

## Goals
* Eliminate the IAM Bearer token from Ignition user-data entirely.
* Generate a time-limited, self-authenticating pre-signed URL using HMAC SigV4 credentials.
* Store HMAC credentials in a Kubernetes Secret managed by the cluster controller.
* Clean up the HMAC Secret during cluster deletion.
* Add the RBAC permissions required to create and delete Secrets.

## Non-Goals
* Changing the COS instance provisioning or bucket management logic.
* Supporting any credential mechanism other than HMAC for pre-signed URL generation.
* Rotating or revoking HMAC credentials during the cluster lifetime.

---

## Proposal

### 1. HMAC Service Credential Lifecycle in the Cluster Reconciler

**Current Drawback:** `ReconcileCOSInstance` sets up the COS bucket but leaves
`ignitionUserData` to fetch a fresh IAM token on every machine reconcile and embed it
in the Ignition config.

**Solution:** Add a new `reconcileCOSHMACKey` step at the end of `ReconcileCOSInstance`.
After the bucket is confirmed ready, the cluster reconciler calls
`ResourceClient.CreateResourceKey` with `parameters.SetProperty("HMAC", true)` to
obtain a `(access_key_id, secret_access_key)` pair scoped to the COS instance. These
are stored in a Kubernetes Secret named `<cluster>-cos-hmac` in the cluster's namespace.
The Secret name is written back to `cluster.Status.COSInstance.HMACSecretName`, making
it discoverable by the machine reconciler without any shared state.

*Before (cluster reconcile end):*
```go
// ReconcileCOSInstance returned after reconciling the bucket.
// ignitionUserData later called GetIAMAuthenticator().GetToken() directly.
```

*After (cluster reconcile end):*
```go
// Step 6: Reconcile the HMAC key Secret for secure ignition pre-signed URL support.
if err := s.reconcileCOSHMACKey(ctx, instanceCRN); err != nil {
    return fmt.Errorf("failed to reconcile COS HMAC key: %w", err)
}
```

*Idempotency:* If `HMACSecretName` is already set and the Secret exists, the step
returns immediately without making an IBM Cloud API call.

### 2. HMAC Secret Deletion on Cluster Teardown

**Current Drawback:** `DeleteCOSInstance` issued a recursive IBM Cloud API delete but
had no knowledge of any Kubernetes Secret to clean up.

**Solution:** `DeleteCOSInstance` now deletes the `<cluster>-cos-hmac` Secret before
issuing the IBM Cloud recursive instance delete. The deletion is best-effort — a failure
is logged but does not block the cloud resource deletion.

```go
// Step 5: Delete the HMAC Secret from Kubernetes (best-effort).
if cluster.Status.COSInstance.HMACSecretName != "" {
    // ... get and delete the Secret
}
// Step 6: Issue recursive DeleteResourceInstance API call.
```

### 3. HMAC-Based COS Client in the Machine Reconciler

**Current Drawback:** `ignitionUserData` called `GetIAMAuthenticator().GetToken()` and
embedded the resulting Bearer token directly in the Ignition redirect document's
`HTTPHeaders`.

**Solution:** A new `createCOSClient` helper reads `HMACSecretName` from
`IBMPowerVSCluster.Status.COSInstance`, fetches the Secret, and builds a COS client via
`cos.NewServiceWithHMAC(accessKeyID, secretAccessKey)`. This client uses
`credentials.NewStaticCredentials(...)` which selects the SigV4 signer (`ProviderType
= ""`), enabling `req.Presign()` to embed the HMAC signature as query parameters.

*Before:*
```go
auth, err := authenticator.GetIAMAuthenticator()
iamtoken, err := auth.GetToken()
token := "Bearer " + iamtoken
// token embedded in Ignition HTTPHeaders
```

*After:*
```go
cosClient, err := s.createCOSClient(ctx)  // reads HMAC Secret, SigV4 signer
presignedURL, err := cosClient.PresignedURL(bucket, key, presignExpiry)
// presignedURL embedded as plain Source — no HTTPHeaders, no credential
```

The resulting Ignition redirect document carries only a URL:

```go
// Ignition v3 — no HTTPHeaders field at all
ignData := &ignV3Types.Config{
    Ignition: ignV3Types.Ignition{
        Config: ignV3Types.IgnitionConfig{
            Replace: ignV3Types.Resource{
                Source: aws.String(presignedURL),
            },
        },
    },
}
```

### 4. New `PresignedURL` Method on the `Cos` Interface

**Current Drawback:** The `Cos` interface only exposed `GetObjectRequest`, which
returned a raw `*request.Request`. Callers had to call `.Presign()` themselves and
handle the OAuth-vs-SigV4 signer distinction inline.

**Solution:** A new `PresignedURL(bucket, key string, expiry time.Duration) (string,
error)` method is added to the `Cos` interface and implemented in `Service`. This
encapsulates the `req.Presign(expiry)` call and the error wrapping, and is mockable via
the generated mock.

```go
// In Cos interface
PresignedURL(bucket, key string, expiry time.Duration) (string, error)

// In Service
func (s *Service) PresignedURL(bucket, key string, expiry time.Duration) (string, error) {
    req, _ := s.client.GetObjectRequest(&s3.GetObjectInput{
        Bucket: aws.String(bucket),
        Key:    aws.String(key),
    })
    return req.Presign(expiry)
}
```

### 5. New `HMACSecretName` Field in `COSInstanceStatus`

**Current Drawback:** `COSInstanceStatus` tracked `ID`, `Name`, `BucketName`, and
`BucketRegion` but had no field to carry the name of the HMAC Secret to the machine
reconciler.

**Solution:** A new `HMACSecretName` field is added to `COSInstanceStatus`:

```go
// hmacSecretName is the name of the Kubernetes Secret (in the same namespace as the
// IBMPowerVSCluster) that holds the HMAC credentials for the COS instance.
// The Secret contains keys "access_key_id" and "secret_access_key".
// +optional
// +kubebuilder:validation:MinLength=1
// +kubebuilder:validation:MaxLength=253
HMACSecretName string `json:"hmacSecretName,omitempty"`
```

### 6. RBAC for Secret Management

**Current Drawback:** The cluster controller's RBAC markers only covered reading
Secrets (`get;list;watch`), which was insufficient once the controller needed to create
and delete the HMAC Secret.

**Solution:** The RBAC marker on `IBMPowerVSClusterReconciler` is extended to include
`create` and `delete`:

```go
// +kubebuilder:rbac:groups="",resources=secrets,verbs=create;delete;get;list;watch
```

This is reflected in `config/rbac/role.yaml`:

```yaml
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - create
  - delete
  - get
  - list
  - watch
```

---

## User Experience (UX)

### Before — IAM Bearer token in user-data

The bootstrap node received an Ignition config that looked like this (simplified):

```json
{
  "ignition": {
    "config": {
      "replace": {
        "source": "https://s3.us-south.cloud-object-storage.appdomain.cloud/bucket/control-plane/node1",
        "httpHeaders": [{ "name": "Authorization", "value": "Bearer eyJhb..." }]
      }
    }
  }
}
```

The `eyJhb...` token was a full-scope IAM Bearer token valid for ~1 hour, scoped to
all resources the `IBMCLOUD_API_KEY` can access.

### After — Self-authenticating pre-signed URL

```json
{
  "ignition": {
    "config": {
      "replace": {
        "source": "https://s3.us-south.cloud-object-storage.appdomain.cloud/bucket/control-plane/node1?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=...&X-Amz-Expires=3600&X-Amz-Signature=..."
      }
    }
  }
}
```

No credential is present. The HMAC signature embedded in the query string expires after
one hour. After expiry the URL is worthless even if the user-data is read.

---

## Security Properties

| Property | Before | After |
|---|---|---|
| Credential in user-data | ✗ Full IAM Bearer token | ✓ None — URL is self-authenticating |
| Blast radius if user-data leaked | All IAM-permitted APIs | Zero — URL expires in 1 hour |
| Pre-signed URL mechanism | Not possible (OAuth signer) | ✓ HMAC SigV4 query-string signature |
| Kubernetes Secret required | ✗ No | ✓ Yes — `<cluster>-cos-hmac` |
| Secret deleted on cluster teardown | N/A | ✓ Yes (best-effort) |

---

## Alternatives Considered

* **Scoped Service ID with IAM Policy:** Create a per-cluster IBM Cloud Service ID with
  a Reader-only IAM policy on the specific bucket, derive a token from its API key, and
  embed that in Ignition as before. Rejected because a Bearer token is still present in
  user-data — the root cause is not eliminated. HMAC pre-signing removes the credential
  entirely at comparable implementation complexity.

* **Public Object + Lifecycle Expiry:** Make the ignition object public-read for the
  bootstrap window, relying on the URL being hard to guess. Rejected because bootstrap
  configs contain sensitive cluster data (certificates, kubeconfig fragments). Making
  them publicly accessible, even temporarily, is not acceptable for production use.
