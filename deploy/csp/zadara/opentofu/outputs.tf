output "capacity" { value = local.capacity }
output "vpc_id" { value = module.network.vpc_id }
output "nodes" {
  value = { for name, n in local.placement : name => {
    id            = module.compute.instance_ids[name]
    private_ip    = n.private_ip
    public_ip     = try(local.public_ips[name], null)
    instance_type = var.nodes[name].instance_type
    role          = n.role
    subnet        = n.subnet
  } }
}
output "kubespray_inventory" { value = local.inventory }
output "inventory_path" { value = local_file.inventory.filename }
output "ssh_master" {
  value = "ssh -i ${pathexpand(var.ssh_private_key_file)} ${var.ssh_user}@${local.public_ips[local.master]}"
}
