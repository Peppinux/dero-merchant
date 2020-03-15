package user

import (
	"database/sql"
	"net/http"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator"

	"github.com/peppinux/dero-merchant/auth"
	"github.com/peppinux/dero-merchant/cryptoutil"
	"github.com/peppinux/dero-merchant/httperror"
	"github.com/peppinux/dero-merchant/postgres"
	"github.com/peppinux/dero-merchant/redis"
	"github.com/peppinux/dero-merchant/webapp/store"
)

type signUpFields struct {
	Username        string `form:"username" binding:"required,max=16"`
	Email           string `form:"email" binding:"required,max=64,min=8"`
	Password        string `form:"password" binding:"required,max=64,min=8"`
	ConfirmPassword string `form:"confirm-password" binding:"required,max=64,min=8"`
}

type signUpErrors struct {
	Username        bool
	UniqueUsername  bool
	Email           bool
	UniqueEmail     bool
	Password        bool
	ConfirmPassword bool
	PasswordMatch   bool
}

type signUpData struct {
	Fields  *signUpFields
	Errors  *signUpErrors
	Success bool
}

// SignUpGetHandler handles GET requests to /user/signup
func SignUpGetHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "signup.html", nil)
}

// SignUpPostHandler handles POST requests to /user/signup
func SignUpPostHandler(c *gin.Context) {
	// Get User data submitted through form
	var (
		f    signUpFields
		e    signUpErrors
		resp = &signUpData{
			Fields:  &f,
			Errors:  &e,
			Success: false,
		}
	)
	err := c.ShouldBind(&f)
	if err != nil {
		// Validate User input
		errs := err.(validator.ValidationErrors)
		for _, err := range errs {
			switch err.Field() {
			case "Username":
				e.Username = true
			case "Email":
				e.Email = true
			case "Password":
				e.Password = true
			case "ConfirmPassword":
				e.ConfirmPassword = true
			}
		}
		if errs != nil {
			c.HTML(http.StatusUnprocessableEntity, "signup.html", resp)
			return
		}
	}

	u, errs := CreateNewUser(f.Username, f.Email, f.Password, f.ConfirmPassword)
	if errs != nil {
		for _, err := range errs {
			switch err {
			case ErrInvalidUsername:
				e.Username = true
			case ErrPasswordNotMatch:
				e.PasswordMatch = true
			case ErrUsernameNotUnique:
				e.UniqueUsername = true
			case ErrEmailNotUnique:
				e.UniqueEmail = true
			default:
				httperror.Render500(c, err, "Error creating new user")
				return
			}
		}

		if e.Username || e.PasswordMatch {
			c.HTML(http.StatusUnprocessableEntity, "signup.html", resp)
			return
		}

		if e.UniqueUsername || e.UniqueEmail {
			c.HTML(http.StatusForbidden, "signup.html", resp)
			return
		}
	}

	err = u.Insert()
	if httperror.Render500IfErr(c, err, "Error inserting user into DB") != nil {
		return
	}

	resp.Success = true
	c.HTML(http.StatusOK, "signup.html", resp)
}

type verifyData struct {
	UserSignedIn bool
	Token        string
	TokenExpired bool
	Success      bool
}

// VerifyHandler handles GET requests to /user/verify
func VerifyHandler(c *gin.Context) {
	// Get and sanitize token submitted by user through query params
	s := c.MustGet("session").(*auth.Session)

	data := &verifyData{
		UserSignedIn: s.SignedIn,
		Token:        strings.TrimSpace(c.Query("token")),
		Success:      false,
	}

	// Validate token
	if len(data.Token) != 64 {
		c.HTML(http.StatusUnprocessableEntity, "verify.html", data)
		return
	}

	// Fetch email of the owner of the token and whether the token has expired or not
	var email string
	err := postgres.DB.QueryRow(`
		SELECT email, ((verification_token_expiration_date - NOW()) < INTERVAL '0 second') AS token_expired 
		FROM users 
		WHERE email_verified=$1 AND verification_token=$2`, false, data.Token).
		Scan(&email, &data.TokenExpired)
	if err != nil {
		if err == sql.ErrNoRows {
			// If no email was found, render failure page
			c.HTML(http.StatusNotFound, "verify.html", data)
			return
		}

		httperror.Render500(c, err, "Error querying database")
		return
	}

	// If token has expired, render failure page
	if data.TokenExpired {
		c.HTML(http.StatusForbidden, "verify.html", data)
		return
	}

	// If token is valid, update user in DB (Set verified)
	_, err = postgres.DB.Exec(`
		UPDATE users 
		SET email_verified=$1 
		WHERE email=$2`, true, email)
	if httperror.Render500IfErr(c, err, "Error executing query") != nil {
		return
	}

	data.Success = true
	c.HTML(http.StatusOK, "verify.html", data)
}

type newVerificationTokenFields struct {
	Email string `form:"email" binding:"required,max=64,min=8"`
}

type newVerificationTokenData struct {
	UserSignedIn bool
	Fields       *newVerificationTokenFields
	Success      bool
}

// NewVerificationTokenGetHandler handles GET requests to /user/new_verification_token
func NewVerificationTokenGetHandler(c *gin.Context) {
	s := c.MustGet("session").(*auth.Session)
	resp := &newVerificationTokenData{
		UserSignedIn: s.SignedIn,
		Fields:       &newVerificationTokenFields{},
	}
	c.HTML(http.StatusOK, "new_verification_token.html", resp)
}

// NewVerificationTokenPostHandler handles POST requests to /user/new_verification_token
func NewVerificationTokenPostHandler(c *gin.Context) {
	s := c.MustGet("session").(*auth.Session)

	var (
		f    newVerificationTokenFields
		resp = &newVerificationTokenData{
			UserSignedIn: s.SignedIn,
			Fields:       &f,
			Success:      false,
		}
	)

	// Get and validate email submitted by user through form
	err := c.ShouldBind(&f)
	if err != nil {
		c.HTML(http.StatusUnprocessableEntity, "new_verification_token.html", resp)
		return
	}

	// Sanitize email
	user := &User{
		Email: strings.ToLower(strings.TrimSpace(f.Email)),
	}

	// Check if user with submitted email exists and is not already verified in DB
	err = postgres.DB.QueryRow(`
		SELECT email_verified 
		FROM users 
		WHERE email=$1`, user.Email).
		Scan(&user.Verified)
	if err != nil {
		if err == sql.ErrNoRows {
			// If email was not found in database, render failure page
			c.HTML(http.StatusForbidden, "new_verification_token.html", resp)
			return

		}

		httperror.Render500(c, err, "Error querying database")
		return
	}

	// If email is already verified, render failure page
	if user.Verified {
		c.HTML(http.StatusOK, "new_verification_token.html", resp)
		return
	}

	// Generate new verification token
	token, err := GenerateUniqueUserToken(verification)
	if httperror.Render500IfErr(c, err, "Error generating verification token") != nil {
		return
	}

	tx, err := postgres.DB.Begin()
	if httperror.Render500IfErr(c, err, "Error beginning TX") != nil {
		return
	}

	// Update user in DB (Set new verification token)
	_, err = tx.Exec(`
		UPDATE users 
		SET verification_token=$1, verification_token_expiration_date=NOW()+interval '1 hour' 
		WHERE email=$2`, token, user.Email)
	if httperror.Render500IfErr(c, err, "Error executing query") != nil {
		tx.Rollback()
		return
	}

	// TODO: Mandare nuovo activation link ad email
	// Se errore durante invio TX Rollback

	err = tx.Commit()
	if httperror.Render500IfErr(c, err, "Error committing TX") != nil {
		return
	}

	resp.Success = true
	c.HTML(http.StatusOK, "new_verification_token.html", resp)
}

type signInFields struct {
	Username   string `form:"username" binding:"required,max=64"`
	Password   string `form:"password" binding:"required,max=64,min=8"`
	RememberMe string `form:"remember"`
}

type signInErrors struct {
	Username    bool
	Password    bool
	NotVerified bool
	NoMatch     bool
}

type signInData struct {
	Fields *signInFields
	Errors *signInErrors
}

const (
	oneDay   = 60 * 60 * 24
	oneMonth = 60 * 60 * 24 * 30
)

// SignInGetHandler handles GET requests to /user/signin
func SignInGetHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "signin.html", nil)
}

// SignInPostHandler handles POST requests to /user/signin
func SignInPostHandler(c *gin.Context) {
	var (
		f    signInFields
		e    signInErrors
		resp = &signInData{
			Fields: &f,
			Errors: &e,
		}
	)

	// Get User data submitted through form
	err := c.ShouldBind(&f)
	if err != nil {
		// Validate User input
		errs := err.(validator.ValidationErrors)
		for _, err := range errs {
			switch err.Field() {
			case "Username":
				e.Username = true
			case "Password":
				e.Password = true
			}
		}
		if errs != nil {
			c.HTML(http.StatusUnprocessableEntity, "signin.html", resp)
			return
		}
	}

	// Sanitize input
	user := &User{
		Username: strings.TrimSpace(f.Username),
		Email:    strings.TrimSpace(strings.ToLower(f.Username)),
		Password: f.Password,
	}

	// Further validate input
	if !user.HasValidUsername() && !user.HasValidEmail() {
		e.Username = true
		c.HTML(http.StatusUnprocessableEntity, "signin.html", resp)
		return
	}
	if f.RememberMe == "on" {
		f.RememberMe = "checked"
	}

	// Fetch user(s) with the username/email submitted by the User
	rows, err := postgres.DB.Query(`
		SELECT id, username, email, password, email_verified 
		FROM users 
		WHERE LOWER(username)=LOWER($1) OR email=$2`, user.Username, user.Email)
	if httperror.Render500IfErr(c, err, "Error querying database") != nil {
		return
	}

	defer rows.Close()

	for rows.Next() {
		var hashedPassword string
		err := rows.Scan(&user.ID, &user.Username, &user.Email, &hashedPassword, &user.Verified)
		if httperror.Render500IfErr(c, err, "Error scanning row") != nil {
			return
		}

		match, err := argon2id.ComparePasswordAndHash(user.Password, hashedPassword)
		if httperror.Render500IfErr(c, err, "Error comparing passwords") != nil {
			return
		}

		if !match {
			continue
		}

		if !user.Verified {
			e.NotVerified = true
			c.HTML(http.StatusForbidden, "signin.html", resp)
		}

		sessionID, err := auth.GenerateUniqueSessionID()
		if httperror.Render500IfErr(c, err, "Error generating unique Session ID") != nil {
			return
		}

		hashedSessionID := cryptoutil.HashStringToSHA256Hex(sessionID)

		// Store hashed Session ID in Redis
		err = redis.SetSessionUser(hashedSessionID, user.ID)
		if httperror.Render500IfErr(c, err, "Error setting hashed session's user ID in Redis") != nil {
			return
		}

		err = redis.AddUserSession(user.ID, hashedSessionID)
		if httperror.Render500IfErr(c, err, "Error adding session ID to user's sessions set") != nil {
			return
		}

		// Store username and email in Redis for quick retrieving in other pages
		redis.SetUserUsername(user.ID, user.Username)
		redis.SetUserEmail(user.ID, user.Email)

		// Fetch user stores' ID and Title from DB and save them in Redis for quick retrieving in other pages
		var store store.Store
		rows, _ := postgres.DB.Query(`
			SELECT id, title 
			FROM stores 
			WHERE owner_id=$1 AND removed=$2`, user.ID, false)
		defer rows.Close()

		for rows.Next() {
			rows.Scan(&store.ID, &store.Title)

			if store.ID != 0 && store.Title != "" {
				redis.AddUserStore(user.ID, store.ID)
				redis.SetStoreTitle(store.ID, store.Title)
			}
		}

		var (
			sessionTTL   int
			cookieMaxAge int
		)
		if f.RememberMe == "checked" { // User wants session to persist after browser is closed
			sessionTTL = oneMonth
			cookieMaxAge = oneMonth
		} else {
			sessionTTL = oneDay
			cookieMaxAge = 0 // Session cookie deleted when browser is closed
		}
		err = redis.SetSessionExpiration(hashedSessionID, sessionTTL)
		if httperror.Render500IfErr(c, err, "Error setting session's expiration") != nil {
			return
		}

		// Set Session Cookie
		c.SetCookie("DM_SessionID", sessionID, cookieMaxAge, "/", "", false, true) // TODO: Rendere secure da false a true. Capire se sostituire domain da "" a qualcos'altro

		// User signed in successfully therefore gets redirect to Dashboard
		c.Redirect(http.StatusSeeOther, "/dashboard")
		return
	}

	err = rows.Err()
	if httperror.Render500IfErr(c, err, "Error iterating over rows") != nil {
		return
	}

	// If this line is reached, User was not found in DB, therefore render failure page
	e.NoMatch = true
	c.HTML(http.StatusOK, "signin.html", resp)
}

// SignOutHandler handles POST requests to /user/signout
func SignOutHandler(c *gin.Context) {
	// Delete Session ID from Redis
	s := c.MustGet("session").(*auth.Session)
	err := redis.DeleteSession(s.ID)
	if httperror.Render500IfErr(c, err, "Error deleting session from Redis") != nil {
		return
	}

	err = redis.RemoveUserSession(s.UserID, s.ID)
	if httperror.Render500IfErr(c, err, "Error removing user's session ID from Redis") != nil {
		return
	}

	// Delete Session ID cookie
	c.SetCookie("DM_SessionID", "", -1, "/", "", false, true) // TODO: Sistemare secure (e domain?)

	// Redirect to index
	c.Redirect(http.StatusSeeOther, "/")
}

// SignOutAllHandler handles POST requests to /user/signout_all
func SignOutAllHandler(c *gin.Context) {
	// Delete Session ID keys from Redis
	s := c.MustGet("session").(*auth.Session)
	sessionIDs, err := redis.GetUserSessions(s.UserID)
	if httperror.Render500IfErr(c, err, "Error getting user's sessions from Redis") != nil {
		return
	}

	for _, sessionID := range sessionIDs {
		err = redis.DeleteSession(sessionID)
		if httperror.Render500IfErr(c, err, "Error deleting session from Redis") != nil {
			return
		}
	}

	err = redis.DeleteUserSessions(s.UserID)
	if httperror.Render500IfErr(c, err, "Error deleting user's sessions set from Redis") != nil {
		return
	}

	// Delete Session ID cookie
	c.SetCookie("DM_SessionID", "", -1, "/", "", false, true) // TODO: Sistemare secure (e domain?)

	// Redirect to index
	c.Redirect(http.StatusSeeOther, "/")
}

type forgotPasswordFields struct {
	Email string `form:"email" binding:"required,max=64,min=8"`
}

type forgotPasswordData struct {
	Fields  *forgotPasswordFields
	Success bool
}

// ForgotPasswordGetHandler handles GET requests to /user/forgot_password
func ForgotPasswordGetHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "forgot_password.html", nil)
}

// ForgotPasswordPostHandler handles POST requests to /user/forgot_password
func ForgotPasswordPostHandler(c *gin.Context) {
	var (
		f    forgotPasswordFields
		resp = &forgotPasswordData{
			Fields:  &f,
			Success: false,
		}
	)

	// Get email submitted through form
	err := c.ShouldBind(&f)
	if err != nil {
		// Validate email
		errs := err.(validator.ValidationErrors)
		if errs != nil {
			c.HTML(http.StatusUnprocessableEntity, "forgot_password.html", resp)
			return
		}
	}

	// Sanitize input
	user := &User{
		Email: strings.TrimSpace(strings.ToLower(f.Email)),
	}

	// Further validate input
	if !user.HasValidEmail() {
		c.HTML(http.StatusUnprocessableEntity, "forgot_password.html", resp)
		return
	}

	// Check if email is in DB and verified
	err = postgres.DB.QueryRow(`
		SELECT email_verified 
		FROM users 
		WHERE email=$1`, user.Email).
		Scan(&user.Verified)
	if err != nil {
		if err == sql.ErrNoRows { // If email does not exist in DB, render failure page
			c.HTML(http.StatusForbidden, "forgot_password.html", resp)
			return

		}

		httperror.Render500(c, err, "Error querying database")
		return
	}

	if !user.Verified { // If email is not verified, render failure page
		c.HTML(http.StatusForbidden, "forgot_password.html", resp)
		return
	}

	// Generate recover token
	token, err := GenerateUniqueUserToken(recover)
	if httperror.Render500IfErr(c, err, "Error generating recover token") != nil {
		return
	}

	// Update User in DB (Set recover token)
	tx, err := postgres.DB.Begin()
	if httperror.Render500IfErr(c, err, "Error beginning TX") != nil {
		return
	}

	_, err = tx.Exec(`
		UPDATE users 
		SET recover_token=$1, recover_token_expiration_date=NOW()+interval '1 hour' 
		WHERE email=$2`, token, user.Email)
	if httperror.Render500IfErr(c, err, "Error executing query") != nil {
		return
	}

	// TODO: Mandare recovery link ad email
	// Se errore durante invio TX Rollback

	err = tx.Commit()
	if httperror.Render500IfErr(c, err, "Error committing TX") != nil {
		return
	}

	resp.Success = true
	c.HTML(http.StatusOK, "forgot_password.html", resp)
}

type recoverFields struct {
	Token           string `form:"token" binding:"required,len=64"`
	Email           string `form:"email" binding:"required,max=64,min=8"`
	Password        string `form:"password" binding:"required,max=64,min=8"`
	ConfirmPassword string `form:"confirm-password" binding:"required,max=64,min=8"`
}

type recoverErrors struct {
	Token           bool
	TokenExpired    bool
	Email           bool
	Password        bool
	ConfirmPassword bool
	PasswordMatch   bool
}

type recoverData struct {
	Fields  *recoverFields
	Errors  *recoverErrors
	Success bool
}

// RecoverGetHandler handles GET requests to /user/recover
func RecoverGetHandler(c *gin.Context) {
	var (
		f = recoverFields{
			Token: c.Query("token"),
		}
		resp = &recoverData{
			Fields:  &f,
			Errors:  &recoverErrors{},
			Success: false,
		}
	)
	c.HTML(http.StatusOK, "recover.html", resp)
}

// RecoverPostHandler handles POST requests to /user/recover
func RecoverPostHandler(c *gin.Context) {
	var (
		f    recoverFields
		e    recoverErrors
		resp = &recoverData{
			Fields:  &f,
			Errors:  &e,
			Success: false,
		}
	)

	// Get User data submitted through form
	err := c.ShouldBind(&f)
	if err != nil {
		// Validate User input
		errs := err.(validator.ValidationErrors)
		for _, err := range errs {
			switch err.Field() {
			case "Token":
				e.Token = true
			case "Email":
				e.Email = true
			case "Password":
				e.Password = true
			case "ConfirmPassword":
				e.ConfirmPassword = true
			}
		}
		if errs != nil {
			c.HTML(http.StatusUnprocessableEntity, "recover.html", resp)
			return
		}
	}

	// Sanitize input
	user := &User{
		Email:    strings.TrimSpace(strings.ToLower(f.Email)),
		Password: f.Password,
	}

	// Further validate input
	if !user.HasValidEmail() {
		e.Email = true
	}
	if f.Password != f.ConfirmPassword {
		e.PasswordMatch = true
	}
	if e.Email || e.PasswordMatch {
		c.HTML(http.StatusUnprocessableEntity, "recover.html", resp)
		return
	}

	// Check if recover token is valid
	var tokenExpired bool
	err = postgres.DB.QueryRow(`
		SELECT ((recover_token_expiration_date - NOW()) < INTERVAL '0 second') AS token_expired 
		FROM users 
		WHERE email=$1 AND recover_token=$2`, user.Email, f.Token).
		Scan(&tokenExpired)
	if err != nil {
		if err == sql.ErrNoRows {
			// If email or recover token are not found in DB, render failure page
			e.Token = true
			c.HTML(http.StatusNotFound, "recover.html", resp)
			return
		}

		httperror.Render500(c, err, "Error querying database")
		return
	}

	// If recover token has expired, render failure page
	if tokenExpired {
		e.TokenExpired = true
		c.HTML(http.StatusForbidden, "recover.html", resp)
		return
	}

	hashedPassword, err := argon2id.CreateHash(user.Password, argon2id.DefaultParams)
	if httperror.Render500IfErr(c, err, "Error hasing password") != nil {
		return
	}

	// Update user in DB (Set new password and delete recover token)
	_, err = postgres.DB.Exec(`
		UPDATE users 
		SET password=$1, recover_token=NULL, recover_token_expiration_date=NULL 
		WHERE email=$2`, hashedPassword, user.Email)
	if httperror.Render500IfErr(c, err, "Error executing query") != nil {
		return
	}

	// Sign Out from all previous sessions since the password was changed
	s := c.MustGet("session").(*auth.Session)
	sessionIDs, _ := redis.GetUserSessions(s.UserID)
	for _, sessionID := range sessionIDs {
		redis.DeleteSession(sessionID)
	}
	redis.DeleteUserSessions(s.UserID)

	resp.Success = true
	c.HTML(http.StatusOK, "recover.html", resp)
}

type userPutFields struct {
	NewEmail string `json:"newEmail"`
	Password string `json:"password"`

	OldPassword        string `json:"oldPassword"`
	NewPassword        string `json:"newPassword"`
	ConfirmNewPassword string `json:"confirmNewPassword"`
}

// PutHandler handles PUT requests to /user
func PutHandler(c *gin.Context) {
	// Get User data from session
	s := c.MustGet("session").(*auth.Session)

	user := &User{
		ID: s.UserID,
	}

	// Get input submitted by user through JSON body
	var f userPutFields
	err := c.ShouldBindJSON(&f)
	if httperror.Send500IfErr(c, err, "Error binding JSON body to fields") != nil {
		return
	}

	switch {
	case f.NewEmail != "" && f.Password != "": // Change Email
		// Sanitize input
		user.Email = strings.ToLower(strings.TrimSpace(f.NewEmail))
		user.Password = f.Password

		// Validate input
		if !user.HasValidEmail() {
			httperror.Send(c, http.StatusUnprocessableEntity, "New email needs to be between 8 and 64 characters long")
			return
		}
		if !user.HasValidPassword() {
			httperror.Send(c, http.StatusUnprocessableEntity, "Password needs to be between 8 and 64 characters long")
			return
		}

		var (
			oldEmail       string
			hashedPassword string
		)

		// Check if new email is different from the old one
		err := postgres.DB.QueryRow(`
			SELECT email, password 
			FROM users 
			WHERE id=$1`, user.ID).
			Scan(&oldEmail, &hashedPassword)
		if httperror.Send500IfErr(c, err, "Error querying database") != nil {
			return
		}

		if user.Email == oldEmail {
			httperror.Send(c, http.StatusForbidden, "You are already using this email")
			return
		}

		// Check if new email is not used by other users
		_, uniqueEmail, err := user.IsUnique()
		if httperror.Send500IfErr(c, err, "Error checking if user email is unique") != nil {
			return
		}

		if !uniqueEmail {
			httperror.Send(c, http.StatusForbidden, "Email not available")
			return
		}

		// Check if password submitted by user for further authentication is correct
		match, _ := argon2id.ComparePasswordAndHash(user.Password, hashedPassword)
		if !match {
			httperror.Send(c, http.StatusForbidden, "Password is incorrect")
			return
		}

		// Generate new verification token
		token, err := GenerateUniqueUserToken(verification)
		if httperror.Send500IfErr(c, err, "Error generating unique verification token") != nil {
			return
		}

		tx, err := postgres.DB.Begin()
		if httperror.Send500IfErr(c, err, "Error beginning TX") != nil {
			return
		}

		// Update User in DB (Set new email and not verified)
		_, err = tx.Exec(`
			UPDATE users 
			SET email=$1, email_verified=$2, verification_token=$3, verification_token_expiration_date=NOW()+interval '1 hour' 
			WHERE id=$4`, user.Email, false, token, user.ID)
		if httperror.Send500IfErr(c, err, "Error executing query") != nil {
			tx.Rollback()
			return
		}

		// TODO: Mandare email di conferma
		// TODO: Mandare avviso a vecchia email
		// Se qualcosa va storto nel processo, TX Rollback

		err = tx.Commit()
		if httperror.Send500IfErr(c, err, "Error committing TX") != nil {
			return
		}

		// Update User email in Redis
		redis.SetUserEmail(user.ID, user.Email)

		c.Status(http.StatusNoContent)

	case f.OldPassword != "" && f.NewPassword != "" && f.ConfirmNewPassword != "": // Change Password
		user.Password = f.OldPassword

		// Validate input
		if !user.HasValidPassword() {
			httperror.Send(c, http.StatusUnprocessableEntity, "Old password needs to be between 8 and 64 characters long")
			return
		}

		user.Password = f.NewPassword

		if !user.HasValidPassword() {
			httperror.Send(c, http.StatusUnprocessableEntity, "New password needs to be between 8 and 64 characters long")
			return
		}

		if f.NewPassword != f.ConfirmNewPassword {
			httperror.Send(c, http.StatusUnprocessableEntity, "New password and confirm password don't match")
			return
		}

		// Check if old password submitted by user for further authentication is correct
		var oldHashedPassword string
		err := postgres.DB.QueryRow(`
			SELECT password 
			FROM users 
			WHERE id=$1`, user.ID).
			Scan(&oldHashedPassword)
		if httperror.Send500IfErr(c, err, "Error querying databse") != nil {
			return
		}

		match, _ := argon2id.ComparePasswordAndHash(f.OldPassword, oldHashedPassword)
		if !match {
			httperror.Send(c, http.StatusForbidden, "Old password is incorrect")
			return
		}

		// Hash new password
		newHashedPassword, err := argon2id.CreateHash(user.Password, argon2id.DefaultParams)
		if httperror.Send500IfErr(c, err, "Error hashing password") != nil {
			return
		}

		// Update User in DB (Set new password)
		_, err = postgres.DB.Exec(`
			UPDATE users 
			SET password=$1 
			WHERE id=$2`, newHashedPassword, user.ID)
		if httperror.Send500IfErr(c, err, "Error executing query") != nil {
			return
		}

		c.Status(http.StatusNoContent)

	default: // Invalid request
		httperror.Send(c, http.StatusBadRequest, "Bad request")
	}
}
