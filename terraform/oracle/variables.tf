# terraform/oracle/variables.tf
variable "tenancy_ocid"        { type = string }
variable "user_ocid"           { type = string }
variable "api_key_fingerprint" { type = string }
variable "api_key_path" {
  type    = string
  default = "~/.oci/oci_api_key.pem"
}
variable "compartment_ocid" { type = string }
variable "region" {
  type    = string
  default = "ap-singapore-1"
}
variable "availability_domain" { type = string }
variable "ops_public_key_path" { type = string }
variable "ops_bastion_cidr"    { type = string }
variable "cloudflare_token" {
  type      = string
  sensitive = true
}
variable "cloudflare_zone_id" { type = string }
variable "hub_subdomain" {
  type    = string
  default = "hub"
}
