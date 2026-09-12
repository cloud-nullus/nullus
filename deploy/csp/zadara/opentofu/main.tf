locals {
  roles         = { for name, n in var.nodes : name => coalesce(n.role, startswith(name, "m") ? "control_plane" : "worker") }
  masters       = sort([for name, role in local.roles : name if role == "control_plane"])
  workers       = sort([for name, role in local.roles : name if role == "worker"])
  master        = try(local.masters[0], keys(var.nodes)[0])
  public_nodes  = sort([for name, role in local.roles : name if role == "control_plane" || var.worker_egress == "public_ip"])
  private_nodes = sort([for name, role in local.roles : name if role == "worker" && var.worker_egress == "nat"])
  placement = { for name, n in var.nodes : name => {
    role       = local.roles[name]
    public     = contains(local.public_nodes, name)
    subnet     = contains(local.public_nodes, name) ? "public" : "private"
    private_ip = coalesce(n.private_ip, cidrhost(contains(local.public_nodes, name) ? var.public_subnet_cidr : var.private_subnet_cidr, 10 + index(contains(local.public_nodes, name) ? local.public_nodes : local.private_nodes, name)))
  } }
  capacity = {
    vcpus     = sum([for n in values(var.nodes) : n.vcpus])
    memory_gb = sum([for n in values(var.nodes) : n.memory_gb])
    disk_gb   = sum([for n in values(var.nodes) : n.disk_gb])
  }
}
module "network" {
  source              = "./modules/network"
  name_prefix         = var.name_prefix
  vpc_cidr            = var.vpc_cidr
  public_subnet_cidr  = var.public_subnet_cidr
  private_subnet_cidr = var.private_subnet_cidr
  availability_zone   = var.availability_zone
  enable_nat          = var.worker_egress == "nat"
}
module "security" {
  source      = "./modules/security"
  name_prefix = var.name_prefix
  vpc_id      = module.network.vpc_id
  admin_cidrs = sort(tolist(var.admin_cidrs))
  web_cidrs   = sort(tolist(var.web_cidrs))
  web_ports   = var.web_ports
}
module "compute" {
  source      = "./modules/compute"
  name_prefix = var.name_prefix
  nodes = { for name, n in var.nodes : name => {
    instance_type      = n.instance_type
    disk_gb            = n.disk_gb
    role               = local.placement[name].role
    subnet_id          = local.placement[name].public ? module.network.public_subnet_id : module.network.private_subnet_id
    private_ip         = local.placement[name].private_ip
    public             = local.placement[name].public
    security_group_ids = concat([module.security.node_security_group_id], local.roles[name] == "control_plane" ? [module.security.bastion_security_group_id] : [], local.placement[name].public ? [module.security.web_security_group_id] : [])
  } }
  image_id   = var.image_id
  key_name   = var.key_name
  user_data  = file("${path.module}/cloud-init.yaml")
  depends_on = [module.network, terraform_data.addresses, terraform_data.flavors]
}
# D5: 잘못된 사설 IP는 provider 호출 전에 차단한다. 사용자 지정 주소는 편의를
# 주지만 subnet 예약 주소/중복 위험이 있어 실제 배치 결과에서 검증한다.
resource "terraform_data" "addresses" {
  input = local.placement
  lifecycle {
    precondition {
      condition     = length(distinct([for n in values(local.placement) : n.private_ip])) == length(local.placement) && alltrue([for n in values(local.placement) : try(cidrcontains(n.public ? var.public_subnet_cidr : var.private_subnet_cidr, n.private_ip) && tonumber(split(".", n.private_ip)[3]) >= 4 && tonumber(split(".", n.private_ip)[3]) < 255, false)])
      error_message = "노드 IP는 해당 subnet의 예약되지 않은 주소여야 하며 중복될 수 없습니다."
    }
  }
}
locals {
  inventory = templatefile("${path.module}/templates/inventory.ini.tftpl", {
    nodes            = local.placement
    masters          = local.masters
    kube_nodes       = var.schedule_on_control_plane ? sort(keys(var.nodes)) : local.workers
    master           = local.master
    master_public_ip = local.public_ips[local.master]
    ssh_user         = var.ssh_user
    ssh_key          = pathexpand(var.ssh_private_key_file)
  })
}
# D2: VM과 inventory까지만 IaC가 소유한다. Kubespray/Helm 실패가 VM 교체를
# 유발하지 않게 remote-exec를 사용하지 않는다. 다음 단계는 README에서 인계한다.
resource "local_file" "inventory" {
  content              = local.inventory
  filename             = "${path.module}/generated/inventory.ini"
  file_permission      = "0600"
  directory_permission = "0700"
}
resource "local_file" "kubespray_overrides" {
  source               = "${path.module}/templates/zz-nullus-overrides.yml"
  filename             = "${path.module}/generated/zz-nullus-overrides.yml"
  file_permission      = "0600"
  directory_permission = "0700"
}

# D10: 공인 IP 는 VM 이나 스택보다 오래 산다. DNS 가 이 주소를 가리키므로 VM 교체·
# nodes 변경·destroy 로 주소가 바뀌면 안 된다. EIP 는 루트에서 prevent_destroy 로
# 보호하고, VM 과의 결합은 association 으로 분리해 VM 교체 시 association 만 다시 만든다.
# 비용: 스택을 완전히 걷어낼 때는 이 블록의 prevent_destroy 를 먼저 풀어야 한다.
resource "aws_eip" "public" {
  for_each = { for name, p in local.placement : name => p if p.public }
  vpc      = true
  lifecycle {
    prevent_destroy = true
    # zCompute 는 EIP 태그를 조회에 되돌리지 않는다(D7 과 같은 이유).
    ignore_changes = [tags]
  }
}
resource "aws_eip_association" "public" {
  for_each      = aws_eip.public
  allocation_id = each.value.id
  instance_id   = module.compute.instance_ids[each.key]
}
locals {
  public_ips = { for name, e in aws_eip.public : name => e.public_ip }
}
