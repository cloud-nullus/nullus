# Compute Module Outputs

output "builder_public_ip" {
  description = "Builder VM 공인 IP"
  value       = kakaocloud_public_ip.builder_ip.public_ip
}

output "builder_private_ip" {
  description = "Builder VM 사설 IP"
  value       = kakaocloud_instance.builder.addresses[0].private_ip
}

output "airgap_public_ip" {
  description = "Airgap VM 공인 IP"
  value       = kakaocloud_public_ip.airgap_ip.public_ip
}

output "airgap_private_ip" {
  description = "Airgap VM 사설 IP"
  value       = kakaocloud_instance.airgap.addresses[0].private_ip
}

output "builder_instance_id" {
  description = "Builder VM 인스턴스 ID"
  value       = kakaocloud_instance.builder.id
}

output "airgap_instance_id" {
  description = "Airgap VM 인스턴스 ID"
  value       = kakaocloud_instance.airgap.id
}
