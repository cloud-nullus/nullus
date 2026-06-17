# Network Module Outputs

output "vpc_id" {
  description = "VPC ID"
  value       = kakaocloud_vpc.nullus_vpc.id
}

output "vpc_cidr" {
  description = "VPC CIDR 블록"
  value       = kakaocloud_vpc.nullus_vpc.cidr_block
}

output "subnet_id" {
  description = "메인 서브넷 ID"
  value       = kakaocloud_subnet.main_subnet.id
}

output "subnet_cidr" {
  description = "메인 서브넷 CIDR 블록"
  value       = kakaocloud_subnet.main_subnet.cidr_block
}

output "availability_zone" {
  description = "가용 영역"
  value       = kakaocloud_subnet.main_subnet.availability_zone
}
