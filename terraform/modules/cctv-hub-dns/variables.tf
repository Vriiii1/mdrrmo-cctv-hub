# terraform/modules/cctv-hub-dns/variables.tf
variable "cloudflare_zone_id" { type = string }
variable "hub_subdomain"      { type = string }
variable "hub_ipv4"           { type = string }
variable "hub_ipv6" {
  type    = string
  default = ""
}
