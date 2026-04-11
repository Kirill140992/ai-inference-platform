resource "hcloud_server" "k3s_master" {
  name        = var.server_name
  image       = "ubuntu-24.04"
  server_type = "ccx23"
  location    = "nbg1"

  user_data = templatefile("${path.module}/templates/user-data.sh.tpl", {
    k3s_version    = var.k3s_version
    argocd_version = var.argocd_version
  })

  lifecycle {
    ignore_changes = [
      image, 
      user_data, 
      ssh_keys, 
      server_type, 
      location
    ]
  }
}