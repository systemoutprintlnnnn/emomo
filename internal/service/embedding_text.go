package service

import "strings"

const maxVLMEmbeddingRunes = 120

func normalizeWhitespace(text string) string {
	if text == "" {
		return ""
	}
	return strings.Join(strings.Fields(text), " ")
}

func normalizeOCRText(text string) string {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.Trim(trimmed, "\"'`")
	if trimmed == "" {
		return ""
	}

	normalized := strings.Trim(trimmed, " .，。;；:：!！?？")
	lower := strings.ToLower(normalized)
	switch lower {
	case "none", "no text", "no_text", "n/a", "null":
		return ""
	}

	switch normalized {
	case "无文字", "没有文字", "无内容", "无文本", "无字", "无文字内容":
		return ""
	}

	return normalizeWhitespace(trimmed)
}

func compactDescription(text string) string {
	cleaned := normalizeWhitespace(strings.TrimSpace(text))
	if cleaned == "" {
		return ""
	}

	runes := []rune(cleaned)
	if len(runes) <= maxVLMEmbeddingRunes {
		return cleaned
	}
	return string(runes[:maxVLMEmbeddingRunes])
}

func extractEmotionWords(text string) []string {
	if text == "" {
		return nil
	}

	lower := strings.ToLower(text)
	matches := make([]string, 0, len(EmotionWords))
	for _, word := range EmotionWords {
		if word == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(word)) {
			matches = append(matches, word)
		}
	}
	return dedupeStrings(matches)
}

func buildEmbeddingText(ocrText, description string, tags, emotions []string) string {
	segments := make([]string, 0, 4)
	if ocrText != "" {
		segments = append(segments, "ocr:"+ocrText)
	}
	if description != "" {
		segments = append(segments, "desc:"+description)
	}
	tags = dedupeStrings(tags)
	if len(tags) > 0 {
		segments = append(segments, "tags:"+strings.Join(tags, " "))
	}
	emotions = dedupeStrings(emotions)
	if len(emotions) > 0 {
		segments = append(segments, "emotions:"+strings.Join(emotions, " "))
	}
	return strings.Join(segments, "\n")
}

func buildBM25Text(ocrText, description string, tags []string) string {
	segments := make([]string, 0, 3)
	if ocrText != "" {
		segments = append(segments, ocrText)
	}
	if description != "" {
		segments = append(segments, description)
	}
	tags = dedupeStrings(tags)
	if len(tags) > 0 {
		segments = append(segments, strings.Join(tags, " "))
	}
	return strings.Join(segments, "\n")
}

func dedupeStrings(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	result := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}
