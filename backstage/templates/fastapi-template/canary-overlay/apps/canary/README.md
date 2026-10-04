---
type: note
title: "Private sense canary"
tags: [sense, canary]
status: active
created: 2026-10-03
updated: 2026-10-03
---

## Purpose

Generated from the FastAPI Golden Path in private-canary mode. This source is intended for the existing kensan-lab repository. No public hostname, HTTPRoute, gateway OAuth2 route, new repository, publication workflow, or catalog registration is generated.

## Acceptance endpoints

| Operation | Result |
|---|---|
| GET /health | healthy |
| GET /api/release | Image-owned release v1; the next candidate changes this to v2 |
| PUT /api/marker with JSON marker | Writes one bounded marker to /data and returns its SHA-256 |
| GET /api/marker | Reads the PVC marker; 404 before the first write |

The app-base chart mounts the existing canary-data PVC in app-canary. PVC and namespace carry Prune=false. Data is retained across pod replacement and image rollback.

## Publication and teardown

Export the Template Editor dry-run files under canary-output. Independently review the fixed source SHA, image digest, generated paths, private route evidence, and rollback before any publisher or cluster operation. Image build/push and deployment are separate Gate operations. No such operation was performed by generation.

Only cluster-internal or explicitly approved local forwarding can be used to test. Roll back to the previous immutable image, confirm release and health, and then remove only the bounded canary resources under the teardown contract. The local tests do not establish cluster health, external non-reachability, PVC persistence across a pod replacement, or a successful image rollback.
