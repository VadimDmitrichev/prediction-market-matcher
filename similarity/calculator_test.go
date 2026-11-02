package similarity

import (
	"testing"

	"prediction-market-matcher/config"
)

func newTestCalculator() *Calculator {
	cfg := &config.Config{
		LevenshteinSimilarity: 80,
		MarketSimilarity:      70,
	}
	cfg.SimilarityThresholds.LevenshteinMin = 60
	cfg.SimilarityThresholds.KeywordOverlapMin = 0.3
	cfg.SimilarityThresholds.CapitalizedMin = 0.3

	return NewCalculator(cfg.LevenshteinSimilarity, cfg)
}

func TestCalculateLevenshteinSimilarity(t *testing.T) {
	calc := newTestCalculator()

	tests := []struct {
		name     string
		str1     string
		str2     string
		expected int
	}{
		{
			name:     "identical strings",
			str1:     "Will Bitcoin hit $100k in 2026?",
			str2:     "Will Bitcoin hit $100k in 2026?",
			expected: 100,
		},
		{
			name:     "case and whitespace insensitive",
			str1:     "  Will Ethereum ETF be approved? ",
			str2:     "will ethereum etf be approved?",
			expected: 100,
		},
		{
			name:     "empty strings",
			str1:     "",
			str2:     "",
			expected: 100,
		},
		{
			name:     "unicode characters rune length accuracy",
			str1:     "Цена — 100€",
			str2:     "Цена - 100€",
			expected: 82, // 1 rune difference out of 11 runes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := calc.CalculateLevenshteinSimilarity(tt.str1, tt.str2)
			if score != tt.expected {
				t.Errorf("got %d, want %d for inputs (%q, %q)", score, tt.expected, tt.str1, tt.str2)
			}
		})
	}
}

func TestCheckCriticalVerbs(t *testing.T) {
	calc := newTestCalculator()

	tests := []struct {
		name       string
		str1       string
		str2       string
		wantOK     bool
		wantReason string
	}{
		{
			name:       "direct modal negation (will vs won't)",
			str1:       "Will Fed cut rates in May?",
			str2:       "Won't Fed cut rates in May?",
			wantOK:     false,
			wantReason: "opposite_verbs",
		},
		{
			name:       "directional opposite (up vs down)",
			str1:       "Bitcoin price goes up today",
			str2:       "Bitcoin price goes down today",
			wantOK:     false,
			wantReason: "opposite_verbs",
		},
		{
			name:       "inserted negation (not)",
			str1:       "Real Madrid will win the Champions League",
			str2:       "Real Madrid will not win the Champions League",
			wantOK:     false,
			wantReason: "negation_change",
		},
		{
			name:       "consistent affirmative phrasing",
			str1:       "Will Arsenal win the Premier League?",
			str2:       "Will Arsenal win the English Premier League?",
			wantOK:     true,
			wantReason: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, reason := calc.CheckCriticalVerbs(tt.str1, tt.str2)
			if ok != tt.wantOK || reason != tt.wantReason {
				t.Errorf("CheckCriticalVerbs() = (%v, %q), want (%v, %q)", ok, reason, tt.wantOK, tt.wantReason)
			}
		})
	}
}

func TestTwoPassSimilarityCheck(t *testing.T) {
	calc := newTestCalculator()

	tests := []struct {
		name        string
		str1        string
		str2        string
		wantSimilar bool
		wantReason  string
	}{
		{
			name:        "matching prediction markets across platforms",
			str1:        "Will Real Madrid beat AS Monaco?",
			str2:        "Will Real Madrid beat Monaco?",
			wantSimilar: true,
			wantReason:  "key_entities_match",
		},
		{
			name:        "high levenshtein but different proper nouns (entity mismatch)",
			str1:        "Will Donald Trump win the election in November?",
			str2:        "Will Kamala Harris win the election in November?",
			wantSimilar: false,
			wantReason:  "different_capitalized_words",
		},
		{
			name:        "semantic inversion rejected on pass 1",
			str1:        "US inflation increased in October",
			str2:        "US inflation decreased in October",
			wantSimilar: false,
			wantReason:  "opposite_verbs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, res := calc.TwoPassSimilarityCheck(tt.str1, tt.str2)
			if res.Similar != tt.wantSimilar || res.Reason != tt.wantReason {
				t.Errorf("TwoPassSimilarityCheck() = (similar=%v, reason=%q), want (similar=%v, reason=%q)",
					res.Similar, res.Reason, tt.wantSimilar, tt.wantReason)
			}
		})
	}
}

// FuzzTwoPassSimilarityCheck feeds random/malformed strings and arbitrary UTF-8 sequences
// into the similarity engine to guarantee zero panics and strict score bounds [0, 100].
func FuzzTwoPassSimilarityCheck(f *testing.F) {
	f.Add("Will BTC hit $100k?", "Will BTC hit $100k in 2026?")
	f.Add("Real Madrid vs AS Monaco", "FC Barcelona vs Real Madrid")
	f.Add("", "   ")
	f.Add("🔥🚀 $100.5%!", "🔥🚀 won't $200%")

	calc := newTestCalculator()

	f.Fuzz(func(t *testing.T, s1, s2 string) {
		score, res := calc.TwoPassSimilarityCheck(s1, s2)
		if score < 0 || score > 100 {
			t.Fatalf("score out of bounds [0, 100]: %d for inputs (%q, %q)", score, s1, s2)
		}
		if res == nil {
			t.Fatal("expected non-nil result struct")
		}
	})
}

// BenchmarkTwoPassSimilarityCheck measures execution speed and heap allocations per pair comparison.
func BenchmarkTwoPassSimilarityCheck(b *testing.B) {
	calc := newTestCalculator()
	s1 := "Will Real Madrid win the UEFA Champions League Final in 2026?"
	s2 := "Will FC Real Madrid win the Champions League Final 2026?"

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calc.TwoPassSimilarityCheck(s1, s2)
	}
}
