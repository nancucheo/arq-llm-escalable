package main

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/jnancucheo/arq-llm-escalable/cronjobs/internal/redisclient"
)

type report struct {
	Timestamp            string `json:"timestamp"`
	QueueDepth           int64  `json:"queue_depth"`
	ActiveResultChannels int    `json:"active_result_channels"`
	UsedMemoryHuman      string `json:"used_memory_human"`
	UsedMemoryRSSHuman   string `json:"used_memory_rss_human"`
	UsedMemoryPeakHuman  string `json:"used_memory_peak_human"`
	ConnectedClients     string `json:"connected_clients"`
}

func main() {
	ctx := context.Background()

	rdb, err := redisclient.New(ctx)
	if err != nil {
		log.Fatalf("connect redis: %v", err)
	}
	defer rdb.Close()

	queueDepth, err := rdb.LLen(ctx, "llm_queue").Result()
	if err != nil {
		log.Fatalf("LLEN llm_queue: %v", err)
	}

	var (
		cursor               uint64
		activeResultChannels int
	)
	for {
		keys, nextCursor, err := rdb.Scan(ctx, cursor, "result:*", 100).Result()
		if err != nil {
			log.Fatalf("SCAN: %v", err)
		}
		activeResultChannels += len(keys)
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	infoStr, err := rdb.Info(ctx, "memory", "clients").Result()
	if err != nil {
		log.Fatalf("INFO: %v", err)
	}

	fields := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(infoStr))
	for scanner.Scan() {
		line := scanner.Text()
		if idx := strings.Index(line, ":"); idx != -1 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			fields[key] = val
		}
	}

	r := report{
		Timestamp:            time.Now().UTC().Format(time.RFC3339),
		QueueDepth:           queueDepth,
		ActiveResultChannels: activeResultChannels,
		UsedMemoryHuman:      fields["used_memory_human"],
		UsedMemoryRSSHuman:   fields["used_memory_rss_human"],
		UsedMemoryPeakHuman:  fields["used_memory_peak_human"],
		ConnectedClients:     fields["connected_clients"],
	}

	b, _ := json.Marshal(r)
	log.Printf("%s", b)
}
