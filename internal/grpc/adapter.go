package grpc

import (
	"encoding/json"
	"fmt"
	"time"

	"prediction-market-matcher/models"
	"prediction-market-matcher/proto/gen/marketsimilarity"
)

// Converter для преобразования между proto и models
type Converter struct{}

// ConvertProtoToModels конвертирует protobuf структуры в models.Event
func (c *Converter) ConvertProtoToModels(groups []*marketsimilarity.MarketGroup) []models.Event {
	var events []models.Event

	for _, group := range groups {
		event := models.Event{
			Title: group.GetTitle(),
		}

		var markets []models.Market
		for _, market := range group.GetMarkets() {
			m := models.Market{
				Question: market.GetQuestion(),
				YES:      market.GetYesPrice(),
				NO:       market.GetNoPrice(),
			}

			// Используем c.convertDateStringToRawMessage для правильной конвертации дат
			m.StartDate = c.convertDateStringToRawMessage(market.GetStartDate())
			m.EndDate = c.convertDateStringToRawMessage(market.GetEndDate())

			markets = append(markets, m)
		}

		event.Markets = markets
		events = append(events, event)
	}

	return events
}

// convertDateStringToRawMessage конвертирует строку даты в json.RawMessage
func (c *Converter) convertDateStringToRawMessage(dateStr string) json.RawMessage {
	if dateStr == "" {
		// Возвращаем null если строка пустая
		return json.RawMessage([]byte("null"))
	}

	// Проверяем, валидна ли дата в формате RFC3339
	if _, err := time.Parse(time.RFC3339, dateStr); err != nil {
		// Если невалидна, пробуем другие форматы
		formats := []string{
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.999Z",
			"2006-01-02T15:04:05.999999Z",
			"2006-01-02T15:04:05.999999999Z",
		}

		valid := false
		for _, format := range formats {
			if _, err := time.Parse(format, dateStr); err == nil {
				valid = true
				break
			}
		}

		if !valid {
			// Если дата вообще невалидна, возвращаем null
			return json.RawMessage([]byte("null"))
		}
	}

	// Возвращаем дату как JSON строку
	data, _ := json.Marshal(dateStr)
	return json.RawMessage(data)
}

// ConvertSimilarMarketsToProto конвертирует models.SimilarMarket в protobuf
func (c *Converter) ConvertSimilarMarketsToProto(similarMarkets []models.SimilarMarket) []*marketsimilarity.MarketComparison {
	var comparisons []*marketsimilarity.MarketComparison

	for _, sm := range similarMarkets {
		comparison := &marketsimilarity.MarketComparison{
			FirstMarketEventTitle:  sm.FirstMarketEventTitle,
			SecondMarketEventTitle: sm.SecondMarketEventTitle,
			EventSimilarityScore:   int32(sm.EventSimilarityScore),
			MarketSimilarityScore:  int32(sm.MarketSimilarityScore),
		}

		// Конвертируем FirstMarket
		firstMarketProto := c.ConvertMarketToProto(sm.FirstMarket)
		if firstMarketProto != nil {
			comparison.FirstMarket = firstMarketProto
		} else {
			// Создаем пустой Market, если конвертация не удалась
			comparison.FirstMarket = &marketsimilarity.Market{}
		}

		// Конвертируем SecondMarket
		secondMarketProto := c.ConvertMarketToProto(sm.SecondMarket)
		if secondMarketProto != nil {
			comparison.SecondMarket = secondMarketProto
		} else {
			comparison.SecondMarket = &marketsimilarity.Market{}
		}

		comparisons = append(comparisons, comparison)
	}

	return comparisons
}

// ConvertMarketToProto конвертирует models.Market в protobuf Market
func (c *Converter) ConvertMarketToProto(m models.Market) *marketsimilarity.Market {
	if m.Question == "" {
		// Если маркет пустой, возвращаем nil
		return nil
	}

	market := &marketsimilarity.Market{
		Question: m.Question,
		YesPrice: m.YES,
		NoPrice:  m.NO,
	}

	// Извлекаем даты из RawMessage
	if startDate, ok := m.GetStartDate(); ok {
		market.StartDate = startDate.Format(time.RFC3339Nano)
	} else {
		// Если дата невалидна, устанавливаем пустую строку
		market.StartDate = ""
	}

	if endDate, ok := m.GetEndDate(); ok {
		market.EndDate = endDate.Format(time.RFC3339Nano)
	} else {
		market.EndDate = ""
	}

	return market
}

// ConvertSimilarEventsToProto конвертирует models.SimilarEvent в protobuf MarketComparison
// (дополнительная функция, если нужна)
func (c *Converter) ConvertSimilarEventsToProto(similarEvents []models.SimilarEvent) []*marketsimilarity.MarketComparison {
	var comparisons []*marketsimilarity.MarketComparison

	for _, se := range similarEvents {
		// Для каждого MatchedMarket создаем отдельное MarketComparison
		for _, mm := range se.MatchedMarkets {
			comparison := &marketsimilarity.MarketComparison{
				FirstMarketEventTitle:  se.FirstEvent.Title,
				SecondMarketEventTitle: se.SecondEvent.Title,
				EventSimilarityScore:   int32(se.EventSimilarityScore),
				MarketSimilarityScore:  int32(mm.MarketSimilarityScore),
			}

			// Конвертируем маркеты
			firstMarketProto := c.ConvertMarketToProto(mm.FirstMarket)
			if firstMarketProto != nil {
				comparison.FirstMarket = firstMarketProto
			}

			secondMarketProto := c.ConvertMarketToProto(mm.SecondMarket)
			if secondMarketProto != nil {
				comparison.SecondMarket = secondMarketProto
			}

			comparisons = append(comparisons, comparison)
		}
	}

	return comparisons
}

// PrintEventsForDebug отладочная функция для проверки данных
func (c *Converter) PrintEventsForDebug(events []models.Event) {
	for i, event := range events {
		fmt.Printf("Event %d: %s\n", i, event.Title)
		for j, market := range event.Markets {
			fmt.Printf("  Market %d:\n", j)
			fmt.Printf("    Question: %s\n", market.Question)
			fmt.Printf("    YES: %.4f, NO: %.4f\n", market.YES, market.NO)

			// Конвертируем RawMessage в строку для вывода
			startDateStr := "null"
			if len(market.StartDate) > 0 && string(market.StartDate) != "null" {
				startDateStr = string(market.StartDate)
			}

			endDateStr := "null"
			if len(market.EndDate) > 0 && string(market.EndDate) != "null" {
				endDateStr = string(market.EndDate)
			}

			fmt.Printf("    StartDate Raw: %s\n", startDateStr)
			fmt.Printf("    EndDate Raw: %s\n", endDateStr)

			// Пытаемся распарсить
			if startDate, ok := market.GetStartDate(); ok {
				fmt.Printf("    StartDate Parsed: %s\n", startDate.Format(time.RFC3339Nano))
			} else {
				fmt.Printf("    StartDate Parsed: INVALID\n")
			}

			if endDate, ok := market.GetEndDate(); ok {
				fmt.Printf("    EndDate Parsed: %s\n", endDate.Format(time.RFC3339Nano))
			} else {
				fmt.Printf("    EndDate Parsed: INVALID\n")
			}
		}
	}
}

// EventsToJSON конвертирует события в JSON для проверки
func (c *Converter) EventsToJSON(events []models.Event) (string, error) {
	// Создаем временную структуру для правильной сериализации
	type TempMarket struct {
		Question  string      `json:"question"`
		StartDate interface{} `json:"startDate"`
		EndDate   interface{} `json:"endDate"`
		YES       float64     `json:"YES"`
		NO        float64     `json:"NO"`
	}

	type TempEvent struct {
		Title   string       `json:"title"`
		Markets []TempMarket `json:"markets"`
	}

	var tempEvents []TempEvent

	for _, event := range events {
		tempEvent := TempEvent{
			Title: event.Title,
		}

		for _, market := range event.Markets {
			tempMarket := TempMarket{
				Question: market.Question,
				YES:      market.YES,
				NO:       market.NO,
			}

			// Обрабатываем StartDate
			var startDateStr interface{}
			if len(market.StartDate) > 0 && string(market.StartDate) != "null" {
				var str string
				if err := json.Unmarshal(market.StartDate, &str); err == nil {
					startDateStr = str
				} else {
					startDateStr = string(market.StartDate)
				}
			} else {
				startDateStr = nil
			}
			tempMarket.StartDate = startDateStr

			// Обрабатываем EndDate аналогично
			var endDateStr interface{}
			if len(market.EndDate) > 0 && string(market.EndDate) != "null" {
				var str string
				if err := json.Unmarshal(market.EndDate, &str); err == nil {
					endDateStr = str
				} else {
					endDateStr = string(market.EndDate)
				}
			} else {
				endDateStr = nil
			}
			tempMarket.EndDate = endDateStr

			tempEvent.Markets = append(tempEvent.Markets, tempMarket)
		}

		tempEvents = append(tempEvents, tempEvent)
	}

	data, err := json.MarshalIndent(tempEvents, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
