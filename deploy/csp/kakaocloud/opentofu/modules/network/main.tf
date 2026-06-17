# Network Module - VPC 및 서브넷
# Nullus Air-Gap 배포용 네트워크 리소스

# VPC 생성 (생성 시 약 5분 이상 소요). name/cidr/기본서브넷은 tfvars 의 vpc_* 로 제어.
resource "kakaocloud_vpc" "nullus_vpc" {
  name       = var.vpc_name
  cidr_block = var.vpc_cidr

  # VPC 내 기본 서브넷 설정
  subnet = {
    cidr_block        = var.vpc_default_subnet_cidr
    availability_zone = var.availability_zone
  }
}

# 메인 서브넷 생성
resource "kakaocloud_subnet" "main_subnet" {
  name              = var.subnet_name
  cidr_block        = var.subnet_cidr
  availability_zone = var.availability_zone
  vpc_id            = kakaocloud_vpc.nullus_vpc.id
}
