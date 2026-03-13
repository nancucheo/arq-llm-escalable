# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Scalable asynchronous LLM processing system using a publish-subscribe pattern with Redis as the central message broker. Users interact via WebSocket; requests are queued in Redis, processed by Python workers (which call the LLM), and results are streamed back via Redis pub/sub.

**Data flow:**
```
Browser (WebSocket) → Nginx → Gateway (Go) --RPUSH--> Redis llm_queue
                                                              ↓ BLPOP
                                                        Worker (Python)
                                                              ↓ PUBLISH result:<client_id>
Browser <-- Gateway <-- Redis pub/sub <--------------------/
```

## Local Development

```bash
# Start all services (builds images automatically)
docker compose up --build

# Scale workers
docker compose up --scale worker=4

# Access app at http://localhost

# View logs
docker logs <container-name>

# Inspect Redis state
docker exec <redis-container> redis-cli KEYS '*'
docker exec <redis-container> redis-cli LLEN llm_queue

# Stop and remove volumes
docker compose down -v
```

## Component Commands

### Gateway (Go)
```bash
cd gateway-go
go build ./...
go run main.go
go test ./...

# Environment variables
REDIS_ADDR=localhost:6379  # default
PORT=8080                  # default
GIN_MODE=release           # for production
```

### Worker (Python)
```bash
cd worker-python
pip install -r requirements.txt
python main.py

# Environment variables
REDIS_HOST=localhost  # default
REDIS_PORT=6379       # default
```

### Frontend
Static HTML/CSS/JS — no build step required. Served by Nginx in both local and production.

## Azure / Kubernetes Deployment

```bash
# 1. Provision infrastructure
cd terraform
terraform init
terraform apply -var="subscription_id=<ID>"

# 2. Get outputs for image push
ACR_SERVER=$(terraform output -raw acr_login_server)
ACR_NAME=$(terraform output -raw acr_name)

# 3. Build and push images
az acr login --name $ACR_NAME
docker build -t $ACR_SERVER/gateway:latest ./gateway-go && docker push $_
docker build -t $ACR_SERVER/worker:latest ./worker-python && docker push $_
docker build -t $ACR_SERVER/frontend:latest ./frontend && docker push $_

# 4. Configure kubectl
$(terraform output -raw aks_get_credentials_command)

# 5. Patch ACR name in manifests then apply
find k8s-manifests -name "*-deployment.yaml" \
  -exec sed -i '' "s|<ACR_NAME>.azurecr.io|$ACR_SERVER|g" {} \;

helm install ingress-nginx ingress-nginx/ingress-nginx --namespace ingress-nginx --create-namespace
kubectl apply -f k8s-manifests/

# Teardown
terraform destroy
```

## Architecture Details

### Redis Communication Contract
- **Queue name:** `llm_queue` — Gateway pushes, Worker pops via `BLPOP`
- **Job payload (JSON):** `{"job_id": "<uuid>", "client_id": "<uuid>", "prompt": "<text>"}`
- **Result channel:** `result:<client_id>` — Worker publishes, Gateway subscribes
- **Result payload (JSON):** `{"job_id": "<uuid>", "result": "<text>", "elapsed": <seconds>}`
- Gateway uses a **60-second timeout** waiting for a result before closing the WebSocket

### LLM Integration Point
The simulated LLM is in `worker-python/main.py` in `simulate_llm()`. Replace this function with actual API calls to integrate a real LLM.

### Kubernetes Autoscaling
- **Gateway HPA:** 2–10 replicas, triggers at CPU > 60%
- **Worker HPA:** 2–20 replicas, triggers at CPU > 70% or Memory > 80%
- **AKS node autoscaler:** 2–5 nodes (configurable via Terraform variables)

### Nginx WebSocket Routing
`/ws` and `/health` → Gateway (port 8080); `/` → Frontend (port 80). WebSocket proxy timeouts are set to 86400s (24h).

### K8s Manifest Placeholders
All deployment YAML files contain `<ACR_NAME>.azurecr.io` as the image registry placeholder — replace with actual ACR login server before applying.
