Especificación Técnica: Arquitectura LLM Escalable con IaC en Azure

1. Contexto y Objetivo
Implementar un sistema full-stack asíncrono para procesamiento de LLMs, diseñado para alta disponibilidad y escalabilidad masiva. El despliegue debe estar automatizado mediante Terraform en Azure.

2. Componentes y Tecnologías
A. Frontend (Vanilla JS)
Interfaz: Chat interactivo con estados de carga.

Comunicación: WebSockets para recibir actualizaciones en tiempo real del estado del prompt.

Implementación: HTML/CSS/JS puro servido por Nginx. Sin framework ni build step.
Archivo: frontend/index.html, frontend/Dockerfile

B. Gateway de Alto Rendimiento (Go)
Stack: Framework Gin + Gorilla WebSocket.

Rol: Orquestador de conexiones. Recibe el prompt, genera un job_id, lo envía a Redis y mantiene el socket abierto para devolver la respuesta cuando esté lista.

Autenticación: Middleware JWT (RS256) que valida tokens emitidos por Zitadel. Deriva user_id determinístico usando UUID v5 (SHA1) sobre el claim sub del JWT. Fallback a modo anónimo si ZITADEL_ISSUER no está configurado.

Persistencia: Capa Repository (interfaz Go) sobre PostgreSQL (pgx/v5). Guarda usuarios, conversaciones y mensajes con PKs compuestas.

Archivos:
- gateway-go/main.go
- gateway-go/internal/auth/middleware.go
- gateway-go/internal/repository/repository.go  (interfaz)
- gateway-go/internal/repository/postgres.go    (implementación pgx)
- gateway-go/internal/repository/models.go      (structs: User, Conversation, Message)
- gateway-go/Dockerfile                          (golang:1.23-alpine → alpine:3.19)

Variables de entorno: REDIS_ADDR, DATABASE_URL, ZITADEL_ISSUER, PORT (default 8080)

C. Backend de Inferencia (Python)
Stack: asyncio + redis.asyncio.

Rol: Consumidor de la cola. Simula o conecta con una API de LLM y publica el resultado en el canal de Pub/Sub de Redis.

Punto de integración: función simulate_llm() en worker-python/main.py — reemplazar con llamada real a OpenAI/Anthropic/etc.

Archivos:
- worker-python/main.py
- worker-python/requirements.txt (redis[hiredis]>=5.0.0)
- worker-python/Dockerfile

D. Infraestructura y Mensajería
Redis: Actúa como Broker de mensajería (Colas BLPOP/RPUSH) y Bus de datos (Pub/Sub).

Nginx: Configurado como Reverse Proxy para manejar tráfico HTTP y el upgrade de WebSockets (proxy_read_timeout 86400s).
Archivo: nginx/nginx.conf

E. Tareas Programadas: Go CronJobs
Rol: Procesos de mantenimiento de corta duración.

cache-cleaner: Escanea claves result:* en Redis y elimina las que llevan más de STALE_TTL_HOURS horas sin actividad (OBJECT IDLETIME). Corre cada 30 min.
usage-reporter: Recoge métricas de Redis (queue depth, canales activos, memoria) y las imprime como JSON. Corre cada 5 min.

Implementación: Binarios Go compilados en imagen Alpine. En local corren en bucle con sleep; en K8s se usan CronJob resources.

Archivos:
- cron-jobs/cmd/cache-cleaner/main.go
- cron-jobs/cmd/usage-reporter/main.go
- cron-jobs/internal/redisclient/client.go
- cron-jobs/Dockerfile

F. Autenticación: Zitadel (Dockerizado)
Versión: v2.70.1 (pinada — versión con login UI embebido).

Modelo: Autenticación basada en Event Sourcing.

Persistencia: Comparte la instancia de PostgreSQL (schema aislado).

Integración: El user_id derivado del claim sub de Zitadel es el identificador único en la tabla users.

Configuración inicial (steps.yaml):
- Org: LLM
- Admin: admin@llm.local / Admin1234!
- App OIDC nativa: llm-app (Authorization Code + PKCE)
- Redirect URIs: http://localhost, http://localhost:5173

Archivos:
- zitadel-config/steps.yaml
- zitadel-config/healthcheck.yaml

Notas de operación:
- La master key debe tener exactamente 32 bytes.
- El healthcheck usa --config /zitadel-config/healthcheck.yaml para apuntar al puerto interno 8080.
- La dependencia del gateway se configura como service_started (no service_healthy) porque zitadel ready checa la URL externa que no es accesible desde dentro del contenedor.

3. Infraestructura como Código (Terraform - Azure)
Archivos en terraform/:

providers.tf   — azurerm ~> 3.110, azuread ~> 2.53, Terraform >= 1.7
variables.tf   — subscription_id, location, project, environment, tamaños de nodos AKS, SKU de Redis y PostgreSQL
main.tf        — recursos completos (ver abajo)
outputs.tf     — acr_login_server, aks_get_credentials_command, redis_hostname, postgres_fqdn

Recursos Azure provisionados:
- Resource Group
- VNet (10.0.0.0/16) con 3 subnets: AKS (10.0.1.0/24), Redis (10.0.2.0/24), PostgreSQL (10.0.3.0/24, delegada)
- Network Security Groups: AKS permite 80/443 desde Internet; PostgreSQL y Redis solo permiten tráfico desde subnet AKS
- Private DNS Zone para resolución interna de PostgreSQL
- Azure Container Registry (Standard SKU, admin deshabilitado)
- AKS Cluster: Azure CNI, autoescalado 2-5 nodos Standard_D2s_v3, identidad system-assigned, Log Analytics
- Azure Cache for Redis: Family C, TLS 1.2, maxmemory-policy allkeys-lru
- Azure PostgreSQL Flexible Server v16: endpoint privado, 32GB storage
- Log Analytics Workspace: PerGB2018, retención 30 días

4. Flujo de Datos
El usuario envía un prompt desde el Frontend.

Go recibe la petición, la valida (JWT si ZITADEL_ISSUER está configurado) y la empuja a la lista llm_queue en Redis (RPUSH).

El worker en Python extrae la tarea (BLPOP), procesa el texto y publica el resultado en el canal result:<client_id> (PUBLISH).

Go recibe la notificación de Redis y envía el mensaje final al Frontend a través del WebSocket activo.

Timeout: Gateway espera máximo 60 segundos por respuesta antes de cerrar el WebSocket.

Contrato Redis:
- Cola: llm_queue
- Job payload (JSON): {"job_id": "<uuid>", "client_id": "<uuid>", "prompt": "<text>"}
- Canal resultado: result:<client_id>
- Result payload (JSON): {"job_id": "<uuid>", "result": "<text>", "elapsed": <segundos>}

5. Orquestación (Kubernetes Manifests)
Archivos en k8s-manifests/:

gateway-deployment.yaml    — 2 réplicas, imagen ACR, health probes en /health
worker-deployment.yaml     — 2 réplicas base, imagen ACR
redis-deployment.yaml      — 1 réplica stateless
frontend-deployment.yaml   — 2 réplicas, imagen ACR
services.yaml              — Services para redis (headless), gateway (:8080), frontend (:80)
hpa.yaml                   — HPAs para gateway (2-10, CPU>60%) y worker (2-20, CPU>70% o RAM>80%)
ingress.yaml               — Nginx Ingress: /ws y /health → gateway, / → frontend; timeouts WebSocket
cronjob-cache-cleaner.yaml — CronJob */30 * * * *, timeout 60s, backoff 2
cronjob-usage-reporter.yaml — CronJob */5 * * * *, timeout 60s
postgres-secret.yaml       — Secret placeholder para DATABASE_URL
zitadel-secret.yaml        — Secret placeholder para ZITADEL_ISSUER

Placeholders: Todos los deployment YAMLs contienen <ACR_NAME>.azurecr.io como registro — reemplazar con acr_login_server del output de Terraform antes de aplicar.

6. Entregables Implementados

Directorio terraform/: Scripts para aprovisionar los recursos en Azure. ✓

Directorio gateway-go/: Código del middleware con auth JWT, repo pattern y WebSocket. ✓

Directorio worker-python/: Código del procesador de IA (simulado, listo para integrar LLM real). ✓

Directorio frontend/: Interfaz de chat en Vanilla JS. ✓

k8s-manifests/: Archivos YAML de Kubernetes (Deployments, Services, Ingress, HPA, CronJobs). ✓

Directorio cron-jobs/: Tareas de mantenimiento en Go (cache-cleaner, usage-reporter). ✓

Directorio db/: Script SQL de inicialización con PKs compuestas y UUIDs. ✓

Directorio zitadel-config/: Configuración del Realm, Admin y App OIDC. ✓

scripts/load_test.py: Script de prueba de carga WebSocket (asyncio + websockets). ✓

7. Capa de Datos: PostgreSQL "Spanner-Ready"
A. Diseño de Tablas (Jerarquía con PKs compuestas)
Para garantizar la compatibilidad con bases de datos distribuidas, se usan UUID v4 y llaves primarias compuestas:

users: (user_id UUID PK)

conversations: (user_id UUID, conv_id UUID, PK compuesta) → FK a users.

messages: (user_id UUID, conv_id UUID, msg_id UUID, PK compuesta) → FK a conversations.

Archivo: db/init.sql

B. Patrón Repository (Go)
La capa de acceso a datos está abstraída tras la interfaz Repository:
- EnsureUser(ctx, userID)        — INSERT ... ON CONFLICT DO NOTHING
- CreateConversation(ctx, ...)   — INSERT ... ON CONFLICT DO NOTHING
- SaveMessage(ctx, msg)         — INSERT, genera MsgID si nil
- GetMessages(ctx, userID, convID) — SELECT ... ORDER BY created_at

ID Generation: Los UUIDs se generan en Go antes del INSERT.

Consultas: Todas incluyen el prefijo completo de la PK (WHERE user_id=? AND conv_id=?) para evitar fan-out en sistemas distribuidos.

8. Identidad: Zitadel (Dockerizado)
Modelo: Autenticación basada en Event Sourcing.

Persistencia: Comparte la instancia de PostgreSQL (schema aislado automáticamente por Zitadel).

Integración: El claim sub del JWT de Zitadel se transforma en un UUID v5 determinístico (namespace fijo f47ac10b-58cc-4372-a567-0e02b2c3d479) que se usa como user_id en la tabla users de la aplicación, garantizando consistencia en el ecosistema.

9. Desarrollo Local
Stack completo levantado con Docker Compose (docker-compose.yml):

Servicios: postgres, redis, zitadel, gateway, worker (x2), frontend, nginx, cache-cleaner, usage-reporter

Comandos:
  docker compose up --build          # levantar todo
  docker compose up --scale worker=4 # escalar workers
  docker compose down -v             # limpiar volúmenes

URLs locales:
  http://localhost           — Aplicación (Nginx)
  http://localhost:8080      — Gateway directo
  http://localhost:8085/ui/console — Zitadel console (admin@llm.local / Admin1234!)
  localhost:6379             — Redis

Notas de compatibilidad resueltas durante la implementación:
- Gateway Dockerfile usa golang:1.23-alpine (1.22 tenía conflicto de toolchain con dependencia rogpeppe/go-internal v1.14.1)
- jwk.NewCache API en lestrrat-go/jwx/v2: WithRefreshInterval se pasa a Register(), no a NewCache()
- Zitadel v4 (latest) usa Login UI v2 externo — se usa v2.70.1 que tiene login embebido
- Master key de Zitadel debe tener exactamente 32 bytes
- Contraseña del admin debe cumplir política de complejidad (mínimo 8 chars, mayúscula, número, símbolo)
