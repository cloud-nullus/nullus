resource "aws_instance" "node" {
  for_each               = var.nodes
  ami                    = var.image_id
  instance_type          = each.value.instance_type
  key_name               = var.key_name
  subnet_id              = each.value.subnet_id
  private_ip             = each.value.private_ip
  vpc_security_group_ids = each.value.security_group_ids
  user_data              = var.user_data
  # D8: zCompute 의 RunInstances 는 AssociatePublicIpAddress 를 400
  # `Network details contain unsupported params` 로 거부한다(2026-09-13 apply 실측).
  # false 로도 보낼 수 없어 속성 자체를 두지 않는다. subnet 의
  # map_public_ip_on_launch = false 가 자동 공인 IP 를 막고, 공인 IP 는
  # aws_eip.node 로만 명시적으로 붙인다.
  root_block_device {
    volume_size = each.value.disk_gb
    # D4→D9: false 로 두려 했으나 zCompute 는 생성 시 이 값을 무시하고 true 로
    # 만들며, 이후 ModifyInstanceAttribute 도 끝나지 않는다(2026-09-13 apply 8분+
    # 멈춤 실측). 실제 동작에 맞춰 true 로 둔다 — VM 반납 = root 볼륨 삭제.
    # 데이터 보존은 볼륨이 아니라 백업(G5)으로 해결한다.
    delete_on_termination = true
  }
  lifecycle {
    # zCompute 는 root 볼륨 태그를 조회에 되돌리지 않아 매 plan 마다 드리프트가 난다.
    ignore_changes = [root_block_device[0].tags]
  }
  tags = { Name = "${var.name_prefix}-${each.key}", Cluster = "platform", Role = each.value.role }
}
