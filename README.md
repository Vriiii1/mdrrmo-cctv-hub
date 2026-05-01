# mdrrmo-cctv-hub

Single-server self-hosted CCTV hub for MDRRMO. Sibling repo to [mdrrmo_app](https://github.com/Vriiii1/mdrrmo_app).

## What this is
- Hetzner CCX23 EU + Caddy + MediaMTX + audit-shim, all in Docker Compose.
- Validates publish-JWTs and viewer-JWTs minted by `mdrrmo_app/super-admin-landing/src/lib/cctv/stream-token.ts`.
- Source of truth for keys: `https://<dashboard>/.well-known/jwks.json`.
- Owns no persistent data; full state ephemeral.

## Quickstart (local dev)
See `docs/local-dev.md` (created in Phase 4).

## Production deployment
See `docs/production-deployment.md` (created in Phase 7).

## Source spec
[mdrrmo_app/docs/superpowers/specs/2026-04-29-cctv-hub-deployment-design.md](../mdrrmo_app/docs/superpowers/specs/2026-04-29-cctv-hub-deployment-design.md)
