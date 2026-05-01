# terraform/hetzner/variables.tf
variable "hcloud_token" {
  type      = string
  sensitive = true
}
variable "cloudflare_token" {
  type      = string
  sensitive = true
}
variable "cloudflare_zone_id" { type = string }
variable "hub_subdomain" {
  type    = string
  default = "hub"
}
variable "ops_public_key_path" { type = string }
variable "ops_bastion_cidr"    { type = string }
