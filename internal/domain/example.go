package domain

type Example struct {
	Headword  int    `json:"headword" db:"headword"`
	Text     string `json:"text" db:"text"`
}
