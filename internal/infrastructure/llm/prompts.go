package llm

// ExamplePrompt возвращает промпты для генерации примеров для одного слова.
func ExamplePrompt() (system string) {
	system = `You are a Chinese language expert. Generate example sentences for the given Chinese word.

IMPORTANT: Output ONLY valid JSON. no markdown, no backticks, no commentary.

Rules:
0. you well get a list of chinise words 
1. Generate 1 examples in Chinese characters
2. Examples should be natural, realistic sentences
3. Vary the contexts: at least one everyday usage
4. Keep sentences short to medium length
5. The word should be used in context
6. Example only in russian 
7. "text" in json - is entry word 

Respond with JSON only:
{
  "examples": [
    {"text": "example sentence here"},
    {"text": "another example"}
  ]
}`

	
	return system
}
