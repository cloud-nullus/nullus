# D6: zCompute의 단일 type filter가 무시돼 AWS data source를 직접 쓸 수 없다.
# 전체 카탈로그를 읽고 정확한 이름 1건과 CPU/RAM을 대조한다.
data "external" "catalog" {
  program = ["python3", "${path.module}/scripts/instance_types.py"]
  query = {
    profile  = var.aws_profile
    region   = var.region
    endpoint = var.ec2_endpoint
    types    = jsonencode(distinct([for n in values(var.nodes) : n.instance_type]))
  }
}
locals {
  catalog         = jsondecode(data.external.catalog.result.catalog_json)
  actual_capacity = { for name, n in var.nodes : name => local.catalog[n.instance_type] }
}
resource "terraform_data" "flavors" {
  input = local.actual_capacity
  lifecycle {
    precondition {
      condition     = alltrue([for name, n in var.nodes : n.vcpus == local.actual_capacity[name].vcpus && n.memory_gb == local.actual_capacity[name].memory_gb])
      error_message = "nodes의 CPU/RAM이 실제 instance type과 다릅니다. describe-instance-types 결과에 맞추세요."
    }
  }
}
