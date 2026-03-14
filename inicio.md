Especificación Técnica: Arquitectura LLM Escalable con IaC en Azure

1. Contexto y Objetivo
Implementar un sistema full-stack asíncrono para procesamiento de LLMs, diseñado para alta disponibilidad y escalabilidad masiva. El despliegue debe estar automatizado mediante Terraform en Azure.

2. Componentes y Tecnologías
A. Frontend (React/Vanilla JS)
Interfaz: Chat interactivo con estados de carga.

Comunicación: WebSockets para recibir actualizaciones en tiempo real del estado del prompt.

B. Gateway de Alto Rendimiento (Go)
Stack: Framework Gin/Echo + Gorilla WebSocket.

Rol: Orquestador de conexiones. Recibe el prompt, genera un job_id, lo envía a Redis y mantiene el socket abierto para devolver la respuesta cuando esté lista.

C. Backend de Inferencia (Python)
Stack: FastAPI + Redis-py.

Rol: Consumidor de la cola. Simula o conecta con una API de LLM y publica el resultado en el canal de Pub/Sub de Redis.

D. Infraestructura y Mensajería
Redis: Actúa como Broker de mensajería (Colas) y como Bus de datos (Pub/Sub).

Nginx: Configurado como Reverse Proxy para manejar tráfico HTTP y el upgrade de WebSockets.

E. Tareas Programadas: Go CronJobs
Rol: Procesos de mantenimiento (ej. limpieza de caché, reportes de uso).

Implementación: Código Go ligero diseñado para ejecutarse y finalizar (Jobs de corta duración).

3. Infraestructura como Código (Terraform - Azure)
El agente debe generar los archivos .tf necesarios para levantar en Azure:

Azure Kubernetes Service (AKS): Un cluster básico para orquestar los contenedores.

Azure Cache for Redis: Instancia administrada de Redis (o un contenedor dentro de AKS para pruebas).

Azure Container Registry (ACR): Para almacenar las imágenes de Docker de Go, Python y el Frontend.

Network: VNet, Subnets y Network Security Groups necesarios.


4. Flujo de Datos
El usuario envía un prompt desde el Frontend.

Go recibe la petición, la valida y la empuja a una lista en Redis.

El worker en Python extrae la tarea, procesa el texto y publica el resultado en un canal de Redis usando el client_id.

Go recibe la notificación de Redis y envía el mensaje final al Frontend a través del WebSocket activo.

5. Orquestación (Kubernetes Manifests)
Deployments: Para el Gateway (Go) y los Workers (Python).

Horizontal Pod Autoscaler (HPA): Escalar los Workers de Python automáticamente según la carga.

CronJob Resource: Configurar la ejecución periódica del binario de mantenimiento en Go.

Ingress: Configuración de Nginx para permitir el Upgrade de protocolos (HTTP -> WebSocket).

6. Entregables del Agente de Codificación
Directorio terraform/: Scripts para aprovisionar los recursos en Azure.

Directorio gateway-go/: Código del middleware.

Directorio worker-python/: Código del procesador de IA.

Directorio frontend/: Código de la interfaz de usuario.

k8s-manifests/: Archivos YAML de Kubernetes (Deployments, Services, Ingress).

Directorio /cron-jobs: Tareas de mantenimiento en Go.

README.md: Guía paso a paso: terraform apply -> docker build -> kubectl apply.
