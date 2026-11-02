package storage

import (
	"encoding/json"
	"os"
	"prediction-market-matcher/models"
)

func SaveSimilarEvents(events []models.SimilarEvent, filename string) error {
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func SaveSimilarMarkets(markets []models.SimilarMarket, filename string) error {
	data, err := json.MarshalIndent(markets, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}
