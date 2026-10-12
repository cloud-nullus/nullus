# Compute Module Variables

variable "name_prefix" {
  description = "자원 이름/태그 접두어"
  type        = string
}

variable "nodes" {
  description = <<-EOT
    루트에서 배치까지 확정한 노드 정의. 이 모듈은 배치를 다시 계산하지 않는다.
    key 가 VM 이름이자 Kubespray inventory 의 host 이름이다.
  EOT
  type = map(object({
    instance_type      = string
    disk_gb            = number
    role               = string
    subnet_id          = string
    private_ip         = string
    public             = bool
    security_group_ids = list(string)
  }))
}

variable "image_id" {
  description = "확정한 이미지 ID"
  type        = string
}

variable "key_name" {
  description = "등록된 keypair 이름"
  type        = string
}

variable "user_data" {
  description = "cloud-init 사용자 데이터 (평문)"
  type        = string
}
