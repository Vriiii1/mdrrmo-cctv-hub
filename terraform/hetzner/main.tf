# terraform/hetzner/main.tf
terraform {
  required_providers {
    hcloud     = { source = "hetznercloud/hcloud",   version = "~> 1.49" }
    cloudflare = { source = "cloudflare/cloudflare", version = "~> 4.40" }
  }
}

provider "hcloud"     { token = var.hcloud_token }
provider "cloudflare" { api_token = var.cloudflare_token }

resource "hcloud_ssh_key" "ops" {
  name       = "mdrrmo-cctv-hub-ops"
  public_key = file(var.ops_public_key_path)
}

resource "hcloud_firewall" "hub" {
  name = "mdrrmo-cctv-hub"
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = [var.ops_bastion_cidr]
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "80"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "8443"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
  rule {
    direction  = "in"
    protocol   = "icmp"
    source_ips = ["0.0.0.0/0", "::/0"]
  }
}

resource "hcloud_server" "hub" {
  name         = "mdrrmo-cctv-hub-prod"
  image        = "ubuntu-22.04"
  server_type  = "ccx23"
  location     = "fsn1"
  ssh_keys     = [hcloud_ssh_key.ops.id]
  firewall_ids = [hcloud_firewall.hub.id]
  backups      = true
}

module "dns" {
  source             = "../modules/cctv-hub-dns"
  cloudflare_zone_id = var.cloudflare_zone_id
  hub_subdomain      = var.hub_subdomain
  hub_ipv4           = hcloud_server.hub.ipv4_address
  hub_ipv6           = hcloud_server.hub.ipv6_address
}

output "hub_ipv4" { value = hcloud_server.hub.ipv4_address }
output "hub_ipv6" { value = hcloud_server.hub.ipv6_address }
output "active_provider" { value = "hetzner" }
