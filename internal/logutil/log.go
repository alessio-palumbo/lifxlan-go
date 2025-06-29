package logutil

import (
	"os"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

var once sync.Once

func Init() {
	once.Do(func() {
		levelStr := strings.ToLower(os.Getenv("LIFXLAN_LOG_LEVEL"))
		level, err := log.ParseLevel(levelStr)
		if err != nil {
			level = log.InfoLevel
		}

		log.SetLevel(level)
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp: true,
		})
	})
}
