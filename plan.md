# Plan: Arquitectura Full-Stack Asíncrona LLM en Azure

## Progreso

### Fase 1: Scaffolding base
- [x] `.gitignore`
- [x] `docker-compose.yml`

### Fase 2: Frontend (Vanilla JS + Nginx)
- [x] `frontend/index.html`
- [x] `frontend/Dockerfile`

### Fase 3: Gateway Go (Gin + Gorilla WS + Redis)
- [x] `gateway-go/main.go`
- [x] `gateway-go/go.mod`
- [x] `gateway-go/Dockerfile`

### Fase 4: Worker Python (FastAPI + redis-py)
- [x] `worker-python/main.py`
- [x] `worker-python/requirements.txt`
- [x] `worker-python/Dockerfile`

### Fase 5: Nginx config (reverse proxy + WS upgrade)
- [x] `nginx/nginx.conf`

### Fase 6: K8s Manifests
- [x] `k8s-manifests/frontend-deployment.yaml`
- [x] `k8s-manifests/gateway-deployment.yaml`
- [x] `k8s-manifests/worker-deployment.yaml`
- [x] `k8s-manifests/redis-deployment.yaml`
- [x] `k8s-manifests/services.yaml`
- [x] `k8s-manifests/ingress.yaml`
- [x] `k8s-manifests/hpa.yaml`

### Fase 7: Terraform Azure
- [x] `terraform/providers.tf`
- [x] `terraform/main.tf`
- [x] `terraform/variables.tf`
- [x] `terraform/outputs.tf`

### Fase 8: README.md
- [x] `README.md`
