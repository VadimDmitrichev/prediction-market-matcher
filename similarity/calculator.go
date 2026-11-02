package similarity

import (
	"regexp"
	"strings"
	"unicode"

	levenshtein "github.com/texttheater/golang-levenshtein/levenshtein"

	"prediction-market-matcher/config"
	"prediction-market-matcher/models"
	logger "prediction-market-matcher/utils"
)

// Pre-compiled regular expressions and replacers to avoid allocations in hot loops.
var (
	reNumbers = regexp.MustCompile(`\d+(?:\.\d+)?%?`)
	reNonWord = regexp.MustCompile(`[^\w\s]`)

	// Deterministic single-pass string replacer (replaces randomized map iteration).
	termReplacer = strings.NewReplacer(
		"vs.", "vs",
		"vs ", "versus ",
		"winner", "win",
		"winning", "win",
		"real madrid", "realmadrid",
		"as monaco", "monaco",
	)

	// Zero-byte value sets (map[string]struct{}) for memory efficiency.
	stopWords = map[string]struct{}{
		"the": {}, "a": {}, "an": {}, "in": {}, "on": {}, "at": {},
		"to": {}, "for": {}, "of": {}, "with": {}, "by": {}, "is": {},
		"was": {}, "are": {}, "were": {}, "and": {}, "or": {}, "but": {},
		"this": {}, "that": {}, "these": {}, "those": {}, "it": {},
		"vs": {}, "versus": {},
	}

	opposites = map[string][]string{
		"increased": {"decreased"},
		"decreased": {"increased"},
		"rose":      {"fell"},
		"fell":      {"rose"},
		"up":        {"down"},
		"down":      {"up"},
		"will":      {"won't"},
		"won't":     {"will"},
		"can":       {"cannot"},
		"cannot":    {"can"},
		"not":       {""},
		"no":        {""},
		"never":     {""},
	}

	entityPrefixes = []string{"as ", "fc ", "real ", "atletico ", "fc", "as"}
)

type Calculator struct {
	thresholds struct {
		levenshtein int     // Default: 80%
		jaccard     float64 // Default: 0.7
		dice        float64 // Default: 0.7
		lcs         float64 // Default: 0.6
	}
	cfg *config.Config
}

func NewCalculator(levenshteinSimilarity int, cfg *config.Config) *Calculator {
	c := &Calculator{
		cfg: cfg,
	}
	c.thresholds.levenshtein = levenshteinSimilarity
	c.thresholds.jaccard = 0.7
	c.thresholds.dice = 0.7
	c.thresholds.lcs = 0.6
	return c
}

// normalizeString converts a string to lowercase and trims leading/trailing whitespace.
func normalizeString(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

// CalculateLevenshteinSimilarity computes the similarity percentage (0-100) using rune-based Levenshtein distance.
func (c *Calculator) CalculateLevenshteinSimilarity(str1, str2 string) int {
	r1 := []rune(normalizeString(str1))
	r2 := []rune(normalizeString(str2))

	maxLen := max(len(r1), len(r2))
	if maxLen == 0 {
		return 100
	}

	distance := levenshtein.DistanceForStrings(r1, r2, levenshtein.DefaultOptions)
	similarity := 100 - (distance*100)/maxLen
	if similarity < 0 {
		return 0
	}
	return similarity
}

// CalculateJaccardSimilarity computes the Jaccard index over tokenized words (excluding stop words).
func (c *Calculator) CalculateJaccardSimilarity(str1, str2 string) float64 {
	set1 := wordsToSet(tokenize(normalizeString(str1), true))
	set2 := wordsToSet(tokenize(normalizeString(str2), true))

	intersection := countIntersection(set1, set2)
	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// CalculateDiceCoefficient computes the Sørensen–Dice coefficient over tokenized words.
func (c *Calculator) CalculateDiceCoefficient(str1, str2 string) float64 {
	set1 := wordsToSet(tokenize(normalizeString(str1), true))
	set2 := wordsToSet(tokenize(normalizeString(str2), true))

	total := len(set1) + len(set2)
	if total == 0 {
		return 1.0
	}

	intersection := countIntersection(set1, set2)
	return 2.0 * float64(intersection) / float64(total)
}

// extractCapitalizedAndNumbers extracts proper nouns (capitalized tokens) and numerical values.
func extractCapitalizedAndNumbers(str string) (capitalized []string, numbers []string) {
	words := strings.Fields(str)
	for _, word := range words {
		runes := []rune(word)
		if len(runes) > 0 && unicode.IsUpper(runes[0]) {
			capitalized = append(capitalized, word)
		}
	}

	numbers = reNumbers.FindAllString(str, -1)
	return capitalized, numbers
}

// CheckCapitalizedAndNumbers verifies whether normalized proper nouns and numerical tokens match.
func (c *Calculator) CheckCapitalizedAndNumbers(str1, str2 string) (bool, string) {
	cap1, num1 := extractCapitalizedAndNumbers(str1)
	cap2, num2 := extractCapitalizedAndNumbers(str2)

	// Нормализуем имена (убираем префиксы вроде "as", "fc" и склеиваем составные имена)
	norm1 := c.normalizeNames(cap1)
	norm2 := c.normalizeNames(cap2)

	capSet1 := wordsToSet(norm1)
	capSet2 := wordsToSet(norm2)

	if len(norm1) > 0 && len(norm2) > 0 {
		for word := range capSet1 {
			// Игнорируем токены короче 2 символов
			if _, ok := capSet2[word]; !ok && len(word) > 1 {
				return false, "different_capitalized_words"
			}
		}
		for word := range capSet2 {
			if _, ok := capSet1[word]; !ok && len(word) > 1 {
				return false, "different_capitalized_words"
			}
		}
	}

	if len(num1) != len(num2) {
		return false, "different_numbers_count"
	}

	for i := range num1 {
		if num1[i] != num2[i] {
			return false, "different_numbers"
		}
	}

	return true, ""
}

// tokenize splits a string into words and optionally filters out stop words.
func tokenize(s string, filterStopWords bool) []string {
	words := strings.Fields(s)
	if !filterStopWords {
		return words
	}

	filtered := make([]string, 0, len(words))
	for _, word := range words {
		if _, isStop := stopWords[word]; !isStop {
			filtered = append(filtered, word)
		}
	}
	return filtered
}

// wordsToSet converts a slice of strings into a memory-efficient hash set.
func wordsToSet(words []string) map[string]struct{} {
	set := make(map[string]struct{}, len(words))
	for _, word := range words {
		set[word] = struct{}{}
	}
	return set
}

func countIntersection(set1, set2 map[string]struct{}) int {
	// Iterate over the smaller set for O(min(N, M)) lookup efficiency.
	if len(set1) > len(set2) {
		set1, set2 = set2, set1
	}
	intersection := 0
	for word := range set1 {
		if _, ok := set2[word]; ok {
			intersection++
		}
	}
	return intersection
}

// CalculateLCS computes the Longest Common Subsequence score at the word level.
func (c *Calculator) CalculateLCS(str1, str2 string) float64 {
	words1 := tokenize(normalizeString(str1), true)
	words2 := tokenize(normalizeString(str2), true)

	// Cap at 10 words to bound DP complexity.
	if len(words1) > 10 {
		words1 = words1[:10]
	}
	if len(words2) > 10 {
		words2 = words2[:10]
	}

	n, m := len(words1), len(words2)
	totalWords := n + m
	if totalWords == 0 {
		return 1.0
	}

	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}

	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if words1[i-1] == words2[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else {
				dp[i][j] = max(dp[i-1][j], dp[i][j-1])
			}
		}
	}

	return 2.0 * float64(dp[n][m]) / float64(totalWords)
}

// CheckCriticalVerbs detects semantic inversions (e.g., "will" vs "won't", "rose" vs "fell").
func (c *Calculator) CheckCriticalVerbs(str1, str2 string) (bool, string) {
	wordSet1 := wordsToSet(tokenize(normalizeString(str1), false))
	wordSet2 := wordsToSet(tokenize(normalizeString(str2), false))

	hasNegation1 := false
	hasNegation2 := false

	for word1 := range wordSet1 {
		if oppList, exists := opposites[word1]; exists {
			if word1 == "not" || word1 == "no" || word1 == "never" {
				hasNegation1 = true
			}
			for _, oppositeWord := range oppList {
				if _, ok := wordSet2[oppositeWord]; oppositeWord != "" && ok {
					return false, "opposite_verbs"
				}
			}
		}
	}

	for word2 := range wordSet2 {
		if oppList, exists := opposites[word2]; exists {
			if word2 == "not" || word2 == "no" || word2 == "never" {
				hasNegation2 = true
			}
			for _, oppositeWord := range oppList {
				if _, ok := wordSet1[oppositeWord]; oppositeWord != "" && ok {
					return false, "opposite_verbs"
				}
			}
		}
	}

	if hasNegation1 != hasNegation2 {
		return false, "negation_change"
	}

	return true, ""
}

func (c *Calculator) preprocessString(s string) string {
	s = strings.ToLower(s)
	s = reNonWord.ReplaceAllString(s, " ")
	s = termReplacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

func (c *Calculator) tokenizeAndClean(s string) []string {
	norm := c.preprocessString(s)
	return tokenize(norm, true)
}

func (c *Calculator) calculateGenerousJaccard(str1, str2 string) float64 {
	set1 := wordsToSet(c.tokenizeAndClean(str1))
	set2 := wordsToSet(c.tokenizeAndClean(str2))

	intersection := countIntersection(set1, set2)
	union := len(set1) + len(set2) - intersection
	if union == 0 {
		return 1.0
	}

	bonus := 0.0
	for w := range set1 {
		if _, ok := set2[w]; ok && len(w) >= 5 {
			bonus += 0.1
		}
	}

	score := float64(intersection)/float64(union) + bonus
	if score > 1.0 {
		return 1.0
	}
	return score
}

func (c *Calculator) calculateGenerousDice(str1, str2 string) float64 {
	set1 := wordsToSet(c.tokenizeAndClean(str1))
	set2 := wordsToSet(c.tokenizeAndClean(str2))

	total := len(set1) + len(set2)
	if total == 0 {
		return 1.0
	}

	intersection := countIntersection(set1, set2)
	dice := 2.0 * float64(intersection) / float64(total)
	partialBonus := c.calculatePartialMatches(set1, set2)

	score := dice + partialBonus
	if score > 1.0 {
		return 1.0
	}
	return score
}

func (c *Calculator) calculatePartialMatches(set1, set2 map[string]struct{}) float64 {
	bonus := 0.0
	for w1 := range set1 {
		for w2 := range set2 {
			if w1 != w2 && (strings.Contains(w1, w2) || strings.Contains(w2, w1)) {
				minLen := min(len(w1), len(w2))
				bonus += 0.05 * float64(minLen) / 10.0
			}
		}
	}
	if bonus > 0.3 {
		return 0.3
	}
	return bonus
}

func (c *Calculator) calculateKeywordOverlap(str1, str2 string) float64 {
	keywords1 := c.extractKeywords(str1)
	keywords2 := c.extractKeywords(str2)

	if len(keywords1) == 0 && len(keywords2) == 0 {
		return 1.0
	}

	intersection := 0
	for _, k1 := range keywords1 {
		k1Lower := strings.ToLower(k1)
		for _, k2 := range keywords2 {
			k2Lower := strings.ToLower(k2)
			if k1Lower == k2Lower || strings.Contains(k1Lower, k2Lower) || strings.Contains(k2Lower, k1Lower) {
				intersection++
				break
			}
		}
	}

	union := len(keywords1) + len(keywords2) - intersection
	if union <= 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

func (c *Calculator) extractKeywords(s string) []string {
	words := strings.Fields(s)
	keywords := make([]string, 0, len(words))

	for _, word := range words {
		cleanWord := strings.Trim(word, ".,!?;:'\"")
		runes := []rune(cleanWord)
		if len(runes) > 3 || (len(runes) > 0 && unicode.IsUpper(runes[0])) {
			keywords = append(keywords, cleanWord)
		}
	}
	return keywords
}

func (c *Calculator) softCheckCapitalized(str1, str2 string) (float64, map[string]interface{}) {
	cap1 := c.extractCapitalized(str1)
	cap2 := c.extractCapitalized(str2)

	details := map[string]interface{}{
		"cap1": cap1,
		"cap2": cap2,
	}

	norm1 := c.normalizeNames(cap1)
	norm2 := c.normalizeNames(cap2)

	total := len(norm1) + len(norm2)
	if total == 0 {
		return 1.0, details
	}

	intersection := 0
	for _, n1 := range norm1 {
		for _, n2 := range norm2 {
			if n1 == n2 {
				intersection++
				break
			}
		}
	}

	score := 2.0 * float64(intersection) / float64(total)

	if len(norm1) > 0 && len(norm2) > 0 {
		allFound := true
		for _, n1 := range norm1 {
			found := false
			for _, n2 := range norm2 {
				if n1 == n2 {
					found = true
					break
				}
			}
			if !found {
				allFound = false
				break
			}
		}
		if allFound {
			score += 0.2
		}
	}

	if score > 1.0 {
		score = 1.0
	}

	details["score"] = score
	details["intersection"] = intersection
	details["total"] = total

	return score, details
}

func (c *Calculator) normalizeNames(names []string) []string {
	normalized := make([]string, 0, len(names))
	for _, name := range names {
		norm := strings.ToLower(name)
		for _, prefix := range entityPrefixes {
			if strings.HasPrefix(norm, prefix) {
				norm = strings.TrimPrefix(norm, prefix)
				break
			}
		}
		norm = strings.ReplaceAll(norm, " ", "")
		if len(norm) > 0 {
			normalized = append(normalized, norm)
		}
	}
	return normalized
}

func (c *Calculator) extractCapitalized(s string) []string {
	words := strings.Fields(s)
	var result []string

	for _, word := range words {
		cleanWord := strings.Trim(word, ".,!?;:'\"")
		runes := []rune(cleanWord)
		if len(runes) > 0 && unicode.IsUpper(runes[0]) {
			result = append(result, cleanWord)
		}
	}
	return result
}

// TwoPassSimilarityCheck executes a two-stage filtering pipeline:
// Pass 1: Fast Levenshtein distance & critical verb/negation inversion check.
// Pass 2: Named entity (capitalized tokens) and keyword overlap verification.
func (c *Calculator) TwoPassSimilarityCheck(str1, str2 string) (int, *models.SimilarityMarketResult) {
	thresholds := c.cfg.SimilarityThresholds

	result := &models.SimilarityMarketResult{
		Details: make(map[string]interface{}),
	}

	// Pass 1.1: Semantic inversion check (e.g., "will" vs "won't")
	if ok, reason := c.CheckCriticalVerbs(str1, str2); !ok {
		result.Similar = false
		result.Reason = reason
		result.FinalScore = 0
		return 0, result
	}

	// Pass 1.2: Base Levenshtein similarity
	levScore := c.CalculateLevenshteinSimilarity(str1, str2)
	result.Levenshtein = levScore
	logger.Debug("calculated levenshtein score", "event1", str1, "event2", str2, "lev_score", levScore)

	if levScore < thresholds.LevenshteinMin {
		result.Similar = false
		result.Reason = "low_levenshtein"
		result.FinalScore = levScore
		return levScore, result
	}

	// Pass 2: Entity & Keyword verification
	if ok, reason := c.CheckCapitalizedAndNumbers(str1, str2); !ok {
		result.Similar = false
		result.Reason = reason
		result.FinalScore = min(levScore, 50)
		return result.FinalScore, result
	}

	keywordOverlap := c.calculateKeywordOverlap(str1, str2)
	capScore, _ := c.softCheckCapitalized(str1, str2)

	logger.Debug("entity verification scores",
		"event1", str1,
		"event2", str2,
		"keyword_overlap", keywordOverlap,
		"cap_score", capScore,
	)

	result.Details["keyword_overlap"] = keywordOverlap
	result.Details["capitalized_score"] = capScore

	if keywordOverlap < thresholds.KeywordOverlapMin || capScore < thresholds.CapitalizedMin {
		result.Similar = false
		result.Reason = "key_entities_dont_match"
		result.FinalScore = min(levScore, 50)
		return result.FinalScore, result
	}

	result.Similar = true
	result.Reason = "key_entities_match"
	result.FinalScore = levScore

	return result.FinalScore, result
}

func (c *Calculator) CalculateMarketSimilarity(str1, str2 string) int {
	score, _ := c.TwoPassSimilarityCheck(str1, str2)
	return score
}
