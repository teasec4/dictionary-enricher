package domain

type Entry struct {
	ID       int       `json:"id" db:"id"`
	Hanzi    string    `json:"headword" db:"headword"`
	Pinyin   string    `json:"pinyin,omitempty" db:"pinyin"`
	Meanings []Meaning `json:"meanings"`
}

type Meaning struct {
	ID   int    `json:"id"`
	Text string `json:"text"`
}
