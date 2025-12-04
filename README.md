# Farm Fusion

**Intelligent Agricultural Recommendation System with Weather Monitoring**

A platform that combines machine learning, real-time weather monitoring, and automated notifications to help farmers make data-driven decisions about crop selection and fertilizer usage.

---
 I have shared my entire thought process here, why did I choose one thing over another. So if you are reading this then just bear with me, you will find it interesting. Some parts of implementation are under development but entire thought process is here. 

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
- **Data-Driven:** ML models trained on 2,200+ agricultural data points. 
---

## System Architecture

### High-Level Overview

<!-- ```
┌─────────────────────────────────────────────────────────────────┐
│                         CLIENT LAYER                             │
│                    (Mobile/Web/Postman)                          │
└────────────────────────────┬────────────────────────────────────┘
                             │ HTTPS/REST
                             ▼
┌─────────────────────────────────────────────────────────────────┐
│                      GO BACKEND API :8080                        │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │
│  │   Delivery   │  │   UseCase    │  │    Domain    │          │
│  │  (HTTP/REST) │─▶│  (Business)  │─▶│  (Entities)  │          │
│  └──────────────┘  └──────────────┘  └──────────────┘          │
│         │                  │                  │                  │
│         └──────────────────┴──────────────────┘                 │
│                            │                                     │
│                            ▼                                     │
│                   ┌─────────────────┐                           │
│                   │  Infrastructure │                           │
│                   └─────────────────┘                           │
└────────────────────────────┬────────────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
        ▼                    ▼                    ▼
┌──────────────┐    ┌──────────────┐    ┌──────────────┐
│  PostgreSQL  │    │  ML Service  │    │   Redis      │
│   :5432      │    │   :8000      │    │   :6379      │
│              │    │              │    │              │
│ • Users      │    │ • Crop Model │    │ • Weather    │
│ • Farms      │    │ • Fert Model │    │   Cache      │
│ • Alerts     │    │ • FastAPI    │    │ • 3hr TTL    │
└──────────────┘    └──────────────┘    └──────────────┘
        │
        │
        ▼
┌──────────────────────────────────────────────────────────┐
│              BACKGROUND PROCESSING LAYER                  │
│                                                           │
│  ┌──────────────┐         ┌──────────────┐              │
│  │  Scheduler   │────────▶│  RabbitMQ    │              │
│  │  (Cron 5AM)  │  Pub    │   Queue      │              │
│  └──────────────┘         └──────┬───────┘              │
│                                   │ Sub                  │
│  ┌──────────────┐                │                      │
│  │ OpenWeather  │                ▼                      │
│  │     API      │         ┌──────────────┐              │
│  └──────┬───────┘         │    Worker    │              │
│         │                 │  (Consumer)  │              │
│         │ Forecast        └──────┬───────┘              │
│         │                        │                      │
│         ▼                        ▼                      │
│  ┌──────────────┐         ┌──────────────┐              │
│  │ Alert Logic  │         │  SMTP Client │              │
│  │  (Threshold) │         │  (Gmail)     │              │
│  └──────────────┘         └──────────────┘              │
│                                   │                      │
└───────────────────────────────────┼──────────────────────┘
                                    │
                                    ▼
                            ┌──────────────┐
                            │   📧 Email   │
                            │   to Farmer  │
                            └──────────────┘
``` -->

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
## Domain Deep Dive
Start with the recommendation and notification part first. Later other parts will be discussed. 

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
- Trained various models and choose best models on agricultural datasets
- Created separate Python FastAPI service for ML inference
- Implemented HTTP client in Go to call ML service
- Added confidence thresholds and warnings

**Why This Way:**
- **Why Python:** Python is best for ML, Go is best for web APIs
- **Why Classifier (Not Regressor)?**

  Here our Task:

  Input: Soil nutrients (N, P, K), weather (temp, humidity, rainfall), pH. 
  Output: **Crop name** (rice, wheat, maize, etc.) - **Discrete categories**

  Classification works on Discrete categories like "rice", "wheat", "maize" , "Crop/Fertilizer names". Regression works on Continuous numbers like 45.7, 123.4 not predicting quantities. We're not predicting "how much" (quantity), We're predicting "which one" (category).


**Why Random Forest (Not Decision Tree)?**

**Decision Tree Problems**

```
Single Decision Tree:
                    [N > 50?]
                   /         \
              [Yes]           [No]
             /                    \
      [P > 30?]              [Humidity > 80?]
      /      \                /            \
   Rice    Wheat          Maize          Jute

Problems:
 Overfitting - Memorizes training data
 High variance - Small data change = completely different tree
 Unstable - Sensitive to noise
 Lower accuracy - Single perspective
```

### Random Forest Solution

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
We tested other models and got best result for Random Forest.

**Model Performance:**
- **Crop Recommendation:** 99.32% accuracy (2200 samples)
- **Fertilizer Recommendation:** ~95% accuracy (variable by soil type)
- **Inference Time:** <50ms per prediction
- **Model Size:** ~2MB total

---

### 4. Weather Notification Domain

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
│                                                                 │
│ 2. Generate HTML email:                                         │
│    • Alert section (if any alerts)                              │
│    • Forecast summary                                           │
│                                                                 │
│ 3. Send via SMTP                                                │
│                                                                 │
│ 4. Log to notification_log table:                               │
│    • Success: email_sent = true                                 │
│    • Failure: error_message                                     │
│                                                                 │
│ 5. Acknowledge message                                          │
└─────────────────────────────────────────────────────────────────┘


CACHING STRATEGY:

Redis Key: weather:forecast:{lat}_{lon}
TTL: 3 hours
Why: OpenWeather updates every 3 hours, saves API calls

Example:
Farm A (23.81, 90.41) ─┐
Farm B (23.82, 90.42) ─┼─▶ Same location key ─▶ 1 API call
Farm C (23.80, 90.40) ─┘
```

**What We Did:**
- Implemented cron-based scheduler (runs at 5 AM daily)
- Used RabbitMQ for async email processing
- Cached weather data in Redis (1-hour TTL)
- Created alert detection logic with configurable thresholds
- Built HTML email templates with alert details


**Why Scheduled Notifications at 5 AM (Not Instant Messages)?**

**What We Did:** Send weather notifications once daily at 5:00 AM 

**Alternative:** Send instant notifications whenever weather changes


**Why 5 AM Specifically?**

**User Behavior Analysis:**
```
Farmer's Daily Schedule:
├─ 5:00 AM - Wake up, check phone
├─ 5:30 AM - Plan day based on weather
├─ 6:00 AM - Start farm work
├─ 12:00 PM - Lunch break
└─ 6:00 PM - End work, too late to react
```

**Reasoning:**
- **Early enough to plan:** Farmers can adjust their day before starting work
- **Not too early:** 5 AM is when most farmers wake up
- **Actionable window:** 1-2 hours to prepare equipment, protect crops
- **Predictable:** Users expect notification at same time daily

**Why Not Other Times?**
- **Midnight:** Users asleep, notification ignored
- **8 AM:** Too late, already started work
- **Evening (6 PM):** Can't act on tomorrow's weather today

---

### Why NOT Instant/Real-Time Notifications?

#### Technical Challenges

**1. API Rate Limits**
```
OpenWeather API Free Tier: 1,000 calls/day

Instant Approach:
- 100 farms × 24 checks/hour = 2,400 calls/hour
- 2,400 × 24 hours = 57,600 calls/day
- Cost: $50-100/month for API calls

Scheduled Approach (5 AM):
- 100 farms × 1 check/day = 100 calls/day
- With caching: ~8 calls/day (nearby farms share cache)
- Cost: FREE (under 1,000 limit)

Savings: 99.8% reduction in API calls
```

**2. Weather Data Doesn't Change That Fast**
```
OpenWeather Update Frequency: Every 3 hours

Checking every minute:
├─ 5:00 AM - Forecast: Rain at 2 PM
├─ 5:01 AM - Forecast: Rain at 2 PM (same)
├─ 5:02 AM - Forecast: Rain at 2 PM (same)
└─ ... (177 identical checks)
└─ 8:00 AM - Forecast: Rain at 2 PM (finally updated)

Result: 177 wasted API calls for same data
```

**3. Email Fatigue**
```
Instant Notifications:
├─ 5:00 AM - "Temperature dropping to 14°C at 2 PM"
├─ 6:00 AM - "Temperature dropping to 13°C at 2 PM" (updated forecast)
├─ 7:00 AM - "Temperature dropping to 14°C at 2 PM" (forecast changed back)
└─ 8:00 AM - "Temperature dropping to 13°C at 2 PM"

User Experience: 4 emails in 3 hours, all saying similar things
Result: User unsubscribes or ignores emails
```

**4.Costs**: This way cost is much higher due to over API calls.


**5. Database Load**
```
Instant Approach:
- Continuous polling: SELECT * FROM farms every minute
- 100 farms × 60 checks/hour = 6,000 queries/hour
- Database always busy

Scheduled Approach:
- One batch query: SELECT * FROM farms once/day
- 100 farms × 1 check/day = 100 queries/day
- Database mostly idle

Load Reduction: 99.3% fewer queries
```

#### User Experience Challenges

**1. Notification Overload**
```
Problem: Weather forecasts change frequently

Example Day:
├─ 6:00 AM - "Rain expected at 3 PM"
├─ 9:00 AM - "Rain moved to 4 PM"
├─ 12:00 PM - "Rain now at 2 PM"
├─ 3:00 PM - "Rain cancelled"
└─ 6:00 PM - "Rain back on at 8 PM"

Result: 5 notifications, user confused and annoyed
```

**2. Actionability**
```
Instant notification at 1 PM: "Heavy rain in 30 minutes"

Farmer's situation:
- Already in the field
- Equipment not nearby
- Can't protect crops in 30 minutes
- Notification causes stress, not help

Better: Morning notification
- "Heavy rain expected at 1:30 PM"
- Farmer can plan: finish work by 1 PM, bring equipment
- Actionable and helpful
```

**3. Sleep Disruption**
```
Instant Approach:
- Weather changes at 2 AM
- Notification wakes farmer
- Can't do anything until morning anyway
- Lost sleep for no benefit

Scheduled Approach:
- All changes summarized in 5 AM email
- Farmer wakes naturally
- Gets complete picture
- Can act immediately
```

#### Business Logic Challenges

**1. Alert Grouping**
```
Instant Approach:
├─ Alert 1: "Temperature < 15°C at 10 AM"
├─ Alert 2: "Temperature < 15°C at 11 AM"
├─ Alert 3: "Temperature < 15°C at 12 PM"
└─ Alert 4: "Temperature < 15°C at 1 PM"

Problem: 4 separate notifications for same condition

Scheduled Approach:
└─ One alert: "Temperature < 15°C from 10 AM - 1 PM (4 hours)"

Result: Clear, concise, actionable
```

**Our Approach:**
- Daily scheduled for routine planning
- Future: Add emergency alerts for severe weather
- Best of both worlds

---

## 2. Why Rounding Latitude/Longitude (Location Key)?

### The Design Decision

**What We Did:** Round coordinates and create location keys for caching

```go
// Example
Farm A: lat=23.8103, lon=90.4125 → location_key="23.81_90.41"
Farm B: lat=23.8156, lon=90.4189 → location_key="23.82_90.42"
Farm C: lat=23.8099, lon=90.4134 → location_key="23.81_90.41"

Result: Farm A and C share same weather cache
```

**Alternative:** Use exact coordinates for each farm

---

### Why Round Coordinates?

#### Technical Challenges

**1. API Cost Explosion**
```
Without Rounding (Exact Coordinates):

100 farms with unique coordinates:
├─ Farm 1: 23.810345, 90.412567
├─ Farm 2: 23.810389, 90.412601
├─ Farm 3: 23.810412, 90.412634
└─ ... (all slightly different)

API Calls: 100 unique calls/day
Cost: Hits rate limits quickly

With Rounding (2 decimal places):

100 farms grouped by area:
├─ Location 23.81_90.41: 25 farms
├─ Location 23.82_90.42: 30 farms
├─ Location 23.83_90.43: 20 farms
└─ Location 23.84_90.44: 25 farms

API Calls: 4 unique calls/day
Cost: 96% reduction
```

**2. Weather Doesn't Vary That Much Locally**
```
Weather Forecast Resolution:

OpenWeather API Grid: ~10-15 km squares
├─ 23.81, 90.41 → Grid Cell A
├─ 23.8103, 90.4125 → Grid Cell A (same!)
└─ 23.8156, 90.4189 → Grid Cell A (same!)

Reality: API returns identical data for nearby coordinates

Our Rounding: ~1.1 km precision
├─ 0.01° latitude ≈ 1.11 km
└─ 0.01° longitude ≈ 1.11 km (at equator)

Result: Farms within 1 km share forecast (accurate enough)
```

**3. Cache Efficiency**
```
Without Rounding:

Redis Cache:
├─ weather:23.810345_90.412567 → Forecast A
├─ weather:23.810389_90.412601 → Forecast B (99% same as A)
├─ weather:23.810412_90.412634 → Forecast C (99% same as A)
└─ ... (100 nearly identical entries)

Cache Hit Rate: ~5% (each farm unique)
Memory Usage: High (duplicate data)

With Rounding:

Redis Cache:
├─ weather:23.81_90.41 → Forecast A (shared by 25 farms)
├─ weather:23.82_90.42 → Forecast B (shared by 30 farms)
└─ ... (4 entries total)

Cache Hit Rate: ~95% (farms share keys)
Memory Usage: Low (no duplication)

Performance: 20x faster (cache hits vs API calls)
```

**1. Is 1 km Precision Enough?**
```
Weather Variation at Different Scales:

├─ 100 km: Different weather systems
├─ 10 km: Slight variations (hills, water bodies)
├─ 1 km: Essentially identical
└─ 100 m: No measurable difference

Our Rounding: 1.1 km precision

Farm Sizes:
├─ Small farm: 1-5 hectares (100m × 100m)
├─ Medium farm: 10-50 hectares (300m × 300m)
├─ Large farm: 100+ hectares (1km × 1km)

Conclusion: 1 km precision is MORE than enough
```

**2. Real-World Example**
```
Two Farms:
├─ Farm A: 23.8103, 90.4125 (exact)
├─ Farm B: 23.8156, 90.4189 (exact)
└─ Distance: ~650 meters apart

Weather Difference:
├─ Temperature: ±0.1°C (negligible)
├─ Humidity: ±1% (negligible)
├─ Rainfall: Same (unless very localized storm)
└─ Wind: Same direction and speed

Conclusion: Sharing forecast is accurate
```


**2. Growth Handling**
```
System Growth:

100 farms → 10,000 farms

Without Rounding:
- 10,000 unique API calls
- Impossible (rate limits)
- Need expensive API tier

With Rounding:
- ~400 unique location keys (assuming distribution)
- Still under free tier
- Scales naturally

Conclusion: Design supports 100x growth
```

**Future Enhancements:** Add SMS notifications and upport custom notification times per user

### Authentication Domain

**Problem:** Secure user access with token-based authentication

**Architecture:**
```
┌─────────────────────────────────────────────────────────┐
│                  AUTHENTICATION FLOW                     │
└─────────────────────────────────────────────────────────┘

POST /api/v1/auth/register
    │
    ├─▶ Validate Input (email, password strength)
    │
    ├─▶ Hash Password (bcrypt, cost=10)
    │
    ├─▶ Store User in PostgreSQL
    │
    └─▶ Return User ID

POST /api/v1/auth/login
    │
    ├─▶ Fetch User by Email
    │
    ├─▶ Compare Password Hash
    │
    ├─▶ Generate JWT Access Token (15 min expiry)
    │
    ├─▶ Generate Refresh Token (7 days, stored in DB)
    │
    └─▶ Return Both Tokens

POST /api/v1/auth/refresh
    │
    ├─▶ Validate Refresh Token from DB
    │
    ├─▶ Check Expiry & Revocation
    │
    ├─▶ Generate New Access Token
    │
    └─▶ Return New Token
```

**What We Did:**
- Implemented JWT-based authentication with short-lived access tokens
- Stored refresh tokens in PostgreSQL for revocation capability
- Used bcrypt for password hashing
- Created middleware to protect routes

**Why This Way:**
- **JWT for stateless auth:** No session storage needed, scales horizontally
- **Refresh tokens in DB:** Allows logout/revocation (pure JWT can't be revoked)
- **Short access token expiry:** Limits damage if token is stolen
- **Bcrypt over SHA256:** Designed for passwords, has built-in salt, adjustable cost

**Alternative Approaches:**
1. **Session-based auth:** Requires Redis/DB lookup on every request (slower)
2. **OAuth2:** Overkill for this use case, adds complexity
3. **API Keys:** Less secure, no expiration, harder to rotate

**Future Enhancements:**
- Add 2FA (TOTP)
- Implement rate limiting on login attempts
- Add password reset via email
- Support OAuth2 for social login

---

### Farm Management Domain

**Problem:** Users need to manage multiple farms with GPS coordinates

**Architecture:**
```
┌─────────────────────────────────────────────────────────┐
│                    FARM MANAGEMENT                       │
└─────────────────────────────────────────────────────────┘

User (1) ──────── (N) Farm
    │                  │
    │                  ├─ ID (UUID)
    │                  ├─ Name
    │                  ├─ Latitude
    │                  ├─ Longitude
    │                  ├─ Location Key (for weather API)
    │                  └─ Timestamps
    │
    └─────────────────▶ Weather Alerts (N)
                           │
                           ├─ Metric (temp/rain/humidity/wind)
                           ├─ Operator (<, >, =)
                           ├─ Value (threshold)
                           └─ Is Enabled

API Flow:
POST /api/v1/farms
    │
    ├─▶ Extract User ID from JWT
    │
    ├─▶ Validate Coordinates (-90 to 90, -180 to 180)
    │
    ├─▶ Generate Location Key (lat_lon hash)
    │
    ├─▶ Store in PostgreSQL
    │
    └─▶ Return Farm Object
```

**What We Did:**
- Created one-to-many relationship: User → Farms → Weather Alerts
- Used UUIDs for IDs (better for distributed systems)
- Added location_key for efficient weather API caching
- Implemented ownership verification (users can only access their farms)

**Future Enhancements:**
- Add farm boundaries (polygon coordinates)
- Support multiple crops per farm
- Add soil test history tracking
- Implement farm sharing (multiple users per farm)

---

### Why RabbitMQ?

**Requirements:**
- Send emails asynchronously
- Retry failed emails
- Simple pub/sub

### Why Redis for Caching?

**Problem:**
- OpenWeather API: 1000 calls/day free tier
- 100 farms × 24 checks/day = 2400 calls (over limit!)

**Solution:**
- Cache forecasts for 3 hours (weather update frequency)
- Nearby farms share same cache key
- Result: ~8 API calls/day for 100 farms

---

### Prerequisites

```bash
# Required
- Go 1.25+
- Python 3.8+
- PostgreSQL 14+
- Redis 6+
- RabbitMQ 3.9+
```

### Quick Setup

#### 1. Clone & Configure

```bash
git clone https://github.com/yourusername/farm-fusion.git
cd farm-fusion
cp .env.example .env
# Edit .env with your credentials
```

#### 2. Database Setup

```bash
createdb farm_fusion
psql -d farm_fusion -f migrations/*.up.sql
```

#### 3. Start ML Service

```bash
cd ml_service
python3 -m venv venv
source venv/bin/activate
pip install -r requirements.txt
python train_models.py  # First time only
uvicorn app.main:app --host 0.0.0.0 --port 8000
```

#### 4. Start Go Backend

```bash
go mod download
go build -o bin/api cmd/api/main.go
./bin/api
```

#### 5. Start Background Services

```bash
# Terminal 1
go build -o scheduler cmd/scheduler/main.go
./scheduler

# Terminal 2
go build -o worker cmd/worker/main.go
./worker
```
---

### Mistakes & Lessons

**Mistake 1: Over-engineering Early**
- Initially wanted to use gRPC, microservices everywhere
- Learned: Start simple, add complexity when needed

**Mistake 2: Not Planning Database Schema**
- Had to add location_key column later for caching
- Learned: Think about access patterns upfront
- Migrations are painful, get it right first time

**Mistake 3: Ignoring Error Handling**
- Early code had generic error messages
- Learned: Specific errors help debugging
---

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Workflow
1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

## Author

**Md. Soyaib Rahman Zihad**
- GitHub: [Soyaib10](https://github.com/Soyaib10)
- LinkedIn: [Md. Soyaib Rahman](https://www.linkedin.com/in/md-soyaib-rahman-788261194/)
- Email: soyaibzihad10@gmail.com

---


