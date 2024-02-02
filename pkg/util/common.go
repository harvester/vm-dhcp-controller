package util

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	ExcludedMark = "EXCLUDED"

	AgentSuffixName = "agent"
)

func agentConcatName(name ...string) string {
	return strings.Join(append(name, AgentSuffixName), "-")
}

func SafeAgentConcatName(name ...string) string {
	fullPath := strings.Join(name, "-")

	if len(fullPath) < 58 {
		return agentConcatName(fullPath)
	}

	digest := sha256.Sum256([]byte(fullPath))
	// since we cut the string in the middle, the last char may not be compatible with what is expected in k8s
	// we are checking and if necessary removing the last char
	c := fullPath[50]
	if 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return agentConcatName(fullPath[0:51], hex.EncodeToString(digest[0:])[0:5])
	}

	return agentConcatName(fullPath[0:50], hex.EncodeToString(digest[0:])[0:6])
}
