package nubo_error

import "errors"

// Erreurs métier spécifiques, routables par le handler HTTP
var (
	ErrUsernameTaken = errors.New("This username is already taken")
	ErrEmailTaken    = errors.New("This email is already taken")
	ErrPhoneTaken    = errors.New("This phone number is already taken")
	ErrInvalidDate   = errors.New("Invalid date format. Expected format: ddmmaaaa")
	ErrAgeUnder13    = errors.New("You must be at least 13 years old")
	ErrAgeOver120    = errors.New("Invalid birthdate")
	ErrInvalidGender = errors.New("Gender must be 0, 1, 2, or null")
)
