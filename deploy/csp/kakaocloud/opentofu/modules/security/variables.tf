# Security Module Variables

variable "security_group_name" {
  description = "보안 그룹 이름"
  type        = string
}

variable "description" {
  description = "보안 그룹 설명"
  type        = string
  default     = "Nullus Air-Gap Terraform managed security group"
}

variable "vpc_cidr" {
  description = "VPC CIDR 블록 (내부 트래픽 규칙용)"
  type        = string
  default     = "172.16.0.0/16"
}

variable "web_allowed_cidr" {
  description = "웹(80) 외부 접근 허용 CIDR. 운영자 IP 로 제한 권장 (예: x.x.x.x/32). 0.0.0.0/0 = 전체 공개."
  type        = string
  default     = "0.0.0.0/0"
}
