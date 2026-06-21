# Network Module Variables

variable "vpc_name" {
  description = "VPC 이름"
  type        = string
  default     = "nullus-airgap"
}

variable "vpc_cidr" {
  description = "VPC CIDR 블록"
  type        = string
  default     = "172.16.0.0/16"
}

variable "vpc_default_subnet_cidr" {
  description = "VPC 기본 서브넷 CIDR"
  type        = string
  default     = "172.16.255.0/24"
}

variable "subnet_name" {
  description = "메인 서브넷 이름"
  type        = string
  default     = "main_subnet"
}

variable "subnet_cidr" {
  description = "메인 서브넷 CIDR 블록"
  type        = string
  default     = "172.16.0.0/24"
}

variable "availability_zone" {
  description = "가용 영역"
  type        = string
  default     = "kr-central-2-a"
}
