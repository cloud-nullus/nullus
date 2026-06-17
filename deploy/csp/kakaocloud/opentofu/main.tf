# Nullus Air-Gap — Kakao Cloud 인프라
# builder VM (온라인, amd64 번들 빌드) + airgap VM (오프라인 설치 타겟)

locals {
  # cloud-init.yaml 을 base64 로 인코딩하여 두 VM 모두에 전달
  cloud_init_base64 = filebase64("${path.module}/cloud-init.yaml")
}

#####################################################################
# Module 1: Network - VPC 및 서브넷
#####################################################################
module "network" {
  source = "./modules/network"

  vpc_name                = var.vpc_name
  vpc_cidr                = var.vpc_cidr
  vpc_default_subnet_cidr = var.vpc_default_subnet_cidr
  subnet_name             = var.subnet_name
  subnet_cidr             = var.subnet_cidr
  availability_zone       = var.availability_zone
}

#####################################################################
# Module 2: Security - 보안 그룹
# 포트: 22(SSH), 80(HTTP), 443(HTTPS), VPC 내부 전체, 아웃바운드 전체.
#       registry(5001)·kind API(16443)는 VM 내부 127.0.0.1 바인딩 → 공개 미노출(SSH 터널)
#####################################################################
module "security" {
  source = "./modules/security"

  security_group_name = var.security_group_name
  vpc_cidr            = module.network.vpc_cidr
  web_allowed_cidr    = var.web_allowed_cidr
  depends_on          = [module.network]
}

#####################################################################
# Keypair - TF 가 신규 키페어를 생성한다 (계정에 미등록 키페어 대응).
#   private_key 는 생성 시점에만 반환되므로 즉시 로컬 .pem 으로 저장한다.
#####################################################################
resource "kakaocloud_keypair" "nullus" {
  name = var.key_name
}

resource "local_sensitive_file" "private_key" {
  content         = kakaocloud_keypair.nullus.private_key
  filename        = "${path.module}/${var.key_name}.pem"
  file_permission = "0600"
}

#####################################################################
# Module 3: Compute - builder + airgap VM
# 로드밸런서/provisioner 모듈 없음 — 스크립트(scripts/)로 핸드오프
#####################################################################
module "compute" {
  source = "./modules/compute"

  image_name          = var.image_name
  builder_flavor      = var.builder_flavor
  airgap_flavor       = var.airgap_flavor
  volume_size         = var.volume_size
  key_name            = kakaocloud_keypair.nullus.name # 생성된 키페어에 의존
  subnet_id           = module.network.subnet_id
  security_group_name = module.security.security_group_name
  cloud_init_base64   = local.cloud_init_base64
  depends_on          = [module.security, kakaocloud_keypair.nullus]
}
