package models

// Login struct to describe login payload with an optional second factor code.
type Login struct {
	Email    string `json:"email" validate:"required,email,lte=255"`
	Password string `json:"password" validate:"required,lte=255"`
	Code     string `json:"code,omitempty"`
}
