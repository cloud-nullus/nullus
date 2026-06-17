# Compute Module Variables

variable "image_name" {
  description = "OS 이미지 이름"
  type        = string
  default     = "Ubuntu 24.04"
}

# builder VM: 인터넷에서 이미지 pull + amd64 번들 빌드 (t1i.large 충분)
variable "builder_flavor" {
  description = "Builder VM 인스턴스 플레이버 (amd64 Intel)"
  type        = string
  default     = "t1i.large"
}

# airgap VM: kind 클러스터 + 레지스트리 + Nullus 설치 (최소 4GB RAM, 15GB+ 디스크)
variable "airgap_flavor" {
  description = "Airgap VM 인스턴스 플레이버 (amd64 Intel)"
  type        = string
  default     = "t1i.xlarge"
}

variable "volume_size" {
  description = "부트 볼륨 크기 (GB, 최소 100 권장)"
  type        = number
  default     = 100
}

variable "key_name" {
  description = "SSH 키페어 이름"
  type        = string
}

variable "subnet_id" {
  description = "인스턴스를 연결할 서브넷 ID"
  type        = string
}

variable "security_group_name" {
  description = "적용할 보안 그룹 이름"
  type        = string
}

variable "cloud_init_base64" {
  description = "Base64 인코딩된 cloud-init 사용자 데이터"
  type        = string
}
