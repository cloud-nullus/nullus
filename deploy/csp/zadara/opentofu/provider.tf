# Zadara zCompute 는 AWS EC2 호환 API 를 제공한다. 공식 가이드가 검증한 조합은
# AWS provider 3.33(공식 Kubernetes 모듈 제약: >=3.33.0, <=3.35.0)이므로 최신본을
# 무조건 올리지 않고 이 버전으로 고정한다. .terraform.lock.hcl 도 함께 커밋한다.
terraform {
  required_version = ">= 1.9.0, < 2.0.0"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "3.33.0"
    }
    external = {
      source  = "hashicorp/external"
      version = "2.3.5"
    }
    local = {
      source  = "hashicorp/local"
      version = "2.9.0"
    }
  }
}

provider "aws" {
  # 자격증명은 코드에 두지 않는다. ~/.aws/config 의 [profile zadara] 를 사용한다.
  profile = var.aws_profile
  region  = var.region

  # 사용하는 서비스 endpoint 를 모두 명시한다. 누락되면 AWS 실제 endpoint 로 향한다.
  endpoints {
    ec2 = var.ec2_endpoint
  }

  # zCompute 는 AWS 의 자격증명/메타데이터/계정조회 경로를 제공하지 않는다.
  # insecure(TLS 검증 해제)는 공식 예제에 있더라도 복사하지 않는다.
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_region_validation      = true
  skip_requesting_account_id  = true
}
