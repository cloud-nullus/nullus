# Network Module Outputs

output "vpc_id" {
  description = "VPC ID"
  value       = aws_vpc.this.id
}

output "vpc_cidr" {
  description = "VPC CIDR"
  value       = aws_vpc.this.cidr_block
}

output "public_subnet_id" {
  description = "public 서브넷 ID"
  value       = aws_subnet.public.id
}

output "private_subnet_id" {
  description = "private 서브넷 ID. enable_nat = false 면 null."
  value       = one(aws_subnet.private[*].id)
}

output "nat_gateways" {
  description = "생성된 NAT gateway 목록. 길이 0 이면 private worker 의 egress 경로가 없다."
  value       = aws_nat_gateway.this
}
