# Security Module Outputs

output "security_group_id" {
  description = "보안 그룹 ID"
  value       = kakaocloud_security_group.security_group.id
}

output "security_group_name" {
  description = "보안 그룹 이름"
  value       = kakaocloud_security_group.security_group.name
}
