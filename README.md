# Arquitectura Full-Stack Asíncrona LLM en Azure

Sistema de chat asíncrono para procesamiento de LLMs con alta disponibilidad y escalabilidad masiva.

## Arquitectura

```
Browser (WS)
    │
    ▼
Nginx (Reverse Proxy)
    │                 │
    ▼                 ▼
Frontend          Gateway Go (Gin + Gorilla WS)
(Static HTML)         │               ▲
                      │ RPUSH         │ PUBLISH
                      ▼               │
                    Redis ────────────┘
                      │ BLPOP
                      ▼
                  Worker Python (FastAPI + asyncio)
                  (simula / llama LLM API)
```

### Flujo de datos

1. El usuario escribe un prompt en el Frontend y lo envía por WebSocket.
2. El **Gateway Go** recibe el mensaje, genera un `job_id`, empuja el payload a la cola `llm_queue` de Redis y queda escuchando en `result:<client_id>`.
3. El **Worker Python** hace `BLPOP` sobre `llm_queue`, procesa el prompt (simulado o real) y hace `PUBLISH` al canal `result:<client_id>`.
4. El Gateway recibe el resultado y lo reenvía al cliente por WebSocket.

## Estructura del proyecto

```
.
├── docker-compose.yml
├── frontend/          # Interfaz de usuario (Vanilla JS + Nginx)
├── gateway-go/        # Orquestador de conexiones (Go + Gin + Gorilla WS)
├── worker-python/     # Procesador de inferencia (Python async + Redis)
├── nginx/             # Configuración de Nginx como reverse proxy
├── k8s-manifests/     # Manifiestos de Kubernetes
└── terraform/         # Infraestructura como Código (Azure)
```

## Desarrollo local (Docker Compose)

### Prerrequisitos

- Docker 24+
- Docker Compose v2

### Levantar el stack

```bash
docker compose up --build
```

Abrir el navegador en `http://localhost` (Nginx en el puerto 80).

Para escalar workers en local:

```bash
docker compose up --scale worker=4
```

## Despliegue en Azure

### 1. Prerrequisitos

```bash
# Instalar herramientas
brew install terraform azure-cli kubectl helm

# Autenticarse en Azure
az login
az account set --subscription "<SUBSCRIPTION_ID>"
```

### 2. Provisionar infraestructura con Terraform

```bash
cd terraform

# Crear archivo de variables
cat > terraform.tfvars <<EOF
subscription_id = "<SUBSCRIPTION_ID>"
location        = "eastus"
project         = "llm-scalable"
environment     = "prod"
EOF

terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

Los outputs incluyen los valores necesarios para los pasos siguientes.

### 3. Build y push de imágenes a ACR

```bash
ACR_NAME=$(terraform output -raw acr_name)
ACR_SERVER=$(terraform output -raw acr_login_server)

az acr login --name $ACR_NAME

# Build y push
docker build -t $ACR_SERVER/gateway:latest ./gateway-go
docker build -t $ACR_SERVER/worker:latest ./worker-python
docker build -t $ACR_SERVER/frontend:latest ./frontend

docker push $ACR_SERVER/gateway:latest
docker push $ACR_SERVER/worker:latest
docker push $ACR_SERVER/frontend:latest
```

### 4. Configurar kubectl

```bash
$(terraform output -raw aks_get_credentials_command)
kubectl get nodes
```

### 5. Actualizar manifiestos K8s

Reemplazar `<ACR_NAME>` en los archivos de deployment:

```bash
ACR_SERVER=$(cd terraform && terraform output -raw acr_login_server)
find k8s-manifests -name "*-deployment.yaml" \
  -exec sed -i '' "s|<ACR_NAME>.azurecr.io|$ACR_SERVER|g" {} \;
```

### 6. Instalar Nginx Ingress Controller

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace
```

### 7. Aplicar manifiestos

```bash
kubectl apply -f k8s-manifests/redis-deployment.yaml
kubectl apply -f k8s-manifests/services.yaml
kubectl apply -f k8s-manifests/gateway-deployment.yaml
kubectl apply -f k8s-manifests/worker-deployment.yaml
kubectl apply -f k8s-manifests/frontend-deployment.yaml
kubectl apply -f k8s-manifests/ingress.yaml
kubectl apply -f k8s-manifests/hpa.yaml

# Verificar
kubectl get pods
kubectl get ingress
```

### 8. Actualizar el Ingress con tu dominio

Editar `k8s-manifests/ingress.yaml` y reemplazar `llm.example.com` con tu dominio. Apuntar el DNS al IP del Ingress:

```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

## Componentes

| Componente | Tecnología | Puerto |
|-----------|-----------|--------|
| Frontend | Vanilla JS + Nginx | 80 |
| Gateway | Go 1.22 + Gin + Gorilla WS | 8080 |
| Worker | Python 3.12 + asyncio + redis-py | — |
| Broker | Redis 7 | 6379 |
| Proxy | Nginx | 80/443 |

## Escalabilidad

- **Gateway HPA**: 2–10 réplicas (CPU > 60%)
- **Worker HPA**: 2–20 réplicas (CPU > 70% o RAM > 80%)
- **AKS Autoscaler**: 2–5 nodos según demanda

## Limpiar recursos

```bash
cd terraform
terraform destroy
```
