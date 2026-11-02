package grpc

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"prediction-market-matcher/config"
	"prediction-market-matcher/models"
	pb "prediction-market-matcher/proto/gen/marketsimilarity"
	"prediction-market-matcher/similarity"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MarketHandler implements the gRPC MarketComparisonServiceServer.
type MarketHandler struct {
	pb.UnimplementedMarketComparisonServiceServer
	cfg        *config.Config
	logger     *slog.Logger
	conv       *Converter
	calculator *similarity.Calculator
}

// NewMarketHandler creates a new gRPC handler instance.
func NewMarketHandler(cfg *config.Config, logger *slog.Logger) *MarketHandler {
	return &MarketHandler{
		cfg:        cfg,
		logger:     logger,
		conv:       &Converter{},
		calculator: similarity.NewCalculator(cfg.LevenshteinSimilarity, cfg),
	}
}

// CompareMarkets processes two sets of market groups and returns matched pairs.
func (h *MarketHandler) CompareMarkets(
	ctx context.Context,
	req *pb.CompareMarketsRequest,
) (*pb.CompareMarketsResponse, error) {
	start := time.Now()

	h.logger.Info("received CompareMarkets gRPC request",
		"first_groups_count", len(req.GetFirstMarkets()),
		"second_groups_count", len(req.GetSecondMarkets()),
	)

	events1 := h.conv.ConvertProtoToModels(req.GetFirstMarkets())
	events2 := h.conv.ConvertProtoToModels(req.GetSecondMarkets())

	_, similarMarkets, err := h.processEventsParallel(
		ctx,
		events1,
		events2,
		h.calculator,
		h.cfg.WorkerCount,
	)
	if err != nil {
		h.logger.Warn("request processing canceled", "error", err, "duration", time.Since(start))
		return nil, status.Errorf(codes.Canceled, "request processing canceled: %v", err)
	}

	comparisons := h.conv.ConvertSimilarMarketsToProto(similarMarkets)

	h.logger.Info("request processed successfully",
		"matches_found", len(comparisons),
		"duration", time.Since(start),
	)

	return &pb.CompareMarketsResponse{
		Success:     true,
		Message:     fmt.Sprintf("Found %d market comparisons", len(comparisons)),
		Comparisons: comparisons,
	}, nil
}

func (h *MarketHandler) processEventsParallel(
	ctx context.Context,
	events1, events2 []models.Event,
	calculator *similarity.Calculator,
	workerCount int,
) ([]models.SimilarEvent, []models.SimilarMarket, error) {
	if len(events1) == 0 || len(events2) == 0 {
		return nil, nil, nil
	}

	var wg sync.WaitGroup

	// Bounded channel buffers prevent OOM on large Cartesian products (N * M)
	bufferSize := workerCount * 2
	jobs := make(chan similarity.Job, bufferSize)
	results := make(chan similarity.Result, bufferSize)

	for w := 0; w < workerCount; w++ {
		wg.Add(1)
		go similarity.Worker(ctx, w, jobs, results, events1, events2, calculator, &wg, h.logger)
	}

	// Dispatch jobs with context cancellation support
	go func() {
		defer close(jobs)
		for i := range events1 {
			for j := range events2 {
				select {
				case <-ctx.Done():
					return
				case jobs <- similarity.Job{I: i, J: j}:
				}
			}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	var similarEvents []models.SimilarEvent
	var similarMarkets []models.SimilarMarket

	for result := range results {
		if result.SimilarEvent != nil {
			similarEvents = append(similarEvents, *result.SimilarEvent)
			similarMarkets = append(similarMarkets, result.SimilarMarket...)
		}
	}

	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}

	return similarEvents, similarMarkets, nil
}
