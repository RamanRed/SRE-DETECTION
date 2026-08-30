################################################################################
# Terraform Variables
################################################################################

variable "aws_region" {
  description = "AWS region to deploy resources"
  type        = string
  default     = "eu-north-1"
}

variable "project_name" {
  description = "Project identifier used for resource naming"
  type        = string
  default     = "sre-copilot"
}

variable "vpc_cidr" {
  description = "CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
}

variable "public_subnet_cidr" {
  description = "CIDR for the public subnet (EC2)"
  type        = string
  default     = "10.0.1.0/24"
}

variable "private_subnet_a_cidr" {
  description = "CIDR for the first private subnet (RDS)"
  type        = string
  default     = "10.0.10.0/24"
}

variable "private_subnet_b_cidr" {
  description = "CIDR for the second private subnet (RDS AZ failover)"
  type        = string
  default     = "10.0.11.0/24"
}

variable "ec2_instance_type" {
  description = "EC2 instance type (t3.small provides additional K3s and rollout headroom)"
  type        = string
  default     = "t3.small"
}

variable "rds_instance_class" {
  description = "RDS instance class (cost-conscious default: db.t3.micro)"
  type        = string
  default     = "db.t3.micro"
}

variable "ec2_public_key" {
  description = "SSH public key material for EC2 key pair"
  type        = string
  sensitive   = true
}

variable "db_password" {
  description = "PostgreSQL master password (use strong secret)"
  type        = string
  sensitive   = true
}

variable "allowed_cidr" {
  description = "Trusted administrator IPv4 CIDR for SSH, K3s API, Prometheus, and Grafana (for example 1.2.3.4/32)"
  type        = string

  validation {
    condition = (
      can(cidrnetmask(var.allowed_cidr)) &&
      var.allowed_cidr != "0.0.0.0/0" &&
      var.allowed_cidr != "::/0"
    )
    error_message = "allowed_cidr must be a valid, non-global administrator IPv4 CIDR; 0.0.0.0/0 is forbidden."
  }
}

variable "public_ingress_cidrs" {
  description = "Trusted client IPv4 CIDRs permitted to reach Traefik on ports 80 and 443; keep this private until TLS and federated identity are configured"
  type        = list(string)

  validation {
    condition = (
      length(var.public_ingress_cidrs) > 0 &&
      alltrue([
        for cidr in var.public_ingress_cidrs :
        can(cidrnetmask(cidr)) && cidr != "0.0.0.0/0" && cidr != "::/0"
      ])
    )
    error_message = "public_ingress_cidrs must contain valid, non-global IPv4 CIDRs; global Internet exposure is blocked for the internal bootstrap-auth deployment."
  }
}

variable "enable_observability_ingress" {
  description = "Open the optional Prometheus/Grafana administration ports 3000-3001 to allowed_cidr"
  type        = bool
  default     = false
}

variable "common_tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default = {
    Project     = "sre-copilot"
    ManagedBy   = "terraform"
    Environment = "demo"
    Unit        = "DevOps-Training"
  }
}
