# API 호출만 mock한다. 네트워크/인벤토리/입력 검증은 실제 모듈을 평가한다.
mock_provider "aws" {}
mock_provider "local" {}

mock_provider "external" {}
override_data {
  target = data.external.catalog
  values = {
    result = { catalog_json = "{\"test-master\":{\"vcpus\":4,\"memory_gb\":8},\"test-worker\":{\"vcpus\":16,\"memory_gb\":32}}" }
  }
}

variables {
  # terraform.tfvars(운영 값) 가 자동 로드되므로 테스트가 기대하는 기본 상태를 못박는다.
  web_cidrs            = []
  ec2_endpoint         = "https://zcompute.example.test/api/v2/aws/ec2"
  availability_zone    = "test-zone"
  image_id             = "ami-12345678"
  key_name             = "test-existing-key"
  ssh_private_key_file = "/tmp/test-key"
  admin_cidrs          = ["192.0.2.10/32"]
  nodes = {
    m1 = { instance_type = "test-master", vcpus = 4, memory_gb = 8, disk_gb = 100 }
    w1 = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 300 }
  }
}

run "private_worker_and_inventory" {
  command = plan
  assert {
    condition     = output.capacity == { vcpus = 20, memory_gb = 40, disk_gb = 400 }
    error_message = "m1+w1 must stay within the supported capacity."
  }
  assert {
    condition     = module.compute.instances["w1"].associate_public_ip_address == false && length(aws_eip.public) == 1
    error_message = "Private worker must not receive a public IP."
  }
  assert {
    condition     = length(module.network.nat_gateways) == 1
    error_message = "A private worker needs a working outbound route."
  }
  assert {
    condition     = strcontains(output.kubespray_inventory, "[kube_control_plane]\nm1\n") && strcontains(output.kubespray_inventory, "[etcd]\nm1\n")
    error_message = "m1 must own the control plane and etcd."
  }
  assert {
    condition     = strcontains(output.kubespray_inventory, "[kube_node]\nm1\nw1\n")
    error_message = "이 PoC 는 m1 에도 워크로드를 배치한다. control plane 이 kube_node 에 함께 있어야 Kubespray 가 NoSchedule taint 를 걸지 않는다."
  }
  assert {
    condition     = length(module.security.web_rules) == 0
    error_message = "Web NodePorts must not be public by default."
  }
}

run "public_worker_without_nat" {
  command = plan
  variables { worker_egress = "public_ip" }
  assert {
    condition     = length(module.network.nat_gateways) == 0 && length(aws_eip.public) == 2
    error_message = "Public-IP egress must not also allocate a NAT gateway."
  }
  assert {
    condition     = length(module.compute.security_group_ids["w1"]) == 2
    error_message = "공인 IP 를 받은 worker 에는 web 보안 그룹도 붙어야 한다."
  }
}

run "reject_excess_capacity" {
  command = plan
  variables {
    nodes = {
      m1 = { instance_type = "test-master", vcpus = 4, memory_gb = 8, disk_gb = 100 }
      w1 = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 301 }
    }
  }
  expect_failures = [var.nodes]
}

run "allow_public_ssh" {
  # 운영자가 여럿이라 SSH 출발지를 고정하지 않는다. 대신 SG 는 22 만 열고 키 인증만 허용한다.
  command = plan
  variables { admin_cidrs = ["0.0.0.0/0"] }
  assert {
    condition     = length(module.security.ssh_rule_cidrs) == 1 && contains(module.security.ssh_rule_cidrs, "0.0.0.0/0")
    error_message = "admin_cidrs 의 0.0.0.0/0 이 bastion SSH 규칙에 그대로 반영되어야 한다."
  }
}

run "reject_http_endpoint" {
  command = plan
  variables { ec2_endpoint = "http://zcompute.example.test/api/v2/aws/ec2" }
  expect_failures = [var.ec2_endpoint]
}

run "deterministic_private_ips" {
  command = plan
  assert {
    condition     = output.nodes["m1"].private_ip == "10.20.0.10" && output.nodes["w1"].private_ip == "10.20.1.10"
    error_message = "apply 전에 검토하려면 사설 IP 가 plan 시점에 확정되어야 한다."
  }
  assert {
    condition     = output.nodes["m1"].subnet == "public" && output.nodes["w1"].subnet == "private"
    error_message = "control plane 은 public, nat 모드의 worker 는 private 서브넷이어야 한다."
  }
  assert {
    condition     = length(module.security.web_rules) == 0 && length(module.compute.instances) == 2
    error_message = "노드 수와 웹 규칙 수가 입력과 일치해야 한다."
  }
}

run "web_opens_only_requested_sources" {
  command = plan
  variables {
    web_cidrs = ["198.51.100.5/32"]
    web_ports = [443]
  }
  assert {
    condition     = length(module.security.web_rules) == 1
    error_message = "web_cidrs x web_ports 조합만큼만 규칙을 만들어야 한다."
  }
  # mock provider 는 모든 보안 그룹에 같은 가짜 id 를 주므로 id 비교 대신 부착 개수를 본다.
  # m1 = node + bastion + web(3), w1 = node(1).
  assert {
    condition     = length(module.compute.security_group_ids["m1"]) == 3
    error_message = "nat 모드에서 공인 IP 는 m1 뿐이다. web 규칙이 m1 에 붙지 않으면 web_cidrs 가 무의미해진다."
  }
  assert {
    condition     = length(module.compute.security_group_ids["w1"]) == 1
    error_message = "공인 IP 가 없는 private worker 에는 node 보안 그룹만 붙인다."
  }
}

run "explicit_role_overrides_name_rule" {
  command = plan
  variables {
    nodes = {
      cp = { instance_type = "test-master", vcpus = 4, memory_gb = 8, disk_gb = 100, role = "control_plane" }
      wk = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 300, role = "worker" }
    }
  }
  assert {
    condition     = strcontains(output.kubespray_inventory, "[kube_control_plane]\ncp\n") && strcontains(output.kubespray_inventory, "[kube_node]\ncp\nwk\n")
    error_message = "role 을 명시하면 m*/w* 이름 규칙 밖에서도 배치가 결정되어야 한다."
  }
}

run "reject_unnamed_role" {
  command = plan
  variables {
    nodes = {
      node1 = { instance_type = "test-master", vcpus = 4, memory_gb = 8, disk_gb = 100 }
      w1    = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 300 }
    }
  }
  expect_failures = [var.nodes]
}

run "reject_worker_only_cluster" {
  command = plan
  variables {
    nodes = {
      w1 = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 300 }
    }
  }
  expect_failures = [var.nodes]
}

run "reject_unknown_egress" {
  command = plan
  variables { worker_egress = "m1_nat" }
  expect_failures = [var.worker_egress]
}

run "isolate_control_plane_when_disabled" {
  command = plan
  variables { schedule_on_control_plane = false }
  assert {
    condition     = strcontains(output.kubespray_inventory, "[kube_node]\nw1\n") && !strcontains(output.kubespray_inventory, "[kube_node]\nm1")
    error_message = "false 면 control plane 을 kube_node 에서 빼 NoSchedule taint 가 유지되어야 한다."
  }
  assert {
    condition     = strcontains(output.kubespray_inventory, "[kube_control_plane]\nm1\n")
    error_message = "kube_node 에서 빠져도 control plane 역할은 유지되어야 한다."
  }
}

run "reject_type_capacity_mismatch" {
  command = plan
  variables {
    nodes = {
      m1 = { instance_type = "test-master", vcpus = 2, memory_gb = 8, disk_gb = 100 }
      w1 = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 300 }
    }
  }
  expect_failures = [terraform_data.flavors]
}

run "reject_overlapping_subnets" {
  command = plan
  variables { private_subnet_cidr = "10.20.0.0/24" }
  expect_failures = [var.private_subnet_cidr]
}

run "reject_duplicate_node_ips" {
  command = plan
  variables {
    worker_egress = "public_ip"
    nodes = {
      m1 = { instance_type = "test-master", vcpus = 4, memory_gb = 8, disk_gb = 100, private_ip = "10.20.0.10" }
      w1 = { instance_type = "test-worker", vcpus = 16, memory_gb = 32, disk_gb = 300, private_ip = "10.20.0.10" }
    }
  }
  expect_failures = [terraform_data.addresses]
}
