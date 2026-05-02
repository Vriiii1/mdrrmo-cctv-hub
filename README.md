# mdrrmo-cctv-hub

Single-server self-hosted CCTV hub for MDRRMO. Sibling repo to [mdrrmo_app](https://github.com/Vriiii1/mdrrmo_app).

## What this is
- Hetzner CCX23 EU + Caddy + MediaMTX + auth-shim, all in Docker Compose.
- Validates publish-JWTs and viewer-JWTs minted by `mdrrmo_app/super-admin-landing/src/lib/cctv/stream-token.ts`.
- Source of truth for keys: `https://<dashboard>/.well-known/jwks.json`.
- Owns no persistent data; full state ephemeral.

## Prerequisites before `docker compose up`

### RTSPS TLS certificates (required for RTSPS on :8443)

MediaMTX reads TLS material from `mediamtx/certs/`. Before starting the stack, place:

```
mediamtx/certs/server.crt   # PEM-encoded certificate (chain or self-signed)
mediamtx/certs/server.key   # PEM-encoded private key
```

These files are bind-mounted into the mediamtx container at `/certs/` (read-only).
They are gitignored — do not commit private keys.

> **Phase 3 note:** Automated cert provisioning via Caddy's cert exchange is out of scope for Phase 1.
> For now, generate or copy your cert/key pair manually before starting.

### Stack profile (`CADDY_PROFILE`)

Copy `.env.example` to `.env` and set `CADDY_PROFILE` to `path-a` or `path-b` (default: `path-b`).
Do not rename this variable to `COMPOSE_PROFILES` — that name is reserved by Docker Compose.

## Quickstart (local dev)
See `docs/local-dev.md` (created in Phase 4).

## Production deployment
See `docs/production-deployment.md` (created in Phase 7).

## Source spec
[mdrrmo_app/docs/superpowers/specs/2026-04-29-cctv-hub-deployment-design.md](../mdrrmo_app/docs/superpowers/specs/2026-04-29-cctv-hub-deployment-design.md)
