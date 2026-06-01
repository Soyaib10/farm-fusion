# Farm Fusion — Complete Free Deployment Plan

## Project Stack

```
Frontend   → Next.js
Backend    → Go (API + Scheduler + Worker)
ML Service → Python FastAPI
Database   → PostgreSQL
Cache      → Redis
Queue      → RabbitMQ
```

---

## Deployment Map (All Free)

| Service | Platform | Free Tier |
|---|---|---|
| Next.js Frontend | Vercel | Completely free |
| Go Backend (API + Worker + Scheduler) | Render | 750 hrs/month |
| Python ML Service | Render | 750 hrs/month |
| PostgreSQL | Supabase | 500MB free |
| Redis | Upstash | 10,000 req/day free |
| RabbitMQ | CloudAMQP | 1M messages/month free |

---

## Order of Work (Bottom to Top)

```
Step 1 → Supabase   (PostgreSQL setup)
Step 2 → Upstash    (Redis setup)
Step 3 → CloudAMQP  (RabbitMQ setup)
Step 4 → Render     (Python ML Service deploy)
Step 5 → Render     (Go API deploy)
Step 6 → Render     (Go Scheduler deploy — Background Worker)
Step 7 → Render     (Go Worker/Consumer deploy — Background Worker)
Step 8 → Vercel     (Next.js Frontend deploy)
```

Always set up databases and infrastructure first, then backend services, and frontend last — because each step needs the URL from the previous one.

---

## STEP 1 — Supabase (PostgreSQL)

### What to do:
1. Create an account at https://supabase.com (sign in with GitHub)
2. Click "New Project"
3. Remember your database password
4. Once the project is created: Settings → Database → Connection String → copy the "URI"

### You will get a URL like this:
```
postgresql://postgres:[PASSWORD]@db.[PROJECT_REF].supabase.co:5432/postgres
```

### Save this URL in your `.env`:
```
DATABASE_URL=postgresql://postgres:[PASSWORD]@db.[PROJECT_REF].supabase.co:5432/postgres
```

### Run Migrations:
Supabase Dashboard → SQL Editor → paste the contents of your `migrations/*.up.sql` files and run them.

---

## STEP 2 — Upstash (Redis)

### What to do:
1. Create an account at https://upstash.com
2. "Create Database" → Redis → Region: "US-East-1" or closest to you
3. Once created, copy the "REDIS_URL" from the Dashboard

### You will get a URL like this:
```
rediss://default:[PASSWORD]@[HOST].upstash.io:6379
```

### Save in `.env`:
```
REDIS_URL=rediss://default:[PASSWORD]@[HOST].upstash.io:6379
```

---

## STEP 3 — CloudAMQP (RabbitMQ)

### What to do:
1. Create an account at https://cloudamqp.com
2. "Create New Instance" → Plan: select "Little Lemur" (Free)
3. Once created, copy the "AMQP URL" from the Dashboard

### You will get a URL like this:
```
amqps://[USER]:[PASSWORD]@[HOST].cloudamqp.com/[VHOST]
```

### Save in `.env`:
```
RABBITMQ_URL=amqps://[USER]:[PASSWORD]@[HOST].cloudamqp.com/[VHOST]
```

---

## STEP 4 — Python ML Service Deploy (Render)

### First, create this file:

**`ml_service/Dockerfile`**
```dockerfile
FROM python:3.10-slim

WORKDIR /app

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY . .

# Remove this line if models are already trained and committed
RUN python train_models.py

EXPOSE 8000

CMD ["uvicorn", "app.main:app", "--host", "0.0.0.0", "--port", "8000"]
```

### Deploy on Render:
1. Create an account at https://render.com (sign in with GitHub)
2. "New" → "Web Service"
3. Connect your GitHub repo: `Soyaib10/farm-fusion`
4. Settings:
   - **Name:** `farm-fusion-ml`
   - **Root Directory:** `ml_service`
   - **Runtime:** `Docker`
   - **Instance Type:** Free
5. Click Deploy
6. Once deployed, you will get a URL: `https://farm-fusion-ml.onrender.com`

---

## STEP 5 — Go API Deploy (Render)

### First, create this file:

**`Dockerfile.api`** (in repo root)
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -o bin/api cmd/api/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin/api .

EXPOSE 8080

CMD ["./api"]
```

### Add CORS to the Go Backend:
The Next.js frontend will be sending requests to your API, so you need to add CORS headers to the Go API.

```go
// In main.go or a middleware file
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "https://your-frontend.vercel.app")
        w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
        if r.Method == "OPTIONS" {
            w.WriteHeader(http.StatusOK)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

### Add a Health Endpoint to the Go API:
This is needed to keep the service alive on Render's free tier (explained in the Sleep Fix section below).

```go
// GET /health
func healthHandler(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte(`{"status": "ok"}`))
}
```

### Deploy on Render:
1. "New" → "Web Service"
2. Settings:
   - **Name:** `farm-fusion-api`
   - **Root Directory:** `/` (root)
   - **Runtime:** `Docker`
   - **Dockerfile Path:** `Dockerfile.api`
   - **Instance Type:** Free
3. Add Environment Variables:
   ```
   DATABASE_URL=<URL from Supabase>
   REDIS_URL=<URL from Upstash>
   RABBITMQ_URL=<URL from CloudAMQP>
   ML_SERVICE_URL=https://farm-fusion-ml.onrender.com
   JWT_SECRET=<any random secret string>
   OPENWEATHER_API_KEY=<your API key>
   SMTP_HOST=smtp.gmail.com
   SMTP_PORT=587
   SMTP_USER=<your email>
   SMTP_PASS=<your app password>
   PORT=8080
   ENV=production
   ```
4. Click Deploy
5. You will get a URL: `https://farm-fusion-api.onrender.com`

---

## STEP 6 — Go Scheduler Deploy (Render Background Worker)

### First, create this file:

**`Dockerfile.scheduler`**
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/scheduler cmd/scheduler/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin/scheduler .
CMD ["./scheduler"]
```

### Deploy on Render:
1. "New" → **"Background Worker"** (NOT Web Service!)
2. Settings:
   - **Name:** `farm-fusion-scheduler`
   - **Dockerfile Path:** `Dockerfile.scheduler`
3. Add the same Environment Variables as the API
4. Click Deploy

---

## STEP 7 — Go Worker/Consumer Deploy (Render Background Worker)

### First, create this file:

**`Dockerfile.worker`**
```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o bin/worker cmd/worker/main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/bin/worker .
CMD ["./worker"]
```

### Deploy on Render:
1. "New" → **"Background Worker"**
2. Settings:
   - **Name:** `farm-fusion-worker`
   - **Dockerfile Path:** `Dockerfile.worker`
3. Add the same Environment Variables
4. Click Deploy

---

## STEP 8 — Next.js Frontend Deploy (Vercel)

### Add this file to your Next.js project:

**`frontend/.env.production`**
```
NEXT_PUBLIC_API_URL=https://farm-fusion-api.onrender.com
```

### Deploy on Vercel:
1. Create an account at https://vercel.com (sign in with GitHub)
2. "New Project" → select your GitHub repo
3. Settings:
   - **Framework Preset:** Next.js
   - **Root Directory:** `frontend` (if it's in a subdirectory)
4. Environment Variables:
   ```
   NEXT_PUBLIC_API_URL=https://farm-fusion-api.onrender.com
   ```
5. Click Deploy
6. You will get a URL: `https://farm-fusion.vercel.app`

---

## After Deployment — Update CORS

Once you have your Vercel URL, update the CORS middleware in your Go API:

```go
w.Header().Set("Access-Control-Allow-Origin", "https://farm-fusion.vercel.app")
```

Then trigger a redeploy on Render.

---

## Fix for Render Free Tier Sleep Problem

On Render's free tier, services go to sleep after 15 minutes of inactivity. The first request after that takes 30–50 seconds to wake up.

**Solution:** Use https://cron-job.org (free) to ping your service every 10 minutes.

1. Create an account at https://cron-job.org
2. "Create cronjob":
   - URL: `https://farm-fusion-api.onrender.com/health`
   - Schedule: Every 10 minutes
3. This will keep your Go API awake at all times

---

## Complete Environment Variables List

All three services — Go API, Scheduler, and Worker — need these same variables:

```env
# Database
DATABASE_URL=postgresql://...

# Redis
REDIS_URL=rediss://...

# RabbitMQ
RABBITMQ_URL=amqps://...

# ML Service
ML_SERVICE_URL=https://farm-fusion-ml.onrender.com

# Auth
JWT_SECRET=your-super-secret-key-here

# Weather
OPENWEATHER_API_KEY=your-openweather-key

# Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your@gmail.com
SMTP_PASS=your-app-password

# Server
PORT=8080
ENV=production
```

---

## Final Architecture (Production)

```
User Browser
    │
    ▼
Vercel (Next.js)
    │
    │ HTTPS API calls
    ▼
Render (Go API — farm-fusion-api.onrender.com)
    │
    ├──→ Supabase (PostgreSQL)
    ├──→ Upstash (Redis)
    ├──→ CloudAMQP (RabbitMQ) ←── Render (Go Scheduler)
    │                                       │
    │                              Render (Go Worker) ──→ Email (SMTP)
    │
    └──→ Render (Python ML — farm-fusion-ml.onrender.com)
```

---

## Pre-Deployment Checklist

- [ ] `ml_service/Dockerfile` created
- [ ] `Dockerfile.api` created
- [ ] `Dockerfile.scheduler` created
- [ ] `Dockerfile.worker` created
- [ ] CORS middleware added to Go API
- [ ] `/health` endpoint added to Go API
- [ ] Migrations run on Supabase
- [ ] All environment variables collected
- [ ] OpenWeather API key obtained (https://openweathermap.org/api — free)
- [ ] Gmail App Password obtained (for SMTP)

---

## How to Get a Gmail App Password (for SMTP)

1. Go to your Google Account → Security → Enable 2-Step Verification
2. Go to Security → "App passwords"
3. Select "Mail" → "Other (Custom name)" → type "farm-fusion"
4. Click Generate → you will get a 16-character password
5. Use this as your `SMTP_PASS` value

---

## How to Get an OpenWeather API Key (Free)

1. Create an account at https://openweathermap.org/api
2. Go to the "API keys" tab
3. Copy the default key or generate a new one
4. Free tier allows 1,000 calls/day — more than enough for this project
