output "resource_group_name" {
  value       = azurerm_resource_group.main.name
  description = "Resource group containing all resources"
}

output "acr_login_server" {
  value       = azurerm_container_registry.main.login_server
  description = "ACR login server URL for docker push/pull"
}

output "acr_name" {
  value       = azurerm_container_registry.main.name
  description = "ACR name (replace <ACR_NAME> in k8s manifests)"
}

output "aks_cluster_name" {
  value       = azurerm_kubernetes_cluster.main.name
  description = "AKS cluster name"
}

output "aks_get_credentials_command" {
  value       = "az aks get-credentials --resource-group ${azurerm_resource_group.main.name} --name ${azurerm_kubernetes_cluster.main.name}"
  description = "Run this command to configure kubectl"
}

output "redis_hostname" {
  value       = azurerm_redis_cache.main.hostname
  description = "Redis hostname (for production K8s secrets)"
  sensitive   = false
}

output "redis_ssl_port" {
  value       = azurerm_redis_cache.main.ssl_port
  description = "Redis SSL port"
}

output "redis_primary_access_key" {
  value       = azurerm_redis_cache.main.primary_access_key
  description = "Redis primary access key"
  sensitive   = true
}
