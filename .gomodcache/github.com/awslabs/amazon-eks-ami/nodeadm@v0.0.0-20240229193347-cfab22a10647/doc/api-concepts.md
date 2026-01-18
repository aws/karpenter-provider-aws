# API Concepts

## Versioning

The API types for `nodeadm` (the `node.eks.aws` API group) are versioned in a similar manner to the [Kubernetes API](https://kubernetes.io/docs/reference/using-api/#api-versioning).

There are three levels of stability and support:

### Alpha
- Example: `v1alpha2`.
- Support for an alpha API may be removed at any time.
- Subsequent alpha API versions may include incompatible changes, and migration instructions may not be provided.

### Beta
- Example: `v3beta4`.
- Support for a beta API will remain for at least one release following its deprecation.
- Subsequent beta or stable API versions may include incompatible changes, and migration instructions will be provided.

### Stable
- Example: `v5`.
- Support for a stable API will align with the support of a major version of Amazon Linux.