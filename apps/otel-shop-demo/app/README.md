# OTel Shop Demo

OpenTelemetry observability demonstration using a simple e-commerce microservices application.

## Architecture

- **Frontend**: React + Vite + OpenTelemetry Browser SDK
- **Backend API**: Go + Chi router + OpenTelemetry Go SDK
- **Database**: PostgreSQL with sample product data
- **Observability**: OpenTelemetry Collector → Tempo (traces) + Prometheus (metrics) + Loki (logs)

## Directory Structure

```
apps/examples/otel-shop-demo/
├── backend/                    # Go API service
│   ├── handlers/               # HTTP handlers
│   ├── models/                 # Data models
│   ├── database/               # DB connection & helpers
│   ├── telemetry/              # OTel SDK initialization
│   ├── main.go
│   ├── go.mod
│   └── Dockerfile
├── frontend/                   # React application
│   ├── src/
│   │   ├── components/         # React components
│   │   ├── services/           # API client
│   │   ├── telemetry/          # OTel Browser SDK
│   │   ├── App.jsx
│   │   └── main.jsx
│   ├── package.json
│   ├── nginx.conf
│   └── Dockerfile
└── database/
    └── init.sql                # DB schema + seed data

infrastructure/examples/otel-shop-demo/
└── k8s/base/                   # Kubernetes manifests
    ├── namespace.yaml
    ├── postgresql-statefulset.yaml
    ├── backend-deployment.yaml
    ├── frontend-deployment.yaml
    ├── httproute.yaml
    └── kustomization.yaml
```

## Build and Push Docker Images

### Backend

```bash
cd apps/examples/otel-shop-demo/backend

# Build
docker build -t ghcr.io/yu-min3/otel-shop-backend:latest .

# Push
docker push ghcr.io/yu-min3/otel-shop-backend:latest
```

### Frontend

```bash
cd apps/examples/otel-shop-demo/frontend

# Build
docker build -t ghcr.io/yu-min3/otel-shop-frontend:latest .

# Push
docker push ghcr.io/yu-min3/otel-shop-frontend:latest
```

## Deploy to Kubernetes

```bash
cd infrastructure/examples/otel-shop-demo/k8s

# Apply manifests
kubectl apply -k base/

# Check deployment status
kubectl get pods -n otel-shop-demo

# Check HTTPRoute
kubectl get httproute -n otel-shop-demo
kubectl describe httproute otel-shop-route -n otel-shop-demo
```

## Access the Application

After HTTPRoute is configured and DNS is set up:

**Frontend**: http://shop.dev.yu-min3.com

**Backend API**: http://shop.dev.yu-min3.com/api

### API Endpoints

- `GET /api/products` - Get all products
- `GET /api/products/:id` - Get product by ID
- `GET /api/products/search?q=keyword` - Search products
- `POST /api/cart` - Add item to cart
- `GET /api/cart/:userId` - Get user's cart
- `DELETE /api/cart/:userId/:productId` - Remove item from cart
- `POST /api/checkout` - Checkout
- `GET /api/orders?user_id=:userId` - Get user's orders

## Observability

### Grafana Dashboards

Access Grafana at: https://grafana.platform.yu-min3.com

**Check traces**: Navigate to "OpenTelemetry APM" dashboard
- View distributed traces across frontend → backend → database
- Analyze request latency and error rates

**Check metrics**: Navigate to "Prometheus" dashboard
- HTTP request rates
- Response times
- Error rates

**Check logs**: Navigate to "Loki" dashboard
- Application logs with trace ID correlation

### Tempo

Search for traces by:
- Service name: `otel-shop-frontend`, `otel-shop-backend`
- HTTP path: `/api/products`, `/api/cart`, etc.
- Status code: `200`, `404`, `500`

### Prometheus

Query metrics:
```promql
# HTTP request rate
rate(http_server_duration_count[5m])

# Error rate
rate(http_server_duration_count{http_status_code=~"5.."}[5m])

# P95 latency
histogram_quantile(0.95, rate(http_server_duration_bucket[5m]))
```

## Troubleshooting

### Backend can't connect to PostgreSQL

```bash
# Check PostgreSQL pod
kubectl get pods -n otel-shop-demo -l app=postgresql

# Check PostgreSQL logs
kubectl logs -n otel-shop-demo statefulset/postgresql

# Test connection from backend pod
kubectl exec -n otel-shop-demo deployment/backend -- nc -zv postgresql 5432
```

### Frontend can't reach Backend API

```bash
# Check backend service
kubectl get svc -n otel-shop-demo backend

# Test backend from frontend pod
kubectl exec -n otel-shop-demo deployment/frontend -- wget -O- http://backend:8080/health
```

### No traces in Tempo

```bash
# Check OTel Collector
kubectl get pods -n monitoring -l app.kubernetes.io/name=opentelemetry-collector

# Check collector logs
kubectl logs -n monitoring deployment/otel-collector-opentelemetry-collector

# Verify OTLP endpoint from backend pod
kubectl exec -n otel-shop-demo deployment/backend -- nc -zv otel-collector-opentelemetry-collector.monitoring.svc 4317
```

## Development

### Local Backend Development

```bash
cd apps/examples/otel-shop-demo/backend

# Set environment variables
export DATABASE_URL="postgresql://postgres:password@localhost:5432/shopdb?sslmode=disable"
export OTEL_SERVICE_NAME="otel-shop-backend"
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"

# Run
go run main.go
```

### Local Frontend Development

```bash
cd apps/examples/otel-shop-demo/frontend

# Install dependencies
npm install

# Run dev server
npm run dev

# Access at http://localhost:3000
```

## Resource Usage

- **Backend**: 200m CPU, 256Mi memory (requests)
- **Frontend**: 100m CPU, 128Mi memory (requests)
- **PostgreSQL**: 100m CPU, 256Mi memory (requests)
- **Total**: ~400m CPU, ~640Mi memory

All pods are configured with `nodeSelector: worker2` to avoid overloading worker1 (where monitoring stack runs).
