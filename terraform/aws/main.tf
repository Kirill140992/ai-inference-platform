provider "aws" {
  region = "us-east-1"
}

resource "aws_security_group" "allow_ssh" {
  name        = "allow_ssh"
  description = "Allow SSH inbound traffic"
  vpc_id      = "vpc-123456"

# checkov:skip=CKV2_AWS_5: Security group is provisioned for future EC2 instances

  ingress {
    description = "SSH from corporate VPN"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["10.0.0.0/8"]
  }
}

resource "aws_s3_bucket" "qdrant_backups" {
  bucket = "ai-platform-qdrant-backups-dev"

  # checkov:skip=CKV_AWS_18: Access logging is overkill for dev environment
  # checkov:skip=CKV_AWS_144: Cross region replication is not needed for dev
  # checkov:skip=CKV_AWS_300: Lifecycle config is not required
  # checkov:skip=CKV2_AWS_62: Event notifications are not required
  # checkov:skip=CKV2_AWS_6: Public access block is configured in a separate resource below
  # checkov:skip=CKV_AWS_145: KMS encryption is too expensive for dev, using AES256
  # checkov:skip=CKV2_AWS_61: Lifecycle configuration is not required for dev
}

resource "aws_s3_bucket_server_side_encryption_configuration"  "qdrant_backups_encryption" {
    bucket = aws_s3_bucket.qdrant_backups.id
    rule {
        apply_server_side_encryption_by_default {
            sse_algorithm = "AES256"
        }
    }
}

resource "aws_s3_bucket_versioning" "qdrant_backups_versioning" {
    bucket = aws_s3_bucket.qdrant_backups.id
    versioning_configuration {
        status = "Enabled"
    }
}

resource "aws_s3_bucket_public_access_block" "qdrant_backups_public_access" {
   bucket                  = aws_s3_bucket.qdrant_backups.id
   block_public_acls       = true
   block_public_policy     = true
   ignore_public_acls      = true
   restrict_public_buckets = true
}

terraform {
  cloud {
    organization = "kier-che-inc"
    workspaces {
      name = "ai-platform-aws"
    }
  }
}