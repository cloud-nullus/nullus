# Compute Module Outputs

output "instances" {
  description = "생성된 인스턴스 (VM 이름 → 리소스)"
  value       = aws_instance.node
}

output "private_ips" {
  description = "노드별 사설 IP"
  value       = { for name, i in aws_instance.node : name => i.private_ip }
}

output "instance_ids" {
  description = "노드별 인스턴스 ID"
  value       = { for name, i in aws_instance.node : name => i.id }
}

output "security_group_ids" {
  description = "노드별로 부착한 보안 그룹 (중복 제거 전 목록). plan 검토와 테스트에서 배치 결정을 확인한다."
  value       = { for name, n in var.nodes : name => n.security_group_ids }
}
