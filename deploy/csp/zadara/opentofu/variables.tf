variable "ec2_endpoint" {
  description = "Zadara EC2 HTTPS endpoint; AWS 기본 endpoint로 fallback하지 않는다."
  type        = string
  validation {
    condition     = can(regex("^https://[^/]+/api/v2/aws/ec2/?$", var.ec2_endpoint))
    error_message = "Zadara EC2 HTTPS endpoint를 지정해야 합니다."
  }
}
variable "aws_profile" {
  type    = string
  default = "zadara"
}
variable "region" {
  type    = string
  default = "symphony"
}
variable "availability_zone" { type = string }
variable "image_id" { type = string }
variable "key_name" {
  description = "이미 등록된 SSH keypair 이름; 개인키를 state에 저장하지 않는다."
  type        = string
}
variable "ssh_private_key_file" {
  description = "로컬 개인키 경로. ~는 pathexpand로 확장하고 공백/셸 특수문자는 거부한다."
  type        = string
  validation {
    condition     = can(regex("^(~/|/)[A-Za-z0-9_./-]+$", var.ssh_private_key_file))
    error_message = "~/ 또는 /로 시작하는 공백/특수문자 없는 개인키 경로가 필요합니다."
  }
}
variable "ssh_user" {
  type    = string
  default = "ubuntu"
  validation {
    condition     = can(regex("^[a-z_][a-z0-9_-]*$", var.ssh_user))
    error_message = "유효한 SSH 사용자 이름이어야 합니다."
  }
}
variable "name_prefix" {
  type    = string
  default = "nullus-poc-v2"
  validation {
    condition     = can(regex("^[a-z][a-z0-9-]{2,30}$", var.name_prefix))
    error_message = "name_prefix는 영문 소문자/숫자/하이픈 3~31자입니다."
  }
}
variable "vpc_cidr" {
  description = "기존 VPC/VPN/Pod/Service 대역과 겹치지 않는 새 IPv4 /16."
  type        = string
  default     = "10.20.0.0/16"
  validation {
    condition     = can(cidrnetmask(var.vpc_cidr)) && can(regex("/16$", var.vpc_cidr))
    error_message = "IPv4 /16 CIDR가 필요합니다."
  }
}
variable "admin_cidrs" {
  type        = set(string)
  description = "SSH(22) 허용 IPv4 CIDR. 운영자가 여럿·유동 IP 라 0.0.0.0/0 도 허용한다(2026-09-13 결정). 인증은 keypair 전용(비밀번호 로그인 없음)."
  validation {
    condition     = length(var.admin_cidrs) > 0 && alltrue([for c in var.admin_cidrs : can(cidrnetmask(c))])
    error_message = "SSH 허용 IPv4 CIDR를 하나 이상 지정하세요."
  }
}
variable "web_cidrs" {
  type        = set(string)
  default     = []
  description = "m1의 30080/30443 접근 CIDR. 기본은 외부 웹 접근 차단."
  validation {
    condition     = alltrue([for c in var.web_cidrs : can(cidrnetmask(c))])
    error_message = "web_cidrs는 IPv4 CIDR이어야 합니다."
  }
}
variable "worker_egress" {
  type        = string
  default     = "nat"
  description = "nat: private w1 + managed NAT / public_ip: w1 EIP, 외부 SSH 차단."
  validation {
    condition     = contains(["nat", "public_ip"], var.worker_egress)
    error_message = "worker_egress는 nat 또는 public_ip입니다."
  }
}
variable "capacity_limit" {
  description = "현재 사용량 합계를 잠정 한도로 사용. 실제 quota 증거에 따라 조정한다."
  type        = object({ vcpus = number, memory_gb = number, disk_gb = number })
  default     = { vcpus = 20, memory_gb = 40, disk_gb = 400 }
  validation {
    condition     = var.capacity_limit.vcpus > 0 && var.capacity_limit.memory_gb > 0 && var.capacity_limit.disk_gb > 0
    error_message = "capacity_limit은 양수여야 합니다."
  }
}
variable "nodes" {
  description = "카탈로그의 실제 instance_type과 CPU/RAM을 대조한 노드 정의."
  type        = map(object({ instance_type = string, vcpus = number, memory_gb = number, disk_gb = number, role = optional(string), private_ip = optional(string) }))
  validation {
    condition     = length(var.nodes) >= 2 && alltrue([for name, n in var.nodes : can(regex("^[a-z][a-z0-9-]*$", name)) && contains(["control_plane", "worker"], coalesce(n.role, startswith(name, "m") ? "control_plane" : startswith(name, "w") ? "worker" : "unknown")) && length(trimspace(n.instance_type)) > 0 && n.vcpus >= 2 && floor(n.vcpus) == n.vcpus && n.memory_gb >= 4 && n.disk_gb >= 20 && floor(n.disk_gb) == n.disk_gb]) && length([for name, n in var.nodes : name if coalesce(n.role, startswith(name, "m") ? "control_plane" : "worker") == "control_plane"]) == 1 && sum([for n in values(var.nodes) : n.vcpus]) <= var.capacity_limit.vcpus && sum([for n in values(var.nodes) : n.memory_gb]) <= var.capacity_limit.memory_gb && sum([for n in values(var.nodes) : n.disk_gb]) <= var.capacity_limit.disk_gb
    error_message = "control plane 1대와 worker가 필요합니다. 역할/사양을 확인하고 capacity_limit 이내로 배분하세요."
  }
}
variable "public_subnet_cidr" {
  type    = string
  default = "10.20.0.0/24"
  validation {
    condition     = can(cidrnetmask(var.public_subnet_cidr)) && can(regex("/24$", var.public_subnet_cidr)) && try(cidrcontains(var.vpc_cidr, cidrhost(var.public_subnet_cidr, 0)), false)
    error_message = "public subnet은 VPC 내 IPv4 /24여야 합니다."
  }
}
variable "private_subnet_cidr" {
  type    = string
  default = "10.20.1.0/24"
  validation {
    condition     = can(cidrnetmask(var.private_subnet_cidr)) && can(regex("/24$", var.private_subnet_cidr)) && try(cidrcontains(var.vpc_cidr, cidrhost(var.private_subnet_cidr, 0)), false) && try(cidrhost(var.public_subnet_cidr, 0) != cidrhost(var.private_subnet_cidr, 0), false)
    error_message = "private subnet은 VPC 내 별개의 IPv4 /24여야 합니다."
  }
}
variable "web_ports" {
  type        = list(number)
  default     = [80, 443]
  description = "웹 진입점 포트. 80/443은 별도 forwarding/LB가 필요하고 IaC는 listener를 설치하지 않는다."
  validation {
    condition     = alltrue([for p in var.web_ports : contains([80, 443, 30080, 30443], p)])
    error_message = "80/443/30080/30443 중 웹 포트만 지정하세요."
  }
}
variable "schedule_on_control_plane" {
  description = "자원 활용을 위해 master에도 스케줄링. 격리가 필요하면 false."
  type        = bool
  default     = true
}
