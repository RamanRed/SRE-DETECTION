################################################################################
# SRE Copilot Platform — Terraform AWS Infrastructure
# Provisions: VPC, Public Subnet, Security Groups, EC2, RDS PostgreSQL
# Zero-Cost: Uses only AWS Free Tier eligible resources
# Target: Single t3.micro EC2 running K3s + db.t3.micro RDS PostgreSQL
################################################################################

terraform {
  required_version = ">= 1.5.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# ──────────────────────────────────────────────────────────────────────────────
# Data Sources
# ──────────────────────────────────────────────────────────────────────────────

data "aws_availability_zones" "available" {
  state = "available"
}

data "aws_ami" "ubuntu" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }
  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# ──────────────────────────────────────────────────────────────────────────────
# VPC and Networking
# ──────────────────────────────────────────────────────────────────────────────

resource "aws_vpc" "sre_vpc" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(var.common_tags, { Name = "${var.project_name}-vpc" })
}

resource "aws_internet_gateway" "igw" {
  vpc_id = aws_vpc.sre_vpc.id
  tags   = merge(var.common_tags, { Name = "${var.project_name}-igw" })
}

# Public subnet for EC2 (K3s node)
resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.sre_vpc.id
  cidr_block              = var.public_subnet_cidr
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = merge(var.common_tags, { Name = "${var.project_name}-public-subnet" })
}

# Private subnets for RDS (2 AZs required for subnet group)
resource "aws_subnet" "private_a" {
  vpc_id            = aws_vpc.sre_vpc.id
  cidr_block        = var.private_subnet_a_cidr
  availability_zone = data.aws_availability_zones.available.names[0]
  tags              = merge(var.common_tags, { Name = "${var.project_name}-private-a" })
}

resource "aws_subnet" "private_b" {
  vpc_id            = aws_vpc.sre_vpc.id
  cidr_block        = var.private_subnet_b_cidr
  availability_zone = data.aws_availability_zones.available.names[1]
  tags              = merge(var.common_tags, { Name = "${var.project_name}-private-b" })
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.sre_vpc.id
  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.igw.id
  }
  tags = merge(var.common_tags, { Name = "${var.project_name}-public-rt" })
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# ──────────────────────────────────────────────────────────────────────────────
# Security Groups
# ──────────────────────────────────────────────────────────────────────────────

resource "aws_security_group" "ec2_sg" {
  name        = "${var.project_name}-ec2-sg"
  description = "SRE Copilot EC2 - Allow SSH, HTTP, HTTPS, K8s NodePort"
  vpc_id      = aws_vpc.sre_vpc.id

  ingress {
    description = "SSH"
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }
  ingress {
    description = "HTTP"
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description = "HTTPS"
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description = "K3s API Server"
    from_port   = 6443
    to_port     = 6443
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }
  ingress {
    description = "Incident Service REST"
    from_port   = 8081
    to_port     = 8081
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }
  ingress {
    description = "AI Copilot gRPC"
    from_port   = 9090
    to_port     = 9090
    protocol    = "tcp"
    cidr_blocks = [var.vpc_cidr]
  }
  ingress {
    description = "Prometheus/Grafana"
    from_port   = 3000
    to_port     = 3001
    protocol    = "tcp"
    cidr_blocks = [var.allowed_cidr]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.common_tags, { Name = "${var.project_name}-ec2-sg" })
}

resource "aws_security_group" "rds_sg" {
  name        = "${var.project_name}-rds-sg"
  description = "SRE Copilot RDS - Only EC2 can reach PostgreSQL"
  vpc_id      = aws_vpc.sre_vpc.id

  ingress {
    description     = "PostgreSQL from EC2"
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ec2_sg.id]
  }
  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(var.common_tags, { Name = "${var.project_name}-rds-sg" })
}

# ──────────────────────────────────────────────────────────────────────────────
# EC2 Instance (Free Tier: t3.micro, 750h/month for 12 months)
# ──────────────────────────────────────────────────────────────────────────────

resource "aws_key_pair" "sre_key" {
  key_name   = "${var.project_name}-key"
  public_key = var.ec2_public_key
  tags       = var.common_tags
}

resource "aws_instance" "sre_node" {
  ami                    = data.aws_ami.ubuntu.id
  instance_type          = var.ec2_instance_type
  key_name               = aws_key_pair.sre_key.key_name
  subnet_id              = aws_subnet.public.id
  vpc_security_group_ids = [aws_security_group.ec2_sg.id]

  root_block_device {
    volume_type = "gp3"
    volume_size = 30
    encrypted   = true
  }

  # 2GB swap for K3s + Spring Boot on t3.micro
  user_data = <<-EOF
    #!/bin/bash
    set -ex
    # Swap for lightweight K8s
    fallocate -l 2G /swapfile
    chmod 600 /swapfile
    mkswap /swapfile
    swapon /swapfile
    echo '/swapfile none swap sw 0 0' >> /etc/fstab
    # Install git and curl
    apt-get update -qq
    apt-get install -y git curl wget unzip
    echo "EC2 bootstrap complete" > /tmp/bootstrap.log
  EOF

  tags = merge(var.common_tags, { Name = "${var.project_name}-k3s-node" })
}

resource "aws_eip" "sre_eip" {
  instance = aws_instance.sre_node.id
  domain   = "vpc"
  tags     = merge(var.common_tags, { Name = "${var.project_name}-eip" })
}

# ──────────────────────────────────────────────────────────────────────────────
# RDS PostgreSQL (Free Tier: db.t3.micro, 750h/month for 12 months)
# IMPORTANT: Use Single-AZ; avoid Multi-AZ and Aurora Serverless (both cost $)
# ──────────────────────────────────────────────────────────────────────────────

resource "aws_db_subnet_group" "rds_subnets" {
  name       = "${var.project_name}-rds-subnet-group"
  subnet_ids = [aws_subnet.private_a.id, aws_subnet.private_b.id]
  tags       = merge(var.common_tags, { Name = "${var.project_name}-rds-subnets" })
}

resource "aws_db_instance" "postgres" {
  identifier              = "${var.project_name}-postgres"
  engine                  = "postgres"
  engine_version          = "16.3"
  instance_class          = var.rds_instance_class  # db.t3.micro (Free Tier)
  allocated_storage       = 20
  max_allocated_storage   = 25
  storage_type            = "gp2"
  storage_encrypted       = true
  db_name                 = "sredb"
  username                = "sreuser"
  password                = var.db_password
  db_subnet_group_name    = aws_db_subnet_group.rds_subnets.name
  vpc_security_group_ids  = [aws_security_group.rds_sg.id]
  publicly_accessible     = false
  skip_final_snapshot     = true
  backup_retention_period = 1
  multi_az                = false  # Single-AZ = Free Tier safe
  deletion_protection     = false

  tags = merge(var.common_tags, { Name = "${var.project_name}-rds" })
}
