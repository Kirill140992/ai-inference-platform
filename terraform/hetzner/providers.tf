terraform {
    cloud {
        organization = "kier-che-inc"
        workspaces {
            name = "ai-platform-prod"
        }
    }
    required_providers {
        hcloud = {
            source = "hetznercloud/hcloud"
            version = "~> 1.45"
        }
    }
}

provider "hcloud" {
    token = var.hcloud_token
}