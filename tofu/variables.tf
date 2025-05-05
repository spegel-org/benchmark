variable "kubernetes_version" {
  type    = string
}

variable "default_node_pool_vm_size" {
  type    = string
  default = "Standard_D2ds_v5"
}

variable "vm_size" {
  type    = string
  default = "Standard_D8ds_v5"
}

variable "node_count" {
  type    = number
  default = 10
}
