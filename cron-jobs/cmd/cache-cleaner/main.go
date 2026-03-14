package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/jnancucheo/arq-llm-escalable/cronjobs/internal/redisclient"
)

func main() {
	ctx := context.Background()

	rdb, err := redisclient.New(ctx)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rdb.Close()

	staleTTL := 2 * time.Hour
	if v := os.Getenv("STALE_TTL_HOURS"); v != "" {
		if h, err := strconv.ParseFloat(v, 64); err == nil {
			staleTTL = time.Duration(h * float64(time.Hour))
		}
	}

	queueDepth, err := rdb.LLen(ctx, "llm_queue").Result()
	if err != nil {
		log.Fatalf("LLEN llm_queue: %v", err)
	}
	log.Printf("queue_depth=%d", queueDepth)

	var (
		cursor  uint64
		scanned int
		deleted int
	)

	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "result:*", 100).Result()
		if err != nil {
			log.Fatalf("SCAN: %v", err)
		}
		scanned += len(keys)

		for _, key := range keys {
			idleSeconds, err := rdb.ObjectIdleTime(ctx, key).Result()
			if err != nil {
				// Key may have expired between SCAN and OBJECT IDLETIME — skip.
				continue
			}
			if idleSeconds >= staleTTL {
				if delErr := rdb.Del(ctx, key).Err(); delErr == nil {
					deleted++
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	log.Printf("scanned=%d deleted=%d queue_depth=%d", scanned, deleted, queueDepth)
}
