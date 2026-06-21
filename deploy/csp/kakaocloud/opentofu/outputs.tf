# Root Outputs - Nullus Air-Gap Kakao Cloud

#####################################################################
# Network Outputs
#####################################################################
output "vpc_id" {
  description = "VPC ID"
  value       = module.network.vpc_id
}

output "subnet_id" {
  description = "메인 서브넷 ID"
  value       = module.network.subnet_id
}

#####################################################################
# Security Outputs
#####################################################################
output "security_group_id" {
  description = "보안 그룹 ID"
  value       = module.security.security_group_id
}

#####################################################################
# Compute Outputs
#####################################################################
output "builder_public_ip" {
  description = "Builder VM 공인 IP"
  value       = module.compute.builder_public_ip
}

output "builder_private_ip" {
  description = "Builder VM 사설 IP"
  value       = module.compute.builder_private_ip
}

output "airgap_public_ip" {
  description = "Airgap VM 공인 IP"
  value       = module.compute.airgap_public_ip
}

output "airgap_private_ip" {
  description = "Airgap VM 사설 IP"
  value       = module.compute.airgap_private_ip
}

#####################################################################
# SSH 접속 커맨드 (복사용)
#####################################################################
output "ssh_builder" {
  description = "Builder VM SSH 접속 커맨드"
  value       = "ssh -i ${local_sensitive_file.private_key.filename} ubuntu@${module.compute.builder_public_ip}"
}

output "ssh_airgap" {
  description = "Airgap VM SSH 접속 커맨드"
  value       = "ssh -i ${local_sensitive_file.private_key.filename} ubuntu@${module.compute.airgap_public_ip}"
}

#####################################################################
# 다음 단계 안내
#####################################################################
output "next_steps" {
  description = "프로비저닝 완료 후 실행할 스크립트 순서"
  value = {
    step1 = "BUILDER_IP=${module.compute.builder_public_ip} AIRGAP_IP=${module.compute.airgap_public_ip} SSH_KEY=${local_sensitive_file.private_key.filename} ../scripts/10-build-on-builder.sh"
    step2 = "BUILDER_IP=${module.compute.builder_public_ip} AIRGAP_IP=${module.compute.airgap_public_ip} SSH_KEY=${local_sensitive_file.private_key.filename} ../scripts/20-transfer-bundle.sh"
    step3 = "AIRGAP_IP=${module.compute.airgap_public_ip} SSH_KEY=${local_sensitive_file.private_key.filename} ../scripts/30-install-on-airgap.sh"
  }
}
