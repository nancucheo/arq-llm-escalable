variable "subscription_id" {
  type        = string
  description = "Azure Subscription ID"
}

variable "location" {
  type        = string
  default     = "eastus"
  description = "Azure region for all resources"
}

variable "project" {
  type        = string
  default     = "llm-scalable"
  description = "Project name prefix used in resource naming"
}

variable "environment" {
  type        = string
  default     = "prod"
  description = "Environment tag (prod, staging, dev)"
}

variable "aks_node_count" {
  type        = number
  default     = 2
  description = "Initial number of AKS system nodes"
}

variable "aks_node_vm_size" {
  type        = string
  default     = "Standard_D2s_v3"
  description = "VM size for AKS nodes"
}

variable "aks_min_count" {
  type        = number
  default     = 2
  description = "Minimum node count for AKS autoscaler"
}

variable "aks_max_count" {
  type        = number
  default     = 5
  description = "Maximum node count for AKS autoscaler"
}

variable "redis_capacity" {
  type        = number
  default     = 1
  description = "Redis Cache capacity (SKU family C: 0-6)"
}

variable "redis_sku" {
  type        = string
  default     = "Standard"
  description = "Redis Cache SKU (Basic, Standard, Premium)"
}
