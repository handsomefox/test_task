package main

type Image struct {
	ID        int64  `json:"id"`
	UserID    int32  `json:"user_id"`
	ImagePath string `json:"image_path"`
	ImageURL  string `json:"image_url"`
}

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"password_hash"`
}
