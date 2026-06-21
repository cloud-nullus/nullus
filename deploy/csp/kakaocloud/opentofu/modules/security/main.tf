# Security Module - 보안 그룹
# Nullus Air-Gap 배포에 필요한 포트를 허용하는 보안 그룹

resource "kakaocloud_security_group" "security_group" {
  name        = var.security_group_name
  description = var.description

  rules = [
    # SSH 접속 (외부)
    {
      direction        = "ingress"
      protocol         = "TCP"
      port_range_min   = 22
      port_range_max   = 22
      remote_ip_prefix = "0.0.0.0/0"
      description      = "SSH 접속 허용"
    },
    # HTTP
    {
      direction        = "ingress"
      protocol         = "TCP"
      port_range_min   = 80
      port_range_max   = 80
      remote_ip_prefix = var.web_allowed_cidr
      description      = "HTTP 트래픽 허용 (web_allowed_cidr 로 제한)"
    },
    # HTTPS
    {
      direction        = "ingress"
      protocol         = "TCP"
      port_range_min   = 443
      port_range_max   = 443
      remote_ip_prefix = "0.0.0.0/0"
      description      = "HTTPS 트래픽 허용"
    },
    # NOTE: registry(5001) 와 kind API Server(16443) 는 airgap VM 내부에서
    #   127.0.0.1 로만 바인딩된다 (kind/kind-airgap.yaml: apiServerAddress=127.0.0.1,
    #   registry:2 호스트 매핑=127.0.0.1:5001). 외부 ingress 불필요 — 접근은 SSH
    #   터널(ssh -L)로 한다. 공개 노출을 막기 위해 의도적으로 룰을 두지 않는다.
    #
    # 인트라-VPC 전체 허용 (builder ↔ airgap VM 간 내부 통신)
    {
      direction        = "ingress"
      protocol         = "ALL"
      remote_ip_prefix = var.vpc_cidr
      description      = "VPC 내부 전체 트래픽 허용"
    },
    # ICMP (ping)
    {
      direction        = "ingress"
      protocol         = "ICMP"
      remote_ip_prefix = "0.0.0.0/0"
      description      = "ICMP (Ping) 허용"
    },
    # 모든 아웃바운드 (builder 가 인터넷에서 이미지·바이너리 다운로드)
    {
      direction        = "egress"
      protocol         = "ALL"
      remote_ip_prefix = "0.0.0.0/0"
      description      = "모든 아웃바운드 트래픽 허용"
    }
  ]
}
