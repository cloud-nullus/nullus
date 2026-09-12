# D7: zCompute 의 CreateSecurityGroup 은 요청에 포함된 tags 를 400
# `Invalid format for tags` 로 거부한다(2026-09-13 apply 실측). 별도 CreateTags 는
# 받지만 조회에 반영되지 않아 태그 자체가 이 플랫폼에서 동작하지 않는다. SG 는
# name 으로 식별한다. bastion/web 은 원래 태그가 없어 동일하다.
resource "aws_security_group" "node" {
  name        = "${var.name_prefix}-node"
  description = "Nullus cluster nodes"
  vpc_id      = var.vpc_id
}
resource "aws_security_group" "bastion" {
  name        = "${var.name_prefix}-bastion"
  description = "Operator SSH entry"
  vpc_id      = var.vpc_id
}
resource "aws_security_group" "web" {
  name        = "${var.name_prefix}-web"
  description = "Optional web entry"
  vpc_id      = var.vpc_id
}
resource "aws_security_group_rule" "ssh" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.admin_cidrs
  security_group_id = aws_security_group.bastion.id
}
resource "aws_security_group_rule" "ssh_worker" {
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.bastion.id
  security_group_id        = aws_security_group.node.id
}
locals {
  # Calico VXLAN 전용 규칙. 전 VPC/전체 NodePort 대역을 허용하지 않는다.
  node_rules = {
    api     = { protocol = "tcp", from = 6443, to = 6443 }
    kubelet = { protocol = "tcp", from = 10250, to = 10250 }
    vxlan   = { protocol = "udp", from = 4789, to = 4789 }
    http    = { protocol = "tcp", from = 30080, to = 30080 }
    https   = { protocol = "tcp", from = 30443, to = 30443 }
    icmp    = { protocol = "icmp", from = -1, to = -1 }
  }
  web_rules = { for pair in setproduct(toset(var.web_cidrs), toset([for p in var.web_ports : tostring(p)])) : "${pair[0]}:${pair[1]}" => { cidr = pair[0], port = tonumber(pair[1]) } }
}
resource "aws_security_group_rule" "node" {
  for_each          = local.node_rules
  type              = "ingress"
  from_port         = each.value.from
  to_port           = each.value.to
  protocol          = each.value.protocol
  self              = true
  security_group_id = aws_security_group.node.id
}
resource "aws_security_group_rule" "web" {
  for_each          = local.web_rules
  type              = "ingress"
  from_port         = each.value.port
  to_port           = each.value.port
  protocol          = "tcp"
  cidr_blocks       = [each.value.cidr]
  security_group_id = aws_security_group.web.id
}
# 온라인 PoC의 패키지/이미지/DNS/NTP egress. 폐쇄망은 별도 구성이다.
resource "aws_security_group_rule" "egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.node.id
}
