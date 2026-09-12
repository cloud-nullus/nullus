# Security Module Variables

variable "name_prefix" {
  description = "자원 이름/태그 접두어"
  type        = string
}

variable "vpc_id" {
  description = "보안 그룹을 만들 VPC"
  type        = string
}

variable "admin_cidrs" {
  description = "운영자 SSH 허용 출발지"
  type        = list(string)
}

variable "web_cidrs" {
  description = "웹 진입점 허용 출발지. 비어 있으면 규칙을 만들지 않는다."
  type        = list(string)
  default     = []
}

variable "web_ports" {
  description = "web_cidrs 에 개방할 포트"
  type        = list(number)
  default     = [80, 443]
}
