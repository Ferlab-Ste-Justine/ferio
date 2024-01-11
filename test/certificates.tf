module "ca" {
  source = "./ca"
}

resource "tls_private_key" "key" {
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}

resource "tls_cert_request" "request" {
  private_key_pem = tls_private_key.key.private_key_pem
  ip_addresses    = [
    "127.0.0.1",
    "127.0.0.2",
    "127.0.0.3"
  ]
  subject {
    common_name  = "localhost"
    organization = "localhost"
  }
}

resource "tls_locally_signed_cert" "certificate" {
  cert_request_pem   = tls_cert_request.request.cert_request_pem
  ca_private_key_pem = module.ca.key
  ca_cert_pem        = module.ca.certificate

  validity_period_hours = 10000
  early_renewal_hours = 24

  allowed_uses = [
    "client_auth",
    "server_auth",
  ]

  is_ca_certificate = false
}

module "root_certificate" {
    source = "git::https://github.com/Ferlab-Ste-Justine/terraform-tls-client-certificate.git"
    ca = module.ca
    username = "root"
}

resource "local_file" "ca_cert" {
  content = module.ca.certificate
  filename = "${path.module}/certs/ca.crt"
  file_permission = "0600"
}

resource "local_file" "server_cert" {
  content = tls_locally_signed_cert.certificate.cert_pem
  filename = "${path.module}/certs/server.crt"
  file_permission = "0600"
}

resource "local_file" "server_key" {
  content = tls_private_key.key.private_key_pem
  filename = "${path.module}/certs/server.key"
  file_permission = "0600"
}

resource "local_file" "root_cert" {
  content = module.root_certificate.certificate
  filename = "${path.module}/certs/root.pem"
  file_permission = "0600"
}

resource "local_file" "root_key" {
  content = module.root_certificate.key
  filename = "${path.module}/certs/root.key"
  file_permission = "0600"
}