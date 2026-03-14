package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jnancucheo/arq-llm-escalable/gateway/internal/auth"
	"github.com/jnancucheo/arq-llm-escalable/gateway/internal/repository"
	"github.com/redis/go-redis/v9"
)

var (
	ctx      = context.Background()
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

type PromptRequest struct {
	Prompt string `json:"prompt"`
}

type JobPayload struct {
	JobID    string `json:"job_id"`
	ClientID string `json:"client_id"`
	Prompt   string `json:"prompt"`
}

func newRedisClient() *redis.Client {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	return redis.NewClient(&redis.Options{Addr: addr})
}

func handleWS(rdb *redis.Client, repo repository.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("upgrade error: %v", err)
			return
		}
		defer conn.Close()

		userID, authenticated := auth.UserIDFromContext(c)
		if !authenticated {
			userID = uuid.New() // fallback: anonymous mode (auth disabled)
		}
		convID := uuid.New()
		clientID := userID.String()
		log.Printf("client connected: %s", clientID)

		// Persist user and conversation if repo is available
		if repo != nil {
			if err := repo.EnsureUser(ctx, userID); err != nil {
				log.Printf("EnsureUser error: %v", err)
			}
			if err := repo.CreateConversation(ctx, userID, convID); err != nil {
				log.Printf("CreateConversation error: %v", err)
			}
		}

		// Subscribe to the client's personal Redis channel
		pubsub := rdb.Subscribe(ctx, "result:"+clientID)
		defer pubsub.Close()

		resultCh := pubsub.Channel()

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("client %s disconnected: %v", clientID, err)
				return
			}

			var req PromptRequest
			if err := json.Unmarshal(msg, &req); err != nil || req.Prompt == "" {
				_ = conn.WriteJSON(gin.H{"error": "invalid request"})
				continue
			}

			// Persist user message
			if repo != nil {
				if err := repo.SaveMessage(ctx, repository.Message{
					UserID:  userID,
					ConvID:  convID,
					MsgID:   uuid.New(),
					Role:    "user",
					Content: req.Prompt,
				}); err != nil {
					log.Printf("SaveMessage(user) error: %v", err)
				}
			}

			jobID := uuid.New().String()
			payload := JobPayload{
				JobID:    jobID,
				ClientID: clientID,
				Prompt:   req.Prompt,
			}
			data, _ := json.Marshal(payload)

			log.Printf("client connected: %s, jobID: %s, prompt: %s", clientID, jobID, req.Prompt)
			log.Printf("payload: %s", string(data))

			// Push job to Redis queue
			if err := rdb.RPush(ctx, "llm_queue", data).Err(); err != nil {
				log.Printf("redis push error: %v", err)
				_ = conn.WriteJSON(gin.H{"error": "queue unavailable"})
				continue
			}

			log.Printf("job queued: %s for client: %s", jobID, clientID)

			// Wait for result with timeout
			select {
			case redisMsg := <-resultCh:
				// Persist assistant message
				if repo != nil {
					if err := repo.SaveMessage(ctx, repository.Message{
						UserID:  userID,
						ConvID:  convID,
						MsgID:   uuid.New(),
						Role:    "assistant",
						Content: redisMsg.Payload,
					}); err != nil {
						log.Printf("SaveMessage(assistant) error: %v", err)
					}
				}
				_ = conn.WriteMessage(websocket.TextMessage, []byte(redisMsg.Payload))
			case <-time.After(60 * time.Second):
				_ = conn.WriteJSON(gin.H{"error": "timeout waiting for LLM response"})
			}
		}
	}
}

func main() {
	rdb := newRedisClient()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("cannot connect to Redis: %v", err)
	}
	log.Println("connected to Redis")

	issuer := os.Getenv("ZITADEL_ISSUER")
	authMiddleware := auth.NewMiddleware(issuer)

	// Optional PostgreSQL persistence — gracefully disabled if DATABASE_URL is absent
	var repo repository.Repository
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		pgRepo, err := repository.New(ctx, dbURL)
		if err != nil {
			log.Printf("WARNING: cannot connect to PostgreSQL, running without persistence: %v", err)
		} else {
			repo = pgRepo
			defer pgRepo.Close()
			log.Println("connected to PostgreSQL")
		}
	} else {
		log.Println("DATABASE_URL not set — running without persistence")
	}

	r := gin.Default()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
	r.GET("/ws", authMiddleware, handleWS(rdb, repo))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("gateway listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
