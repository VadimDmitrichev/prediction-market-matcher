package similarity

import (
	"context"
	"log/slog"
	"sync"

	"prediction-market-matcher/models"
)

// Job represents a pair of event indices to compare.
type Job struct {
	I int
	J int
}

// Result holds the matched event and market pairs produced by a worker.
type Result struct {
	SimilarEvent  *models.SimilarEvent
	SimilarMarket []models.SimilarMarket
}

// Worker processes event comparison jobs concurrently until the jobs channel is closed or context is canceled.
func Worker(
	ctx context.Context,
	id int,
	jobs <-chan Job,
	results chan<- Result,
	events1, events2 []models.Event,
	calculator *Calculator,
	wg *sync.WaitGroup,
	logger *slog.Logger,
) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case job, ok := <-jobs:
			if !ok {
				return
			}

			event1 := events1[job.I]
			event2 := events2[job.J]

			eventSimilarity := calculator.CalculateLevenshteinSimilarity(
				event1.Title,
				event2.Title,
			)

			if eventSimilarity >= calculator.thresholds.levenshtein {
				logger.Debug("matched similar events",
					"worker_id", id,
					"event1", event1.Title,
					"event2", event2.Title,
					"similarity", eventSimilarity,
				)

				matchedMarkets, similarMarkets := findSimilarMarkets(
					ctx, event1, event2, eventSimilarity, calculator,
				)

				if len(matchedMarkets) > 0 {
					similarEvent := &models.SimilarEvent{
						FirstEvent:           event1,
						SecondEvent:          event2,
						EventSimilarityScore: eventSimilarity,
						MatchedMarkets:       matchedMarkets,
						TotalMarketMatches:   len(matchedMarkets),
					}

					select {
					case <-ctx.Done():
						return
					case results <- Result{
						SimilarEvent:  similarEvent,
						SimilarMarket: similarMarkets,
					}:
					}
				}
			}
		}
	}
}

func findSimilarMarkets(
	ctx context.Context,
	event1, event2 models.Event,
	eventSimilarity int,
	calculator *Calculator,
) ([]models.MatchedMarket, []models.SimilarMarket) {
	var matchedMarkets []models.MatchedMarket
	var similarMarkets []models.SimilarMarket

	marketThreshold := calculator.cfg.MarketSimilarity
	if marketThreshold == 0 {
		marketThreshold = calculator.thresholds.levenshtein
	}

	for _, market1 := range event1.Markets {
		if ctx.Err() != nil {
			return nil, nil
		}

		for _, market2 := range event2.Markets {
			marketSimilarity := calculator.CalculateMarketSimilarity(
				market1.Question,
				market2.Question,
			)

			if marketSimilarity >= marketThreshold {
				matchedMarket := models.MatchedMarket{
					FirstMarket:           market1,
					SecondMarket:          market2,
					MarketSimilarityScore: marketSimilarity,
				}

				similarMarket := models.SimilarMarket{
					FirstMarketEventTitle:  event1.Title,
					SecondMarketEventTitle: event2.Title,
					EventSimilarityScore:   eventSimilarity,
					FirstMarket:            market1,
					SecondMarket:           market2,
					MarketSimilarityScore:  marketSimilarity,
				}

				matchedMarkets = append(matchedMarkets, matchedMarket)
				similarMarkets = append(similarMarkets, similarMarket)
			}
		}
	}

	return matchedMarkets, similarMarkets
}
