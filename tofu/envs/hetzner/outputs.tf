output "kubeconfig" {
  description = "Kubeconfig administrateur (tofu output -raw kubeconfig > ~/.kube/hetzner.yaml ; chmod 600)."
  value       = module.kube_hetzner.kubeconfig
  sensitive   = true
}

output "control_plane_ipv4" {
  description = "IP publiques des nœuds (cible des enregistrements DNS Cloudflare gérés par external-dns)."
  value       = module.kube_hetzner.control_planes_public_ipv4
}

output "tailscale_hosts" {
  value = module.kube_hetzner.tailscale_control_plane_magicdns_hosts
}

output "buckets" {
  value = { for k, b in aws_s3_bucket.platform : k => b.bucket }
}

output "s3_endpoint" {
  value = "https://${local.s3_endpoint_host}"
}
