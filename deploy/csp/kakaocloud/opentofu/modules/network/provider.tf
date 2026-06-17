# 모듈 provider 요구사항 — kakaocloud 리소스를 쓰므로 source 명시 필수.
# 미선언 시 terraform 이 namespace 를 hashicorp/kakaocloud 로 잘못 추론한다.
terraform {
  required_version = ">= 1.13.5"
  required_providers {
    kakaocloud = {
      source  = "kakaoenterprise/kakaocloud"
      version = "0.3.5"
    }
  }
}
