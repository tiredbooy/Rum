package middlewares

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tiredbooy/Rum/backend/internal/pkg/api/dto"
	"golang.org/x/time/rate"
)

// Sane defaults for the per-IP token-bucket limiter. These are generous enough
// for a desktop download manager's UI (polling + SSE + bursty user actions) yet
// still cap abusive clients.
const (
	DefaultRateLimitRPS   = 20
	DefaultRateLimitBurst = 40

	// idleEvictAfter is how long a per-IP limiter may sit unused before it is
	// evicted, keeping the map bounded so the limiter is leak-free.
	idleEvictAfter = 10 * time.Minute
	// evictInterval is how often the background sweep runs.
	evictInterval = 5 * time.Minute
)

// RateLimitConfig configures the token-bucket rate limiter.
type RateLimitConfig struct {
	// RPS is the sustained requests-per-second allowed per client.
	RPS float64
	// Burst is the maximum burst size (bucket capacity).
	Burst int
}

// ipLimiter pairs a token bucket with a last-seen timestamp for idle eviction.
type ipLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// rateLimiterStore is a leak-free, concurrency-safe registry of per-IP token
// buckets. Idle entries are swept periodically so the map cannot grow without
// bound under churn (e.g. many distinct client IPs).
type rateLimiterStore struct {
	mu      sync.Mutex
	clients map[string]*ipLimiter
	rps     rate.Limit
	burst   int

	// stop guards a single shutdown of the sweeper goroutine (test hygiene).
	stop     chan struct{}
	stopOnce sync.Once
}

func newRateLimiterStore(cfg RateLimitConfig) *rateLimiterStore {
	if cfg.RPS <= 0 {
		cfg.RPS = DefaultRateLimitRPS
	}
	if cfg.Burst <= 0 {
		cfg.Burst = DefaultRateLimitBurst
	}
	s := &rateLimiterStore{
		clients: make(map[string]*ipLimiter),
		rps:     rate.Limit(cfg.RPS),
		burst:   cfg.Burst,
		stop:    make(chan struct{}),
	}
	go s.sweep()
	return s
}

// get returns the limiter for key, creating it on first use and refreshing its
// last-seen timestamp.
func (s *rateLimiterStore) get(key string) *rate.Limiter {
	s.mu.Lock()
	defer s.mu.Unlock()

	cl, ok := s.clients[key]
	if !ok {
		cl = &ipLimiter{limiter: rate.NewLimiter(s.rps, s.burst)}
		s.clients[key] = cl
	}
	cl.lastSeen = time.Now()
	return cl.limiter
}

// sweep evicts limiters that have been idle longer than idleEvictAfter so the
// store stays bounded. It exits when stop is closed.
func (s *rateLimiterStore) sweep() {
	ticker := time.NewTicker(evictInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			cutoff := time.Now().Add(-idleEvictAfter)
			s.mu.Lock()
			for key, cl := range s.clients {
				if cl.lastSeen.Before(cutoff) {
					delete(s.clients, key)
				}
			}
			s.mu.Unlock()
		}
	}
}

// Close stops the background sweeper. Optional; intended for tests/shutdown.
func (s *rateLimiterStore) Close() {
	s.stopOnce.Do(func() { close(s.stop) })
}

// RateLimit returns a per-IP token-bucket limiting middleware. A nil/zero cfg
// falls back to sane defaults. On rejection it returns 429 with the standard
// error envelope and a Retry-After header so well-behaved clients back off.
func RateLimit(cfg RateLimitConfig) gin.HandlerFunc {
	store := newRateLimiterStore(cfg)

	// Retry-After communicates roughly how long until a token is available.
	retryAfter := "1"
	if store.rps > 0 {
		if secs := int(1.0/float64(store.rps) + 0.999); secs >= 1 {
			retryAfter = strconv.Itoa(secs)
		}
	}

	return func(c *gin.Context) {
		if !store.get(c.ClientIP()).Allow() {
			c.Header("Retry-After", retryAfter)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, dto.ErrorResponse{
				Error: "too many requests",
				Code:  dto.CodeRateLimited,
			})
			return
		}
		c.Next()
	}
}
