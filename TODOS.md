# Future Improvements

## Store Plugin: Check-and-Set (CAS) / Optimistic Concurrency

The `Save` method currently has no mechanism for optimistic concurrency control.
If two hosts call `Save` concurrently for the same tree ID, the last write wins
silently. Backends like HashiCorp Vault KV v2 support Check-and-Set (CAS) where
a write must specify the expected current version to succeed, preventing
accidental overwrites.

A future revision could extend `Save` with an optional expected version parameter
(e.g. `Save(ctx, id, tree, expectedVersion)`) or introduce a separate
`SaveWithCAS` method to surface version conflicts back to the caller.

## Store Plugin: Tree-Level Metadata

The store plugin currently has no concept of metadata attached to a tree ID
itself (as opposed to per-value metadata within the tree). Backends like
HashiCorp Vault KV v2 support custom metadata on the key level, separate from
version data — for example, ownership info, labels, or access policies.

A future revision could add methods like `GetTreeMetadata(ctx, id)` and
`SetTreeMetadata(ctx, id, metadata)` to allow plugins to expose and manage
tree-level metadata.
