# terraform/modules/cctv-hub-dns/main.tf
terraform {
  required_providers {
    cloudflare = { source = "cloudflare/cloudflare", version = "~> 4.40" }
  }
}

resource "cloudflare_record" "hub_a" {
  zone_id = var.cloudflare_zone_id
  name    = var.hub_subdomain
  type    = "A"
  value   = var.hub_ipv4
  proxied = true
  ttl     = 1
}

resource "cloudflare_record" "hub_aaaa" {
  count   = var.hub_ipv6 == "" ? 0 : 1
  zone_id = var.cloudflare_zone_id
  name    = var.hub_subdomain
  type    = "AAAA"
  value   = var.hub_ipv6
  proxied = true
  ttl     = 1
}
