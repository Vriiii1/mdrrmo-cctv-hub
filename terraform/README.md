# Terraform layout

- `modules/cctv-hub-dns/` — provider-agnostic Cloudflare DNS records (A + optional AAAA).
- `oracle/` — Oracle Cloud Always Free (Ampere A1 ARM) — testing/staging profile.
- `hetzner/` — Hetzner CCX23 EU (x86) — production profile.

## Which to use

Set `<CLOUD_PROVIDER>` per the Cloud Provider Strategy section in the deployment plan.
`cd terraform/<CLOUD_PROVIDER>/`, then `terraform init && terraform plan && terraform apply`.

## Cutover (OCI → Hetzner)

See Phase 6.5 in the deployment plan. TL;DR:

```
cd terraform/hetzner
terraform init && terraform apply
# DNS records are now updated to Hetzner IP
cd ../oracle
terraform destroy
```

DO NOT have both providers' DNS records active simultaneously — the shared module's
zone+name pair will conflict. Apply Hetzner first, then destroy Oracle (the destroy
removes its module-managed DNS records).
