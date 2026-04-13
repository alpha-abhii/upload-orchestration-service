package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port             string
	AWSRegion        string
	AWSAccessKey     string
	AWSSecretKey     string
	S3Bucket         string
	CloudFrontDomain string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:             getEnvOrDefault("PORT", "8080"),
		AWSRegion:        getEnvOrDefault("AWS_REGION", "us-east-1"),
		AWSAccessKey:     os.Getenv("AWS_ACCESS_KEY_ID"),
		AWSSecretKey:     os.Getenv("AWS_SECRET_ACCESS_KEY"),
		S3Bucket:         os.Getenv("S3_BUCKET"),
		CloudFrontDomain: os.Getenv("CLOUDFRONT_DOMAIN"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) validate() error {
	required := map[string]string{
		"S3_BUCKET":             c.S3Bucket,
	}

	for name, val := range required {
		if val == "" {
			return fmt.Errorf("required env var %q is not set", name)
		}
	}

	return nil
}

func getEnvOrDefault(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}
