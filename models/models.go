package models

import (
	"encoding/json"
	"time"
)

type Event struct {
	Title   string   `json:"title"`
	Markets []Market `json:"markets"`
}

type Market struct {
	Question  string          `json:"question"`
	StartDate json.RawMessage `json:"startDate"` // Сырой JSON для гибкой обработки
	EndDate   json.RawMessage `json:"endDate"`   // Сырой JSON для гибкой обработки
	YES       float64         `json:"YES"`
	NO        float64         `json:"NO"`
}

// GetStartDate возвращает time.Time если дата валидна
func (m *Market) GetStartDate() (time.Time, bool) {
	return parseDate(m.StartDate)
}

// GetEndDate возвращает time.Time если дата валидна
func (m *Market) GetEndDate() (time.Time, bool) {
	return parseDate(m.EndDate)
}

func parseDate(data json.RawMessage) (time.Time, bool) {
	if len(data) == 0 {
		return time.Time{}, false
	}

	var dateStr string
	if err := json.Unmarshal(data, &dateStr); err != nil {
		return time.Time{}, false
	}

	if dateStr == "" || dateStr == "null" {
		return time.Time{}, false
	}

	t, err := time.Parse(time.RFC3339, dateStr)
	if err != nil {
		// Пробуем другие форматы
		formats := []string{
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.999Z",
			"2006-01-02T15:04:05.999999Z",
		}

		for _, format := range formats {
			if t, err := time.Parse(format, dateStr); err == nil {
				return t, true
			}
		}
		return time.Time{}, false
	}

	return t, true
}

type SimilarEvent struct {
	FirstEvent           Event           `json:"first_event"`
	SecondEvent          Event           `json:"second_event"`
	EventSimilarityScore int             `json:"event_similarity_score"`
	MatchedMarkets       []MatchedMarket `json:"matched_markets"`
	TotalMarketMatches   int             `json:"total_market_matches"`
}

type MatchedMarket struct {
	FirstMarket           Market `json:"first_market"`
	SecondMarket          Market `json:"second_market"`
	MarketSimilarityScore int    `json:"market_similarity_score"`
}

type SimilarMarket struct {
	FirstMarketEventTitle  string `json:"first_market_event_title"`
	SecondMarketEventTitle string `json:"second_market_event_title"`
	EventSimilarityScore   int    `json:"event_similarity_score"`
	FirstMarket            Market `json:"first_market"`
	SecondMarket           Market `json:"second_market"`
	MarketSimilarityScore  int    `json:"market_similarity_score"`
}

type SimilarityMarketResult struct {
	Similar      bool                   `json:"similar"`
	FinalScore   int                    `json:"final_score"`   // Итоговый процент схожести (0-100)
	Levenshtein  int                    `json:"levenshtein"`   // Процент Левенштейна
	Jaccard      float64                `json:"jaccard"`       // Jaccard коэффициент (0-1)
	Dice         float64                `json:"dice"`          // Dice коэффициент (0-1)
	LCS          float64                `json:"lcs"`           // LCS коэффициент (0-1)
	AverageScore float64                `json:"average_score"` // Среднее метрик
	Reason       string                 `json:"reason"`        // Причина решения
	Details      map[string]interface{} `json:"details"`       // Детали
}
