package relayer

import "time"

type Config struct {
	Remote            string        `json:"remote"`
	BlockBatchSize    int           `json:"batch_size"`
	BlockBatchTimeout time.Duration `json:"batch_timeout"`
}

func DefaultConfig() Config {
	return Config{
		Remote:            "tcp://localhost:26657",
		BlockBatchSize:    5,
		BlockBatchTimeout: time.Minute,
	}
}
