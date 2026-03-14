package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const ContextKeyUserID = "userID"

// appNS is the fixed UUID v5 namespace used to derive user_id from Zitadel's sub claim.
var appNS = uuid.MustParse("f47ac10b-58cc-4372-a567-0e02b2c3d479")

// NewMiddleware returns a Gin middleware that validates RS256 JWTs issued by issuerURL.
// If issuerURL is empty, returns a no-op middleware (auth disabled — anonymous fallback in handleWS).
func NewMiddleware(issuerURL string) gin.HandlerFunc {
	if issuerURL == "" {
		log.Println("auth: ZITADEL_ISSUER not set — JWT validation disabled (anonymous mode)")
		return func(c *gin.Context) { c.Next() }
	}

	jwksURL := issuerURL + "/oauth/v2/keys"

	cache := jwk.NewCache(context.Background())

	if err := cache.Register(jwksURL, jwk.WithRefreshInterval(5*time.Minute)); err != nil {
		log.Printf("auth: failed to register JWKS URL %s: %v — JWT validation disabled", jwksURL, err)
		return func(c *gin.Context) { c.Next() }
	}

	// Pre-warm: attempt a first fetch, but only warn on failure (Zitadel may still be starting)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := cache.Refresh(ctx, jwksURL); err != nil {
		log.Printf("auth: JWKS pre-warm failed (will retry on first request): %v", err)
	} else {
		log.Printf("auth: JWKS middleware active, issuer=%s", issuerURL)
	}

	return func(c *gin.Context) {
		tokenStr := c.Query("token")
		if tokenStr == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing token"})
			return
		}

		keySet, err := cache.Get(c.Request.Context(), jwksURL)
		if err != nil {
			log.Printf("auth: JWKS fetch error: %v", err)
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "auth service unavailable"})
			return
		}

		parsed, err := jwt.Parse([]byte(tokenStr),
			jwt.WithKeySet(keySet),
			jwt.WithValidate(true),
			jwt.WithIssuer(issuerURL),
		)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": fmt.Sprintf("invalid token: %v", err)})
			return
		}

		userID := uuid.NewSHA1(appNS, []byte(parsed.Subject()))
		c.Set(ContextKeyUserID, userID)
		c.Next()
	}
}

// UserIDFromContext retrieves the UUID set by the auth middleware.
// Returns (uuid, true) when authenticated, (zero, false) when auth is disabled.
func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	val, exists := c.Get(ContextKeyUserID)
	if !exists {
		return uuid.UUID{}, false
	}
	id, ok := val.(uuid.UUID)
	return id, ok
}
