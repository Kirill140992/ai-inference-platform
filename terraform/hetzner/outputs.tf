output "server_ip" {
  description = "Публичный IPv4 адрес нашего сервера (k3s master node)"
  value       = hcloud_server.k3s_master.ipv4_address
}

output "server_status" {
  description = "Текущий статус сервера"
  value       = hcloud_server.k3s_master.status
}