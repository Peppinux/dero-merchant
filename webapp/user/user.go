package user

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/pkg/errors"

	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/stringutil"
)

// User represents a user
type User struct {
	ID                int
	Username          string
	Email             string
	Password          string
	HashedPassword    string
	VerificationToken string
	Verified          bool
}

// HasValidUsername returns whether User's username has a valid length or not
func (u *User) HasValidUsername() bool {
	if len(u.Username) > 0 && len(u.Username) <= 16 {
		return true
	}
	return false
}

// HasValidEmail returns whether User's email has a valid length or not.
// Not using RegEx since a verification email is sent at Sign Up anyway. If email is invalid, the account will not be able to be verified.
func (u *User) HasValidEmail() bool {
	if len(u.Email) >= 8 && len(u.Email) <= 64 {
		return true
	}
	return false
}

// HasValidPassword returns whether User's password has a valid length or not
func (u *User) HasValidPassword() bool {
	if len(u.Password) >= 8 && len(u.Password) <= 64 {
		return true
	}
	return false
}

// HasValidVerificationToken returns wheter User's verification token has a valid length or not
func (u *User) HasValidVerificationToken() bool {
	if len(u.VerificationToken) == 64 {
		return true
	}
	return false
}

// IsUnique returns whether User's username and email are unique or not
func (u *User) IsUnique() (uniqueUsername bool, uniqueEmail bool, err error) {
	lowerUsername := strings.ToLower(u.Username)

	var (
		existingUsername string
		existingEmail    string
	)
	err = postgres.DB.QueryRow(`
		SELECT LOWER(username), email 
		FROM users 
		WHERE LOWER(username)=$1 OR email=$2`, lowerUsername, u.Email).
		Scan(&existingUsername, &existingEmail)
	if err != nil {
		if err == sql.ErrNoRows {
			return true, true, nil
		}

		err = errors.Wrap(err, "cannot query database")
		return
	}

	if lowerUsername != existingUsername {
		uniqueUsername = true
	}
	if u.Email != existingEmail {
		uniqueEmail = true
	}

	return
}

type tokenType int

const (
	verification tokenType = iota
	recover                = iota
)

func (t tokenType) String() string {
	switch t {
	case verification:
		return "verification"
	case recover:
		return "recover"
	default:
		return ""
	}
}

var errInvalidTokenType = errors.New("invalid token type")

func generateUserToken() (string, error) {
	return stringutil.RandomBase64RawURLString(48)
}

func isUniqueUserToken(t tokenType, token string) (bool, error) {
	typeString := t.String()
	if typeString == "" {
		return false, errInvalidTokenType
	}

	q := fmt.Sprintf(`
		SELECT id
		FROM users
		WHERE %s_token=$1`, typeString)

	var userID int
	err := postgres.DB.QueryRow(q, token).Scan(&userID)
	if err != nil {
		if err == sql.ErrNoRows { // User token is unique
			return true, nil
		}

		return false, errors.Wrap(err, "cannot query database")
	}

	return false, nil
}

// GenerateUniqueUserToken generates a verification/recover token that is not already in use by another user
func GenerateUniqueUserToken(t tokenType) (token string, err error) {
	for {
		// Generate user token
		token, err = generateUserToken()
		if err != nil {
			return "", errors.Wrap(err, "cannot generate user token")
		}

		isUnique, err := isUniqueUserToken(t, token)
		if err != nil {
			return "", errors.Wrap(err, "cannot check if user token is unique")
		}

		if isUnique {
			break
		}
	}

	return
}

// GenerateVerificationTokenAsync generates a verification token and returns it in a channel
func GenerateVerificationTokenAsync() chan string {
	c := make(chan string)

	go func() {
		token, err := GenerateUniqueUserToken(verification)
		if err != nil {
			c <- ""
			return
		}
		c <- token
	}()

	return c
}

// HashPasswordAsync hashes a password and returns it in a channel
func HashPasswordAsync(password string) chan string {
	c := make(chan string)

	go func() {
		hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
		if err != nil {
			c <- ""
			return
		}
		c <- hash
	}()

	return c
}

// New User input validation errors
var (
	ErrInvalidUsername   = errors.New("Invalid username")
	ErrPasswordNotMatch  = errors.New("Passwords do not match")
	ErrUsernameNotUnique = errors.New("Username is not unique")
	ErrEmailNotUnique    = errors.New("Email is not unique")
)

// CreateNewUser returns a new User ready to be inserted into DB
func CreateNewUser(username, email, password, confirmPassword string) (u *User, errs []error) {
	// Sanitize input
	u = &User{
		Username: strings.TrimSpace(username),
		Email:    strings.TrimSpace(strings.ToLower(email)),
	}

	// Further validate input
	if !u.HasValidUsername() {
		errs = append(errs, ErrInvalidUsername)
	}
	if password != confirmPassword {
		errs = append(errs, ErrPasswordNotMatch)
	}
	if errs != nil {
		return
	}

	// Check if username and email are already used by someone else
	uniqueUsername, uniqueEmail, err := u.IsUnique()
	if err != nil {
		errs = append(errs, errors.Wrap(err, "cannot check if user is unique"))
		return
	}
	if !uniqueUsername {
		errs = append(errs, ErrUsernameNotUnique)
	}
	if !uniqueEmail {
		errs = append(errs, ErrEmailNotUnique)
	}
	if errs != nil {
		return
	}

	tokenChan := GenerateVerificationTokenAsync()
	hashChan := HashPasswordAsync(password)

	for i := 0; i < 2; i++ {
		// Wait for both verification token and hashed password simultaneously
		select {
		case u.VerificationToken = <-tokenChan:
			if u.VerificationToken == "" {
				errs = append(errs, errors.Wrap(err, "cannot generate verification token"))
				return
			}
		case u.HashedPassword = <-hashChan:
			if u.HashedPassword == "" {
				errs = append(errs, errors.Wrap(err, "cannot hash password"))
				return
			}
		}
	}

	return
}

// SendVerificationEmail TODO
func (u *User) SendVerificationEmail() error {
	return nil
}

// Insert inserts a User into DB
func (u *User) Insert() error {
	tx, err := postgres.DB.Begin()
	if err != nil {
		return errors.Wrap(err, "cannot begin DB TX")
	}

	_, err = tx.Exec(`
		INSERT INTO users (username, email, password, verification_token) 
		VALUES ($1, $2, $3, $4)`, u.Username, u.Email, u.HashedPassword, u.VerificationToken)
	if err != nil {
		return errors.Wrap(err, "cannot execute query")
	}

	// TODO: Inviare email con verification token ad utente qui
	// Se errore durante invio TX Rollback
	err = u.SendVerificationEmail()
	if err != nil {
		return errors.Wrap(err, "cannot send verification email")
	}

	err = tx.Commit()
	if err != nil {
		return errors.Wrap(err, "cannot commit DB TX")
	}

	return nil
}

// TODO: Continue refactoring user handlers into user functions and write tests.
