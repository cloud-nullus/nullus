# Compute Module - VM 인스턴스
# builder (온라인) + airgap (오프라인 타겟) 2대 프로비저닝

# 이미지 및 플레이버 데이터 소스 (이름으로 ID 조회)
data "kakaocloud_images" "images_all" {}

data "kakaocloud_instance_flavors" "flavors_all" {}

locals {
  ubuntu24_id = [
    for image in data.kakaocloud_images.images_all.images : image.id
    if image.name == var.image_name
  ][0]

  builder_flavor_id = [
    for flavor in data.kakaocloud_instance_flavors.flavors_all.instance_flavors : flavor.id
    if flavor.name == var.builder_flavor
  ][0]

  airgap_flavor_id = [
    for flavor in data.kakaocloud_instance_flavors.flavors_all.instance_flavors : flavor.id
    if flavor.name == var.airgap_flavor
  ][0]
}

# Builder VM: 인터넷 접근 가능, amd64 번들 빌드 담당
resource "kakaocloud_instance" "builder" {
  name        = "nullus-builder"
  description = "Nullus Air-Gap 번들 빌더 (온라인, amd64 빌드 전용)"
  flavor_id   = local.builder_flavor_id
  image_id    = local.ubuntu24_id
  key_name    = var.key_name

  subnets = [{ id = var.subnet_id }]

  initial_security_groups = [{
    name = var.security_group_name
  }]

  volumes = [{ size = var.volume_size }]

  user_data = var.cloud_init_base64

  # image_id/flavor_id 는 data-source 이름조회 결과라 재apply 시 (known after apply) 로
  # churn 되어 인스턴스 강제 교체를 유발한다. 기존 VM 보존을 위해 변경 무시한다.
  # (이미지/플레이버를 의도적으로 바꿔 재생성하려면 이 ignore_changes 를 제거)
  lifecycle {
    ignore_changes = [image_id, flavor_id]
  }
}

# Builder VM 공인 IP
resource "kakaocloud_public_ip" "builder_ip" {
  description = "Public IP for nullus-builder"

  related_resource = {
    id          = kakaocloud_instance.builder.addresses[0].network_interface_id
    device_id   = kakaocloud_instance.builder.id
    device_type = "instance"
  }
  depends_on = [kakaocloud_instance.builder]
}

# Airgap VM: 오프라인 타겟, 번들 수신 후 kind 클러스터 + Nullus 설치
resource "kakaocloud_instance" "airgap" {
  name        = "nullus-airgap"
  description = "Nullus Air-Gap 설치 대상 VM (kind 클러스터 + 레지스트리)"
  flavor_id   = local.airgap_flavor_id
  image_id    = local.ubuntu24_id
  key_name    = var.key_name

  subnets = [{ id = var.subnet_id }]

  initial_security_groups = [{
    name = var.security_group_name
  }]

  volumes = [{ size = var.volume_size }]

  user_data = var.cloud_init_base64

  # image_id/flavor_id churn 으로 인한 강제 교체 방지 (기존 VM 보존). builder 와 동일 사유.
  lifecycle {
    ignore_changes = [image_id, flavor_id]
  }
}

# Airgap VM 공인 IP
resource "kakaocloud_public_ip" "airgap_ip" {
  description = "Public IP for nullus-airgap"

  related_resource = {
    id          = kakaocloud_instance.airgap.addresses[0].network_interface_id
    device_id   = kakaocloud_instance.airgap.id
    device_type = "instance"
  }
  depends_on = [kakaocloud_instance.airgap]
}
