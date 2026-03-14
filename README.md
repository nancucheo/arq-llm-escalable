# Arquitectura Full-Stack Asíncrona LLM en Azure

Sistema de chat asíncrono para procesamiento de LLMs con alta disponibilidad y escalabilidad masiva.

## Arquitectura

```
Browser (WebSocket)
        │
        ▼
   Nginx :80 (Reverse Proxy)
        │                    │
        ▼                    ▼
   Frontend             Gateway Go
  (Vanilla JS)      (Gin + Gorilla WS)
                          │    ▲
                    RPUSH │    │ SUBSCRIBE result:<client_id>
                          ▼    │
                         Redis :6379
                          │
                     BLPOP│
                          ▼
                    Worker Python
                  (asyncio + redis-py)
                  simula / llama LLM API
                          │
                    PUBLISH result:<client_id>
                          │
                    ┌─────┴──────┐
                PostgreSQL :5432  Zitadel :8085
               (users/convs/msgs)  (OIDC / JWT)
```

### Flujo de datos

1. El usuario escribe un prompt en el Frontend y lo envía por WebSocket.
2. El **Gateway Go** valida el JWT (Zitadel), genera un `job_id`, empuja el payload a `llm_queue` en Redis y se suscribe a `result:<client_id>`.
3. El **Worker Python** hace `BLPOP` sobre `llm_queue`, procesa el prompt y publica el resultado en `result:<client_id>`.
4. El Gateway recibe el resultado y lo reenvía al cliente por WebSocket.
5. El mensaje se persiste en PostgreSQL (usuarios, conversaciones, mensajes).

## Estructura del proyecto

```
.
├── docker-compose.yml
├── frontend/               # Interfaz de usuario (Vanilla JS + Nginx)
├── gateway-go/             # Orquestador WebSocket (Go + Gin + Gorilla WS)
│   └── internal/
│       ├── auth/           # Middleware JWT RS256 (Zitadel)
│       └── repository/     # Patrón Repository sobre PostgreSQL
├── worker-python/          # Procesador de inferencia (Python async + Redis)
├── cron-jobs/              # Tareas de mantenimiento en Go
│   └── cmd/
│       ├── cache-cleaner/  # Limpia canales Redis obsoletos (cada 30 min)
│       └── usage-reporter/ # Reporta métricas Redis como JSON (cada 5 min)
├── nginx/                  # Reverse proxy (HTTP + WebSocket upgrade)
├── db/                     # Scripts SQL de inicialización
├── zitadel-config/         # Configuración OIDC (Realm, admin, app)
├── k8s-manifests/          # Manifiestos Kubernetes
├── terraform/              # Infraestructura como Código (Azure)
└── scripts/                # Herramientas (load_test.py)
```

## Desarrollo local (Docker Compose)

### Prerrequisitos

- Docker 24+
- Docker Compose v2

### Levantar el stack

```bash
docker compose up --build
```

### URLs locales

| Servicio | URL |
|---------|-----|
| Aplicación | http://localhost |
| Gateway (directo) | http://localhost:8080 |
| Zitadel console | http://localhost:8085/ui/console |
| Redis | localhost:6379 |

**Credenciales Zitadel:** `admin@llm.local` / `Admin1234!`

### Escalar workers

```bash
docker compose up --scale worker=4
```

### Inspeccionar estado

```bash
# Logs de un servicio
docker logs arq-llm-escalable-gateway-1

# Cola de Redis
docker exec arq-llm-escalable-redis-1 redis-cli LLEN llm_queue

# Claves activas en Redis
docker exec arq-llm-escalable-redis-1 redis-cli KEYS '*'

# Limpiar todo (incluye volúmenes)
docker compose down -v
```

### Prueba de carga

```bash
cd scripts
pip install websockets
python load_test.py --url ws://localhost/ws --total 100 --concurrency 10
```

## Integrar un LLM real

Editar `worker-python/main.py`, función `simulate_llm()`:

```python
async def simulate_llm(prompt: str) -> str:
    # Reemplazar con llamada real:
    # import anthropic / import openai
    # response = await client.messages.create(...)
    # return response.content[0].text
    pass
```

## Despliegue en Azure

### 1. Prerrequisitos

```bash
brew install terraform azure-cli kubectl helm
az login
az account set --subscription "<SUBSCRIPTION_ID>"
```

### 2. Provisionar infraestructura con Terraform

```bash
cd terraform

cat > terraform.tfvars <<EOF
subscription_id   = "<SUBSCRIPTION_ID>"
location          = "eastus"
project           = "llm-scalable"
environment       = "prod"
pg_admin_password = "<PASSWORD_SEGURO>"
EOF

terraform init
terraform plan -out=tfplan
terraform apply tfplan
```

Recursos creados: Resource Group, VNet (3 subnets), NSGs, ACR, AKS, Azure Redis Cache, Azure PostgreSQL Flexible Server v16, Log Analytics.

### 3. Build y push de imágenes a ACR

```bash
ACR_NAME=$(terraform output -raw acr_name)
ACR_SERVER=$(terraform output -raw acr_login_server)

az acr login --name $ACR_NAME

docker build -t $ACR_SERVER/gateway:latest   ./gateway-go   && docker push $_
docker build -t $ACR_SERVER/worker:latest    ./worker-python && docker push $_
docker build -t $ACR_SERVER/frontend:latest  ./frontend      && docker push $_
docker build -t $ACR_SERVER/cronjobs:latest  ./cron-jobs     && docker push $_
```

### 4. Configurar kubectl

```bash
$(terraform output -raw aks_get_credentials_command)
kubectl get nodes
```

### 5. Crear Secrets de Kubernetes

```bash
# PostgreSQL
kubectl create secret generic postgres-secret \
  --from-literal=DATABASE_URL="postgresql://pgadmin:<PASSWORD>@$(terraform output -raw postgres_fqdn)/llmdb"

# Zitadel (si se despliega separado)
kubectl create secret generic zitadel-secret \
  --from-literal=ZITADEL_ISSUER="https://<tu-dominio-zitadel>"
```

### 6. Actualizar manifiestos K8s con ACR

```bash
ACR_SERVER=$(cd terraform && terraform output -raw acr_login_server)
find k8s-manifests -name "*-deployment.yaml" \
  -exec sed -i '' "s|<ACR_NAME>.azurecr.io|$ACR_SERVER|g" {} \;
```

### 7. Instalar Nginx Ingress Controller

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
helm install ingress-nginx ingress-nginx/ingress-nginx \
  --namespace ingress-nginx --create-namespace
```

### 8. Aplicar manifiestos

```bash
kubectl apply -f k8s-manifests/redis-deployment.yaml
kubectl apply -f k8s-manifests/services.yaml
kubectl apply -f k8s-manifests/gateway-deployment.yaml
kubectl apply -f k8s-manifests/worker-deployment.yaml
kubectl apply -f k8s-manifests/frontend-deployment.yaml
kubectl apply -f k8s-manifests/ingress.yaml
kubectl apply -f k8s-manifests/hpa.yaml
kubectl apply -f k8s-manifests/cronjob-cache-cleaner.yaml
kubectl apply -f k8s-manifests/cronjob-usage-reporter.yaml

kubectl get pods
kubectl get ingress
```

### 9. Actualizar el Ingress con tu dominio

Editar `k8s-manifests/ingress.yaml` y reemplazar `llm.example.com`. Apuntar DNS al IP del Ingress:

```bash
kubectl get svc -n ingress-nginx ingress-nginx-controller \
  -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
```

## Componentes

| Componente | Tecnología | Puerto local |
|-----------|-----------|-------------|
| Frontend | Vanilla JS + Nginx | 80 (vía Nginx) |
| Gateway | Go 1.22 + Gin + Gorilla WS | 8080 |
| Worker | Python 3.12 + asyncio + redis-py | — |
| Broker | Redis 7 | 6379 |
| Base de datos | PostgreSQL 16 | 5432 (interno) |
| Identidad | Zitadel v2.70.1 | 8085 |
| Proxy | Nginx 1.25 | 80 |
| Cache cleaner | Go (cron) | — |
| Usage reporter | Go (cron) | — |

## Contrato Redis

| Elemento | Valor |
|---------|-------|
| Cola | `llm_queue` |
| Job payload | `{"job_id":"<uuid>","client_id":"<uuid>","prompt":"<text>"}` |
| Canal resultado | `result:<client_id>` |
| Result payload | `{"job_id":"<uuid>","result":"<text>","elapsed":<segundos>}` |
| Timeout gateway | 60 segundos |

## Escalabilidad

- **Gateway HPA**: 2–10 réplicas (CPU > 60%)
- **Worker HPA**: 2–20 réplicas (CPU > 70% o RAM > 80%)
- **AKS Autoscaler**: 2–5 nodos según demanda
- **Workers locales**: `docker compose up --scale worker=N`

## Seguridad

- JWT RS256 validado contra JWKS de Zitadel (`/oauth/v2/keys`)
- `user_id` derivado determinísticamente con UUID v5 (no expuesto el sub raw)
- NSGs en Azure: PostgreSQL y Redis solo accesibles desde subnet AKS
- PostgreSQL en endpoint privado (sin acceso a Internet)
- ACR sin admin habilitado (acceso vía identidad AKS)

## Capa de datos (Spanner-Ready)

Tablas con PKs compuestas y UUID v4 para compatibilidad con bases distribuidas:

```sql
users         (user_id UUID PK)
conversations (user_id UUID, conv_id UUID, PK compuesta)
messages      (user_id UUID, conv_id UUID, msg_id UUID, PK compuesta)
```

Todas las consultas incluyen el prefijo completo de la PK para evitar fan-out.

## Limpiar recursos Azure

```bash
cd terraform
terraform destroy
```
