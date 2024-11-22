variable "kubernetes_version" {
  type    = string
  default = "1.30"
}

variable "default_node_pool_vm_size" {
  type    = string
  default = "Standard_D2ds_v5"
}

variable "vm_size" {
  type    = string
  default = "Standard_D2ds_v5"
}

variable "node_count" {
  type    = number
  default = 10
}
