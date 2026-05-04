package domain

type Entry struct {
	ID               int       `json:"id" db:"id"`
	Hanzi            string    `json:"headword" db:"headword"`
	Pinyin           string    `json:"pinyin,omitempty" db:"pinyin"`
	PinyinNormalized string    `json:"pinyin_normalized,omitempty" db:"pinyin_normalized"`
	Meanings         []Meaning `json:"meanings"`
}

type Meaning struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}
