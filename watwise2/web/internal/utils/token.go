package utils

import (
	"fmt"
	"time"
)

func GenerateSimpleToken(username string) string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("%s_%d", username, timestamp)
}
