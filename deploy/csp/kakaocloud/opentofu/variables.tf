# Root Variables - Nullus Air-Gap on Kakao Cloud

# ─── Kakao Cloud 인증 ─────────────────────────────────────────────────────────

variable "application_credential_id" {
  description = "Kakao Cloud Application Credential ID"
  type        = string
  sensitive   = true
}

variable "application_credential_secret" {
  description = "Kakao Cloud Application Credential Secret"
  type        = string
  sensitive   = true
}

# ─── 네트워크 ─────────────────────────────────────────────────────────────────

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

# ─── 보안 ─────────────────────────────────────────────────────────────────────

variable "security_group_name" {
  description = "보안 그룹 이름"
  type        = string
  default     = "nullus-airgap-sg"
}

variable "web_allowed_cidr" {
  description = "웹(80) 외부 접근 허용 CIDR. 운영자 IP/32 로 제한 권장. 0.0.0.0/0 = 전체 공개."
  type        = string
  default     = "0.0.0.0/0"
}

# ─── 컴퓨트 ───────────────────────────────────────────────────────────────────

variable "key_name" {
  description = "Kakao Cloud 에 등록된 SSH 키페어 이름"
  type        = string
}

variable "ssh_key_path" {
  description = "로컬 SSH 개인키 경로 (scripts/ 에서 사용)"
  type        = string
  default     = "~/.ssh/id_rsa"
}

variable "image_name" {
  description = "OS 이미지 이름"
  type        = string
  default     = "Ubuntu 24.04"
}

# amd64 Intel 플레이버 기본값 (t1i.*: x86_64/amd64 전용)
# arm64 번들(로컬 dist/)은 사용 불가 — builder 가 amd64 를 직접 빌드함
variable "builder_flavor" {
  description = "Builder VM 플레이버 (amd64 Intel, t1i.*) — 4c/16GB = t1i.xlarge"
  type        = string
  default     = "t1i.xlarge"
}

variable "airgap_flavor" {
  description = "Airgap VM 플레이버 (amd64 Intel, t1i.*) — kind+레지스트리+Nullus 용 8c/32GB = t1i.2xlarge"
  type        = string
  default     = "t1i.2xlarge"
}

variable "volume_size" {
  description = "부트 볼륨 크기 (GB)"
  type        = number
  default     = 500

  validation {
    condition     = var.volume_size >= 50 && var.volume_size <= 1000
    error_message = "volume_size 는 50~1000 GB 사이여야 합니다."
  }
}
