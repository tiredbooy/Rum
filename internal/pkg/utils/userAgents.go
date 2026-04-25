package utils

import (
	"hash/fnv"
	"math/rand"
)

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:109.0) Gecko/20100101 Firefox/119.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",
}

func GetRandomUserAgent() string {
	return userAgents[rand.Intn(len(userAgents))]
}

func GetUserAgentForQueue(queueID string) string {
	h := fnv.New32a()
	h.Write([]byte(queueID))
	idx := h.Sum32() / uint32(len(userAgents))
	return userAgents[idx]
}
