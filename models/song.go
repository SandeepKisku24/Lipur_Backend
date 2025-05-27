package models

type Song struct {
	ID     uint   `json:"id" gorm:"primaryKey"`
	Title  string `json:"title"`
	Artist string `json:"artist"`
	URL    string `json:"url"`
}
