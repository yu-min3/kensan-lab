---
type: note
title: "Private sense canary"
tags: [sense, canary]
status: active
created: 2026-10-03
updated: 2026-10-03
---

## Purpose

Source for a private acceptance canary in the existing kensan-lab repository. This change adds the application and a build/test workflow only. It does not create a repository, publish an image, register an Argo Application, apply RBAC, expose a route, or register a catalog entity. The catalog file is source metadata only.

## Acceptance endpoints

| Operation | Result |
|---|---|
| GET /health | healthy |
| GET /api/release | Image-owned release v1; the next candidate changes this to v2 |
| PUT /api/marker with JSON marker | Writes one bounded marker to /data and returns its SHA-256 |
| GET /api/marker | Reads the PVC marker; 404 before the first write |

The app can store markers under `/data`. A later, separately reviewed GitOps change must provide the `canary-data` PVC and `Prune=false` protection. This source change does not create that PVC or prove persistence across pod replacement or rollback.

## Publication and teardown

Independently review the fixed source SHA, image digest, private route evidence, and rollback before any publisher or cluster operation. The source/CI pull request contains no image-publishing job. Image build/push and deployment are separate Gate operations. No such operation was performed by generation.

Only cluster-internal or explicitly approved local forwarding can be used to test. Roll back to the previous immutable image, confirm release and health, and then remove only the bounded canary resources under the teardown contract. The local tests do not establish cluster health, external non-reachability, PVC persistence across a pod replacement, or a successful image rollback.
