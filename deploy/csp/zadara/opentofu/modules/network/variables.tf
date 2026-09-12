# Network Module Variables

variable "name_prefix" {
  description = "자원 이름/태그 접두어"
  type        = string
}

variable "vpc_cidr" {
  description = "VPC CIDR"
  type        = string
}

variable "public_subnet_cidr" {
  description = "IGW 경로를 가진 서브넷 CIDR"
  type        = string
}

variable "private_subnet_cidr" {
  description = "NAT 경로를 가진 서브넷 CIDR"
  type        = string
}

variable "availability_zone" {
  description = "가용 영역"
  type        = string
}

variable "enable_nat" {
  description = "private 서브넷 + NAT gateway 생성 여부. false 면 private 서브넷 자체를 만들지 않는다."
  type        = bool
  default     = true
}
