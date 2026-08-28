################################################################################
# Terraform Outputs
################################################################################

output "ec2_public_ip" {
  description = "Elastic IP address of the SRE K3s node"
  value       = aws_eip.sre_eip.public_ip
}

output "ec2_public_dns" {
  description = "Public DNS hostname for EC2 instance"
  value       = aws_instance.sre_node.public_dns
}

output "rds_endpoint" {
  description = "RDS PostgreSQL connection endpoint"
  value       = aws_db_instance.postgres.endpoint
}

output "rds_database_name" {
  description = "PostgreSQL database name"
  value       = aws_db_instance.postgres.db_name
}

output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.sre_vpc.id
}

output "ssh_command" {
  description = "SSH command to connect to EC2 node"
  value       = "ssh -i ~/.ssh/sre-copilot-key ubuntu@${aws_eip.sre_eip.public_ip}"
}

output "dashboard_url" {
  description = "SRE Copilot Dashboard URL after deployment"
  value       = "http://${aws_eip.sre_eip.public_ip}"
}
