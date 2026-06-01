# Farm Fusion

**Intelligent Agricultural Recommendation System with Weather Monitoring**

A platform that combines machine learning, real-time weather monitoring, and automated notifications to help farmers make data-driven decisions about crop selection and fertilizer usage.

This document covers not just what was built, but why each technical decision was made.

---

## Why This Project Matters

### The Problem
Farmers face critical decisions daily:
- **What crop should I plant?** (Wrong choice = entire season lost)
- **What fertilizer do I need?** (Wrong amount = money wasted or crops damaged)
- **Will weather harm my crops?** (Late warning = no time to protect)

### The Solution
An intelligent system that:
- Recommends optimal crops based on soil conditions (99.32% accuracy)
- Suggests precise fertilizer types based on soil and crop data
- Monitors weather 24/7 and sends automated alerts before dangerous conditions
- Scales to handle thousands of farms with minimal latency

### Real-World Impact
- **Time Saved:** Automated daily weather checks for all farms
- **Cost Reduction:** Precise fertilizer recommendations prevent waste
- **Risk Mitigation:** Early weather warnings protect crops
- **Data-Driven:** ML models trained on 2,200+ agricultural data points

---

## System Architecture

### High-Level Overview

![Homepage](./static/farm-fusion.png)

### Architecture Principles

**Clean Architecture**
- Domain entities are independent of frameworks
- Business logic isolated from infrastructure
- Dependencies point inward (Dependency Inversion)
- Go API: Authentication, business logic, orchestration
- Python ML: Model inference (scikit-learn)

**Event-Driven Architecture**
- RabbitMQ decouples notification generation from email sending
- Async processing prevents API blocking
- Retry logic for failed emails

---

## API Endpoints

| Method | Endpoint | Auth | Description |
|--------|----------|------|-------------|
| POST | `/api/v1/auth/register` | No | Register a new user |
| POST | `/api/v1/auth/login` | No | Login and receive tokens |
| POST | `/api/v1/auth/refresh` | No | Refresh access token |
| POST | `/api/v1/farms` | Yes | Create a new farm |
| GET | `/api/v1/farms` | Yes | List all farms for user |
| GET | `/api/v1/farms/:id` | Yes | Get a specific farm |
| DELETE | `/api/v1/farms/:id` | Yes | Delete a farm |
| POST | `/api/v1/farms/:id/alerts` | Yes | Set weather alert thresholds |
| POST | `/api/v1/predict/crop` | Yes | Get crop recommendation |
| POST | `/api/v1/predict/fertilizer` | Yes | Get fertilizer recommendation |
| GET | `/health` | No | Health check |

---

## Domain Deep Dive

### ML Recommendation Domain

**Problem:** Provide accurate crop and fertilizer recommendations using ML models

**Architecture:**

```
┌─────────────────────────────────────────────────────────┐
│              ML RECOMMENDATION SYSTEM                    │
└─────────────────────────────────────────────────────────┘

Go Backend                    Python ML Service
    │                              │
    │  POST /predict/crop          │
    ├─────────────────────────────▶│
    │  {N, P, K, temp, humidity,   │
    │   ph, rainfall}               │
    │                              │
    │                         ┌────┴────┐
    │                         │ Load    │
    │                         │ Model   │
    │                         └────┬────┘
    │                              │
    │                              ▼
    │                         ┌──────────────┐
    │                         │ Run Various  │
    │                         │model & choose│
    │                         └──────┬───────┘
    │                              │
    │                         ┌────▼────┐
    │                         │ Get Top │
    │                         │ 3 Probs │
    │                         └────┬────┘
    │                              │
    │  {crop: "rice",              │
    │   confidence: 0.99,          │
    │◀─────────────────────────────┤
    │   alternatives: [...]}       │
    │                              │


Model Training (Offline):

┌──────────────┐    ┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│ CSV Dataset  │───▶│ Preprocess   │───▶│ Train RF     │───▶│ Save .pkl   │
│ 2200 samples │    │ • Normalize  │    │ • 100 trees  │    │ • Model      │
└──────────────┘    │ • Encode     │    │ • Max depth  │    │ • Encoders   │
                    └──────────────┘    └──────────────┘    └──────────────┘
```

**What We Did:**
- Trained various models and chose the best performer on agricultural datasets
- Created a separate Python FastAPI service for ML inference
- Implemented HTTP client in Go to call the ML service
- Added confidence thresholds and warnings

**Why Python for ML, Go for API:**
Python has the richest ML ecosystem (scikit-learn, pandas, numpy). Go excels at building fast, concurrent web APIs. Separating them lets each language do what it does best.

**Why Classifier (Not Regressor)?**

Input: Soil nutrients (N, P, K), weather (temp, humidity, rainfall), pH.
Output: **Crop name** (rice, wheat, maize, etc.) — **Discrete categories**

Classification predicts discrete categories like "rice" or "wheat". Regression predicts continuous numbers like 45.7 or 123.4. Since we're predicting *which crop*, not *how much of something*, classification is the correct approach.

**Why Random Forest (Not a Single Decision Tree)?**

```
Single Decision Tree Problems:
                    [N > 50?]
                   /         \
              [Yes]           [No]
             /                    \
      [P > 30?]              [Humidity > 80?]
      /      \                /            \
   Rice    Wheat          Maize          Jute

Problems:
 • Overfitting - Memorizes training data
 • High variance - Small data change = completely different tree
 • Unstable - Sensitive to noise
 • Lower accuracy - Single perspective
```

```
Random Forest = Ensemble of Many Trees:

Tree 1: Focuses on N, P, K
Tree 2: Focuses on Temperature, Humidity
Tree 3: Focuses on pH, Rainfall
...
Tree 100: Different feature combinations

Final Prediction = Majority Vote

Tree 1: Rice (90%)
Tree 2: Rice (85%)
Tree 3: Wheat (60%)
Tree 4: Rice (95%)
...
Tree 100: Rice (88%)

Result: Rice (87 trees voted Rice)
```

We tested multiple models (Decision Tree, SVM, KNN, Logistic Regression) and Random Forest consistently gave the best results.

**Model Performance:**
- **Crop Recommendation:** 99.32% accuracy (2,200 samples)
- **Fertilizer Recommendation:** ~95% accuracy (variable by soil type)
- **Inference Time:** <50ms per prediction
- **Model Size:** ~2MB total

---

### Why Round Coordinates for Weather Caching?

**What We Did:** Round GPS coordinates to 2 decimal places and use them as cache keys.

```go
Farm A: lat=23.8103, lon=90.4125 → location_key="23.81_90.41"
Farm B: lat=23.8156, lon=90.4189 → location_key="23.82_90.42"
Farm C: lat=23.8099, lon=90.4134 → location_key="23.81_90.41"

Result: Farm A and C share the same weather cache entry
```

**Why not use exact coordinates?**

**API Cost:**
```
Without Rounding:
- 100 farms × unique coords = 100 API calls/day

With Rounding (2 decimal places):
- 100 farms grouped into ~4 location keys = 4 API calls/day
- Cost reduction: 96%
```

**Accuracy is not sacrificed:**
```
0.01° latitude  ≈ 1.11 km
0.01° longitude ≈ 1.11 km (at equator)

OpenWeather grid resolution: ~10-15 km squares
→ Farms within 1 km receive identical forecast data anyway
→ Temperature difference between two farms 650m apart: ±0.1°C (negligible)
```

**Cache efficiency:**
```
Without Rounding: Cache hit rate ~5%  (every farm is unique)
With Rounding:    Cache hit rate ~95% (farms share keys)
Performance gain: 20x faster on cache hits
```

**Scales naturally:**
```
100 farms   → ~4 unique location keys   (under free API tier)
10,000 farms → ~400 unique location keys (still under free tier)
```

---

### Weather Notification Domain

**Problem:** Automatically alert farmers about dangerous weather conditions

**Architecture:**
```
┌─────────────────────────────────────────────────────────────────┐
│                    WEATHER NOTIFICATION SYSTEM                  │
└─────────────────────────────────────────────────────────────────┘

┌────────────────────┐   ┌─────────────────────────────────────┐
│   CRON (5 AM)      │   │        DATABASE QUERIES             │
│   ───────────      │   │   ───────────────────────────       │
│ • Start daily      │   │   • Fetch all farms                 │
│   scheduler        │──▶│   • Get user emails per farm        │
└────────────────────┘   │   • Get alert thresholds            │
                         └─────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────┐
│                      FARM PROCESSING LOOP                       │
├─────────────────────────────────────────────────────────────────┤
│ 1. Check Redis Cache ──────┐                                    │
│    • HIT: Use cached       │   ┌─────────────────────────────┐  │
│    • MISS: Call API        │◀──│ OPENWEATHER API CALL        │  │
│                            │   │ ───────────────────────     │  │
│ 2. Detect Alerts:          │   │ • Get 24-hour forecast      │  │
│    • Temp < 15°C           │   │ • Cache result (3hr TTL)    │  │
│    • Temp > 35°C           │   └─────────────────────────────┘  │
│    • Rainfall > 50mm       │                                    │
│    • Humidity > 80%        │                                    │
│    • Wind > 40 km/h        │                                    │
│                            │                                    │
│ 3. Generate Summary        │                                    │
└────────────────────────────┼────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                   RABBITMQ PUBLISHING                           │
│  ┌──────────────────────────────────────────────────────────┐   │
│  │ {                                                        │   │
│  │   "farm_id": "123",                                      │   │
│  │   "user_email": "user@example.com",                      │   │
│  │   "alerts": ["Temp > 35°C"],                             │   │
│  │   "summary": "Sunny, high of 38°C"                       │   │
│  │ }                                                        │   │
│  └──────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                    WORKER CONSUMER                              │
├─────────────────────────────────────────────────────────────────┤
│ 1. Receive message from queue                                   │
│ 2. Generate HTML email (alerts + forecast summary)              │
│ 3. Send via SMTP                                                │
│ 4. Log to notification_log table                                │
│ 5. Acknowledge message                                          │
└─────────────────────────────────────────────────────────────────┘
```

**Why Scheduled at 5 AM, Not Real-Time?**

```
Farmer's Daily Schedule:
├─ 5:00 AM - Wake up, check phone
├─ 5:30 AM - Plan day based on weather
├─ 6:00 AM - Start farm work
├─ 12:00 PM - Lunch break
└─ 6:00 PM - End of work day
```

A 5 AM notification gives farmers 1–2 hours to prepare before work starts. Real-time notifications would arrive too late to act on (e.g. "heavy rain in 30 minutes" when you're already in the field) or too early (2 AM weather changes nobody can act on until morning).

**Technical reasons against real-time:**

```
API Calls:
- Real-time (100 farms × 24 checks/hr × 24 hrs): 57,600 calls/day → $50-100/month
- Scheduled (5 AM, with caching):                 ~8 calls/day     → FREE

Database queries:
- Real-time: 6,000 queries/hour
- Scheduled: 100 queries/day (99.3% reduction)
```

**UX reasons against real-time:**

```
Real-time notifications in one day:
├─ 6:00 AM - "Rain expected at 3 PM"
├─ 9:00 AM - "Rain moved to 4 PM"
├─ 12:00 PM - "Rain now at 2 PM"
├─ 3:00 PM - "Rain cancelled"
└─ 6:00 PM - "Rain back on at 8 PM"
→ Result: User unsubscribes

Scheduled alert grouping:
└─ One alert: "Temperature < 15°C from 10 AM - 1 PM (4 hours)"
→ Result: Clear, concise, actionable
```

**Future Enhancement:** Add emergency-only real-time alerts for truly severe conditions (e.g. tornado, flash flood) — best of both worlds.

---

### Why RabbitMQ?

**Requirements:** Send emails asynchronously, retry on failure, decouple producers from consumers.

**Why not a simple goroutine?**
A goroutine sending email inline with the API request means if the SMTP server is slow or down, the farmer's API call hangs or fails. The notification concern is completely separate from the farm management concern — they should not share a failure surface.

**Why not Redis as a queue?**
Redis can be used as a queue but it lacks built-in dead-letter queues, per-message TTL, and consumer acknowledgment semantics. RabbitMQ was purpose-built for message brokering and handles retry logic and failed message routing out of the box.

**Why not Kafka?**
Kafka is designed for high-throughput event streaming (millions of messages/second). This system sends at most a few hundred emails per day. Kafka would be significant operational overhead for no benefit at this scale.

**RabbitMQ fits because:**
- Simple pub/sub is all we need
- Built-in retry and dead-letter queue
- Message acknowledgment prevents lost emails
- Low operational overhead
- Free tier on CloudAMQP covers this use case easily

---

### Authentication Domain

**Problem:** Secure user access with token-based authentication

**Architecture:**
```
POST /api/v1/auth/register
    │
    ├─▶ Validate Input (email, password strength)
    ├─▶ Hash Password (bcrypt, cost=10)
    ├─▶ Store User in PostgreSQL
    └─▶ Return User ID

POST /api/v1/auth/login
    │
    ├─▶ Fetch User by Email
    ├─▶ Compare Password Hash
    ├─▶ Generate JWT Access Token (15 min expiry)
    ├─▶ Generate Refresh Token (7 days, stored in DB)
    └─▶ Return Both Tokens

POST /api/v1/auth/refresh
    │
    ├─▶ Validate Refresh Token from DB
    ├─▶ Check Expiry & Revocation
    └─▶ Return New Access Token
```

**Why this approach:**
- **JWT for stateless auth:** No session storage needed, scales horizontally
- **Refresh tokens in DB:** Enables logout and revocation (pure JWT cannot be revoked before expiry)
- **Short access token expiry (15 min):** Limits damage if a token is stolen
- **Bcrypt over SHA256:** Purpose-built for passwords, has built-in salt, adjustable cost factor

**Alternatives considered:**
- **Session-based auth:** Requires a DB or Redis lookup on every single request — adds latency and a stateful dependency
- **OAuth2:** Adds significant complexity for no real benefit without third-party login requirements
- **API Keys:** No expiration, harder to rotate, less secure for end-user auth

**Future Enhancements:**
- Add 2FA (TOTP)
- Rate limiting on login attempts
- Password reset via email
- OAuth2 for social login

---

### Farm Management Domain

**Problem:** Users need to manage multiple farms with GPS coordinates and per-farm alert thresholds

**Architecture:**
```
User (1) ──────── (N) Farm
    │                  │
    │                  ├─ ID (UUID)
    │                  ├─ Name
    │                  ├─ Latitude
    │                  ├─ Longitude
    │                  ├─ Location Key (for weather cache)
    │                  └─ Timestamps
    │
    └─────────────────▶ Weather Alerts (N)
                           │
                           ├─ Metric (temp/rain/humidity/wind)
                           ├─ Operator (<, >, =)
                           ├─ Value (threshold)
                           └─ Is Enabled

POST /api/v1/farms
    │
    ├─▶ Extract User ID from JWT
    ├─▶ Validate Coordinates (-90 to 90, -180 to 180)
    ├─▶ Generate Location Key (rounded lat_lon)
    ├─▶ Store in PostgreSQL
    └─▶ Return Farm Object
```

**Design decisions:**
- **UUIDs for IDs:** Better for distributed systems, no sequential ID guessing
- **Location key on the farm record:** Avoids recomputing it on every weather check
- **Ownership verification middleware:** Users can only read/modify their own farms

**Future Enhancements:**
- Farm boundaries (polygon coordinates)
- Multiple crops per farm
- Soil test history tracking
- Farm sharing (multiple users per farm)

---

## Quick Setup

### Prerequisites

```bash
- Go 1.21+
- Python 3.8+
- PostgreSQL 14+
- Redis 6+
- RabbitMQ 3.9+
```

### Environment Variables

Copy `.env.example` to `.env` and fill in the values:

```env
# Database
DATABASE_URL=postgresql://postgres:password@localhost:5432/farm_fusion

# Redis
REDIS_URL=redis://localhost:6379

# RabbitMQ
RABBITMQ_URL=amqp://guest:guest@localhost:5672/

# ML Service
ML_SERVICE_URL=http://localhost:8000

# Auth
JWT_SECRET=your-secret-key-here

# Weather
OPENWEATHER_API_KEY=your-api-key

# Email (SMTP)
SMTP_HOST=smtp.gmail.com
SMTP_PORT=587
SMTP_USER=your@gmail.com
SMTP_PASS=your-app-password

# Server
PORT=8080
ENV=development
```

### 1. Clone & Configure

```bash
git clone https://github.com/Soyaib10/farm-fusion.git
cd farm-fusion
cp .env.example .env
# Edit .env with your credentials
```

### 2. Database Setup

```bash
createdb farm_fusion
psql -d farm_fusion -f migrations/*.up.sql
```

### 3. Start ML Service

```bash
cd ml_service
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python train_models.py  # First time only
uvicorn app.main:app --host 0.0.0.0 --port 8000
```

### 4. Start Go Backend

```bash
go mod download
go build -o bin/api cmd/api/main.go
./bin/api
```

### 5. Start Background Services

```bash
# Terminal 1 — Scheduler (runs daily at 5 AM)
go build -o bin/scheduler cmd/scheduler/main.go
./bin/scheduler

# Terminal 2 — Worker (consumes RabbitMQ queue)
go build -o bin/worker cmd/worker/main.go
./bin/worker
```

---

## Mistakes & Lessons

**Mistake 1: Over-engineering Early**
- Initially wanted to use gRPC and microservices everywhere
- Learned: Start simple, add complexity only when justified

**Mistake 2: Not Planning the Database Schema Upfront**
- Had to add the `location_key` column after the fact
- Learned: Think about access patterns before writing the first migration — migrations are painful to undo

**Mistake 3: Ignoring Error Handling**
- Early code returned generic error messages
- Learned: Specific, descriptive errors save hours of debugging

---

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.

---

## Author

**Md. Soyaib Rahman Zihad**
- GitHub: [Soyaib10](https://github.com/Soyaib10)
- LinkedIn: [Md. Soyaib Rahman](https://www.linkedin.com/in/md-soyaib-rahman-788261194/)
- Email: soyaibzihad10@gmail.com