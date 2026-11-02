package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/viper"
)

type LogConfig struct {
	Level  string `mapstructure:"level"`  // debug, info, warn, error
	Format string `mapstructure:"format"` // text, json
	Output string `mapstructure:"output"` // stdout, file, both
	File   string `mapstructure:"file"`   // log file path when output=file or both
}

type Config struct {
	LevenshteinSimilarity int       `mapstructure:"levenshtein_similarity"`
	MarketSimilarity      int       `mapstructure:"market_similarity"`
	InputFile1            string    `mapstructure:"input_file_1"`
	InputFile2            string    `mapstructure:"input_file_2"`
	OutputEventsFile      string    `mapstructure:"output_events_file"`
	OutputMarketsFile     string    `mapstructure:"output_markets_file"`
	WorkerCount           int       `mapstructure:"worker_count"`
	GRPCPort              string    `mapstructure:"grpc_port"`
	Log                   LogConfig `mapstructure:"log"`
	TwoPassEnabled        bool      `mapstructure:"two_pass_enabled"`
	DebugMode             bool      `mapstructure:"debug_mode"`

	SimilarityWeights struct {
		Levenshtein    float64 `mapstructure:"levenshtein" yaml:"levenshtein"`
		Jaccard        float64 `mapstructure:"jaccard" yaml:"jaccard"`
		Dice           float64 `mapstructure:"dice" yaml:"dice"`
		LCS            float64 `mapstructure:"lcs" yaml:"lcs"`
		KeywordOverlap float64 `mapstructure:"keyword_overlap" yaml:"keyword_overlap"`
		Capitalized    float64 `mapstructure:"capitalized" yaml:"capitalized"`
	} `mapstructure:"similarity_weights" yaml:"similarity_weights"`

	SimilarityThresholds struct {
		LevenshteinMin         int     `mapstructure:"levenshtein_min" yaml:"levenshtein_min"`
		JaccardMin             float64 `mapstructure:"jaccard_min" yaml:"jaccard_min"`
		DiceMin                float64 `mapstructure:"dice_min" yaml:"dice_min"`
		LCSMin                 float64 `mapstructure:"lcs_min" yaml:"lcs_min"`
		KeywordOverlapMin      float64 `mapstructure:"keyword_overlap_min" yaml:"keyword_overlap_min"`
		CapitalizedMin         float64 `mapstructure:"capitalized_min" yaml:"capitalized_min"`
		FinalScoreMin          int     `mapstructure:"final_score_min" yaml:"final_score_min"`
		CheckRatioMin          float64 `mapstructure:"check_ratio_min" yaml:"check_ratio_min"`
		HighLevWithGoodJaccard int     `mapstructure:"high_lev_with_good_jaccard" yaml:"high_lev_with_good_jaccard"`
	} `mapstructure:"similarity_thresholds" yaml:"similarity_thresholds"`
}

var globalConfig *Config

// LoadConfig reads configuration from file and environment variables.
func LoadConfig(configPath ...string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if len(configPath) > 0 && configPath[0] != "" {
		v.SetConfigFile(configPath[0])
		if err := v.ReadInConfig(); err != nil {
			slog.Warn("failed to read specified config file, falling back to defaults/env", "path", configPath[0], "error", err)
		} else {
			slog.Info("configuration loaded from file", "path", v.ConfigFileUsed())
		}
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./config")
		v.AddConfigPath("/etc/events-matcher/")

		if err := v.ReadInConfig(); err == nil {
			slog.Info("configuration loaded", "path", v.ConfigFileUsed())
		}
	}

	v.AutomaticEnv()
	v.SetEnvPrefix("EVENTS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}

	globalConfig = &cfg
	return globalConfig, nil
}

// GetConfig returns the global configuration instance.
func GetConfig() *Config {
	if globalConfig == nil {
		cfg, err := LoadConfig()
		if err != nil {
			panic(fmt.Sprintf("failed to load default configuration: %v", err))
		}
		return cfg
	}
	return globalConfig
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("levenshtein_similarity", 80)
	v.SetDefault("market_similarity", 70)
	v.SetDefault("input_file_1", "events1.json")
	v.SetDefault("input_file_2", "events2.json")
	v.SetDefault("output_events_file", "similar_events.json")
	v.SetDefault("output_markets_file", "similar_markets.json")
	v.SetDefault("worker_count", 20)
	v.SetDefault("grpc_port", ":50051")
	v.SetDefault("two_pass_enabled", true)
	v.SetDefault("debug_mode", false)

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.output", "stdout")
	v.SetDefault("log.file", "logs/app.log")

	v.SetDefault("similarity_weights.levenshtein", 0.15)
	v.SetDefault("similarity_weights.jaccard", 0.20)
	v.SetDefault("similarity_weights.dice", 0.20)
	v.SetDefault("similarity_weights.lcs", 0.20)
	v.SetDefault("similarity_weights.keyword_overlap", 0.15)
	v.SetDefault("similarity_weights.capitalized", 0.10)

	v.SetDefault("similarity_thresholds.levenshtein_min", 60)
	v.SetDefault("similarity_thresholds.jaccard_min", 0.4)
	v.SetDefault("similarity_thresholds.dice_min", 0.4)
	v.SetDefault("similarity_thresholds.lcs_min", 0.4)
	v.SetDefault("similarity_thresholds.keyword_overlap_min", 0.3)
	v.SetDefault("similarity_thresholds.capitalized_min", 0.3)
	v.SetDefault("similarity_thresholds.final_score_min", 65)
	v.SetDefault("similarity_thresholds.check_ratio_min", 0.6)
	v.SetDefault("similarity_thresholds.high_lev_with_good_jaccard", 85)
}

func validateConfig(cfg *Config) error {
	if cfg.LevenshteinSimilarity < 0 || cfg.LevenshteinSimilarity > 100 {
		return fmt.Errorf("invalid levenshtein_similarity threshold: %d (must be 0-100)", cfg.LevenshteinSimilarity)
	}
	if cfg.MarketSimilarity < 0 || cfg.MarketSimilarity > 100 {
		return fmt.Errorf("invalid market_similarity threshold: %d (must be 0-100)", cfg.MarketSimilarity)
	}
	if cfg.InputFile1 == "" || cfg.InputFile2 == "" {
		return fmt.Errorf("input file paths cannot be empty")
	}
	if cfg.WorkerCount <= 0 {
		return fmt.Errorf("worker_count must be greater than 0")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[strings.ToLower(cfg.Log.Level)] {
		return fmt.Errorf("invalid log level: %s", cfg.Log.Level)
	}

	validLogFormats := map[string]bool{"text": true, "json": true}
	if !validLogFormats[strings.ToLower(cfg.Log.Format)] {
		return fmt.Errorf("invalid log format: %s", cfg.Log.Format)
	}

	validLogOutputs := map[string]bool{"stdout": true, "file": true, "both": true}
	if !validLogOutputs[strings.ToLower(cfg.Log.Output)] {
		return fmt.Errorf("invalid log output: %s", cfg.Log.Output)
	}

	if (strings.ToLower(cfg.Log.Output) == "file" || strings.ToLower(cfg.Log.Output) == "both") && cfg.Log.File == "" {
		return fmt.Errorf("log file path cannot be empty when output is set to 'file' or 'both'")
	}

	return nil
}
