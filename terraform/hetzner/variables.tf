variable "hcloud_token" {
  description = "Hetzner Cloud API Token"
  type        = string
  sensitive   = true
}

variable "server_name" {
  description = "ai-platform"
  type        = string
  default     = "ai-platform"
}

variable "k3s_version" {
  description = "Version of k3s to install"
  default     = "v1.30.0+k3s1" 
}

variable "argocd_version" {
  description = "Version of ArgoCD to install"
  default     = "v2.10.4"
}