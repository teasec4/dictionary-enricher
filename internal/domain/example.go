package domain

type Example struct {
	Headword  string    `json:"headword" db:"headword"`
	Text     string `json:"text" db:"text"`
}
