# Security Module Outputs

output "node_security_group_id" {
  description = "모든 노드에 붙이는 보안 그룹"
  value       = aws_security_group.node.id
}

output "bastion_security_group_id" {
  description = "운영자 SSH 진입점 보안 그룹 (control plane 전용)"
  value       = aws_security_group.bastion.id
}

output "web_security_group_id" {
  description = "웹 진입점 보안 그룹 (공인 IP 보유 노드에 부착)"
  value       = aws_security_group.web.id
}

output "web_rules" {
  description = "웹 개방 규칙. 기본은 0개이며 web_cidrs 를 지정해야 늘어난다."
  value       = aws_security_group_rule.web
}

output "ssh_rule_cidrs" {
  description = "bastion SG 의 22/tcp 인바운드 출발지. 테스트에서 admin_cidrs 반영을 확인한다."
  value       = aws_security_group_rule.ssh.cidr_blocks
}
