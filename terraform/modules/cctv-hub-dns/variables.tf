# terraform/modules/cctv-hub-dns/variables.tf
variable "cloudflare_zone_id" { type = string }
variable "hub_subdomain"      { type = string }
variable "hub_ipv4"           { type = string }
variable "hub_ipv6" {
  type    = string
  default = ""
}
variable "rtsps_subdomain" {
  type    = string
  default = "rtsps"
  description = "Subdomain for the DNS-only (proxied=false) RTSPS record. Cloudflare orange-cloud cannot proxy raw TLS on :8443."
}
