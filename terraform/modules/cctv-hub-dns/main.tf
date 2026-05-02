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

# DNS-only records for RTSPS (raw TLS on :8443).
# Cloudflare orange-cloud only proxies HTTP(S) — raw TLS packets are dropped at
# the edge. Camera agents must publish to rtsps.<zone> (not hub.<zone>) so the
# connection reaches the server directly without Cloudflare interception.
resource "cloudflare_record" "rtsps_a" {
  zone_id = var.cloudflare_zone_id
  name    = var.rtsps_subdomain
  type    = "A"
  value   = var.hub_ipv4
  proxied = false
  ttl     = 60
}

resource "cloudflare_record" "rtsps_aaaa" {
  count   = var.hub_ipv6 == "" ? 0 : 1
  zone_id = var.cloudflare_zone_id
  name    = var.rtsps_subdomain
  type    = "AAAA"
  value   = var.hub_ipv6
  proxied = false
  ttl     = 60
}
