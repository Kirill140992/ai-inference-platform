provider "aws" {
  region = "us-east-1"
}

resource "aws_security_group" "allow_ssh" {
  name        = "allow_ssh"
  description = "Allow SSH inbound traffic"
  vpc_id      = "vpc-123456"

  ingress {
    description = "SSH from anywhere"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"] # Checkov это возненавидит
  }
}

resource "aws_s3_bucket" "qdrant_backups" {
  bucket = "ai-platform-qdrant-backups-dev"
}

resource "aws_s3_bucket_acl" "qdrant_backups_acl" {
  bucket = aws_s3_bucket.qdrant_backups.id
  acl    = "public-read"
}