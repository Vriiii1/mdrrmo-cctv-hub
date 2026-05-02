# terraform/modules/cctv-hub-dns/outputs.tf
output "hub_ipv4" { value = var.hub_ipv4 }
output "hub_ipv6" { value = var.hub_ipv6 }
output "rtsps_fqdn" {
  description = "FQDN camera agents publish to (DNS-only, no Cloudflare proxy). Used by camera-agent provisioning."
  value       = "${var.rtsps_subdomain}"
}
