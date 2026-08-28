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
  description = "EC2 instance type (Free Tier: t3.micro)"
  type        = string
  default     = "t3.micro"
}

variable "rds_instance_class" {
  description = "RDS instance class (Free Tier: db.t3.micro)"
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
  description = "Your IP CIDR for SSH and admin access (e.g. 1.2.3.4/32)"
  type        = string
  default     = "0.0.0.0/0"
}

variable "common_tags" {
  description = "Common tags applied to all resources"
  type        = map(string)
  default = {
    Project     = "sre-copilot"
    ManagedBy   = "terraform"
    Environment = "production"
    Unit        = "DevOps-Training"
  }
}
