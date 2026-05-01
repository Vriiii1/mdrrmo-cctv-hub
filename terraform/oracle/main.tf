# terraform/oracle/main.tf
terraform {
  required_providers {
    oci        = { source = "oracle/oci",            version = "~> 5.40" }
    cloudflare = { source = "cloudflare/cloudflare", version = "~> 4.40" }
  }
}

provider "oci" {
  tenancy_ocid = var.tenancy_ocid
  user_ocid    = var.user_ocid
  fingerprint  = var.api_key_fingerprint
  private_key_path = var.api_key_path
  region       = var.region
}

provider "cloudflare" { api_token = var.cloudflare_token }

# --- Network: VCN, subnet, internet gateway, route table, security list ---
resource "oci_core_vcn" "hub" {
  compartment_id = var.compartment_ocid
  display_name   = "mdrrmo-cctv-hub-vcn"
  cidr_blocks    = ["10.0.0.0/16"]
}

resource "oci_core_internet_gateway" "hub" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.hub.id
  display_name   = "mdrrmo-cctv-hub-igw"
  enabled        = true
}

resource "oci_core_route_table" "hub" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.hub.id
  display_name   = "mdrrmo-cctv-hub-rt"
  route_rules {
    destination       = "0.0.0.0/0"
    destination_type  = "CIDR_BLOCK"
    network_entity_id = oci_core_internet_gateway.hub.id
  }
}

resource "oci_core_security_list" "hub" {
  compartment_id = var.compartment_ocid
  vcn_id         = oci_core_vcn.hub.id
  display_name   = "mdrrmo-cctv-hub-sl"

  egress_security_rules {
    destination = "0.0.0.0/0"
    protocol    = "all"
  }

  ingress_security_rules {  # SSH from ops bastion only
    source   = var.ops_bastion_cidr
    protocol = "6"  # TCP
    tcp_options {
      min = 22
      max = 22
    }
  }
  ingress_security_rules {  # 80 (LE HTTP-01)
    source   = "0.0.0.0/0"
    protocol = "6"
    tcp_options {
      min = 80
      max = 80
    }
  }
  ingress_security_rules {  # 443 (Caddy HTTPS)
    source   = "0.0.0.0/0"
    protocol = "6"
    tcp_options {
      min = 443
      max = 443
    }
  }
  ingress_security_rules {  # 8443 (RTSPS publish)
    source   = "0.0.0.0/0"
    protocol = "6"
    tcp_options {
      min = 8443
      max = 8443
    }
  }
  ingress_security_rules {  # ICMP
    source   = "0.0.0.0/0"
    protocol = "1"
  }
}

resource "oci_core_subnet" "hub" {
  compartment_id      = var.compartment_ocid
  vcn_id              = oci_core_vcn.hub.id
  display_name        = "mdrrmo-cctv-hub-subnet"
  cidr_block          = "10.0.1.0/24"
  route_table_id      = oci_core_route_table.hub.id
  security_list_ids   = [oci_core_security_list.hub.id]
  prohibit_public_ip_on_vnic = false
}

# --- Compute: Ampere A1 ARM Always Free ---
data "oci_core_images" "ubuntu_22_arm" {
  compartment_id           = var.compartment_ocid
  operating_system         = "Canonical Ubuntu"
  operating_system_version = "22.04"
  shape                    = "VM.Standard.A1.Flex"
  sort_by                  = "TIMECREATED"
  sort_order               = "DESC"
}

resource "oci_core_instance" "hub" {
  compartment_id      = var.compartment_ocid
  availability_domain = var.availability_domain
  display_name        = "mdrrmo-cctv-hub-staging"
  shape               = "VM.Standard.A1.Flex"

  shape_config {
    ocpus         = 4
    memory_in_gbs = 24
  }

  source_details {
    source_type = "image"
    source_id   = data.oci_core_images.ubuntu_22_arm.images[0].id
  }

  create_vnic_details {
    subnet_id        = oci_core_subnet.hub.id
    assign_public_ip = true
  }

  metadata = {
    ssh_authorized_keys = file(var.ops_public_key_path)
  }
}

# --- Cloudflare DNS via shared module ---
module "dns" {
  source             = "../modules/cctv-hub-dns"
  cloudflare_zone_id = var.cloudflare_zone_id
  hub_subdomain      = var.hub_subdomain
  hub_ipv4           = oci_core_instance.hub.public_ip
  # OCI Always Free does not allocate IPv6 by default; leave blank.
}

output "hub_ipv4" { value = oci_core_instance.hub.public_ip }
output "hub_ipv6" { value = "" }
output "active_provider" { value = "oracle" }
