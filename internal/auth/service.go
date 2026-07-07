package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"nirvaan-backend/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const userSelectColumns = "id, name, username, email, phone, age, profile_photo_path, password_hash, created_at, updated_at"

var (
	emailRegex    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	usernameRegex = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)
	uppercaseRe   = regexp.MustCompile(`[A-Z]`)
	lowercaseRe   = regexp.MustCompile(`[a-z]`)
	numberRe      = regexp.MustCompile(`[0-9]`)
)

// FieldError is returned when a specific request field fails validation.
// The Flutter client uses Field to highlight the right input.
type FieldError struct {
	Message     string
	Field       string
	Suggestions []string
}

func (e FieldError) Error() string {
	return e.Message
}

type Service struct {
	db  *pgxpool.Pool
	jwt JWTService
}

func NewService(db *pgxpool.Pool, jwt JWTService) Service {
	return Service{db: db, jwt: jwt}
}

func (s Service) Register(ctx context.Context, req models.RegisterRequest) (models.AuthResponse, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Phone = strings.TrimSpace(req.Phone)

	if err := validateRegisterRequest(req); err != nil {
		return models.AuthResponse{}, err
	}

	if exists, err := s.exists(ctx, "username", req.Username); err != nil {
		return models.AuthResponse{}, err
	} else if exists {
		return models.AuthResponse{}, FieldError{
			Message:     "Username is already taken",
			Field:       "username",
			Suggestions: s.usernameSuggestions(ctx, req.Username),
		}
	}

	if exists, err := s.exists(ctx, "email", req.Email); err != nil {
		return models.AuthResponse{}, err
	} else if exists {
		return models.AuthResponse{}, FieldError{
			Message: "Email is already registered",
			Field:   "email",
		}
	}

	if exists, err := s.exists(ctx, "phone", req.Phone); err != nil {
		return models.AuthResponse{}, err
	} else if exists {
		return models.AuthResponse{}, FieldError{
			Message: "Phone number is already registered",
			Field:   "phone",
		}
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("hash password: %w", err)
	}

	user, err := s.insertUser(ctx, req, string(passwordHash))
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return models.AuthResponse{}, FieldError{
				Message: "User already exists",
				Field:   "username",
			}
		}
		fmt.Printf("ERROR inserting user: %v\n", err)
		return models.AuthResponse{}, err
	}

	token, err := s.jwt.Generate(user)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("generate jwt: %w", err)
	}

	return models.AuthResponse{Token: token, User: user}, nil
}

// ── Login ──────────────────────────────────────────────────────────────────

// Login accepts either email or username in the emailOrUsername field,
// matching the Flutter client's LoginRequest payload.
func (s Service) Login(ctx context.Context, req models.LoginRequest) (models.AuthResponse, error) {
	identifier := strings.ToLower(strings.TrimSpace(req.EmailOrUsername))
	if identifier == "" {
		return models.AuthResponse{}, FieldError{
			Message: "Email or username is required",
			Field:   "emailOrUsername",
		}
	}
	if req.Password == "" {
		return models.AuthResponse{}, FieldError{
			Message: "Password is required",
			Field:   "password",
		}
	}

	// Determine whether the identifier looks like an email or a username
	var user models.User
	var err error
	if strings.Contains(identifier, "@") {
		user, err = s.findByField(ctx, "email", identifier)
	} else {
		user, err = s.findByField(ctx, "username", identifier)
	}

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.AuthResponse{}, FieldError{
				Message: "No account found with that email or username",
				Field:   "emailOrUsername",
			}
		}
		return models.AuthResponse{}, fmt.Errorf("lookup user: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return models.AuthResponse{}, FieldError{
			Message: "Incorrect password",
			Field:   "password",
		}
	}

	token, err := s.jwt.Generate(user)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("generate jwt: %w", err)
	}

	return models.AuthResponse{Token: token, User: user}, nil
}

func (s Service) GoogleAuth(ctx context.Context, req models.GoogleAuthRequest) (models.AuthResponse, error) {
	idToken := strings.TrimSpace(req.IDToken)
	if idToken == "" {
		return models.AuthResponse{}, FieldError{Message: "Google token is required", Field: "idToken"}
	}

	googleUser, err := verifyGoogleIDToken(ctx, idToken)
	if err != nil {
		return models.AuthResponse{}, FieldError{Message: "Invalid Google token", Field: "idToken"}
	}

	email := strings.ToLower(strings.TrimSpace(googleUser.Email))
	if email == "" {
		return models.AuthResponse{}, FieldError{Message: "Google account did not return an email", Field: "email"}
	}

	user, err := s.findByField(ctx, "email", email)
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return models.AuthResponse{}, fmt.Errorf("lookup google user: %w", err)
		}

		user, err = s.insertGoogleUser(ctx, googleUser)
		if err != nil {
			return models.AuthResponse{}, fmt.Errorf("insert google user: %w", err)
		}
	}

	token, err := s.jwt.Generate(user)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("generate jwt: %w", err)
	}

	return models.AuthResponse{Token: token, User: user}, nil
}

func (s Service) CompleteProfile(ctx context.Context, userID int, req models.CompleteProfileRequest) (models.AuthResponse, error) {
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	req.Phone = strings.TrimSpace(req.Phone)
	var profilePhotoPath *string
	if req.ProfilePhotoPath != nil {
		trimmed := strings.TrimSpace(*req.ProfilePhotoPath)
		if trimmed != "" {
			profilePhotoPath = &trimmed
		}
	}

	if !usernameRegex.MatchString(req.Username) {
		return models.AuthResponse{}, FieldError{
			Message: "Username must be 3-20 characters and use only letters, numbers, and underscores",
			Field:   "username",
		}
	}
	if len(req.Phone) != 10 || !regexp.MustCompile(`^[0-9]{10}$`).MatchString(req.Phone) {
		return models.AuthResponse{}, FieldError{Message: "Phone number must be 10 digits", Field: "phone"}
	}
	if req.Age == nil || *req.Age < 1 || *req.Age > 120 {
		return models.AuthResponse{}, FieldError{Message: "Enter a valid age", Field: "age"}
	}

	if exists, err := s.existsForOtherUser(ctx, "username", req.Username, userID); err != nil {
		return models.AuthResponse{}, err
	} else if exists {
		return models.AuthResponse{}, FieldError{
			Message:     "Username is already taken",
			Field:       "username",
			Suggestions: s.usernameSuggestions(ctx, req.Username),
		}
	}
	if exists, err := s.existsForOtherUser(ctx, "phone", req.Phone, userID); err != nil {
		return models.AuthResponse{}, err
	} else if exists {
		return models.AuthResponse{}, FieldError{Message: "Phone number is already registered", Field: "phone"}
	}

	const query = `
		UPDATE users
		SET username = $1,
			phone = $2,
			age = $3,
			profile_photo_path = COALESCE($4, profile_photo_path),
			updated_at = NOW()
		WHERE id = $5
		RETURNING id, name, username, email, phone, age, profile_photo_path, password_hash, created_at, updated_at
	`

	var user models.User
	var age sql.NullInt64
	var photo sql.NullString
	err := s.db.QueryRow(ctx, query, req.Username, req.Phone, req.Age, profilePhotoPath, userID).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Email,
		&user.Phone,
		&age,
		&photo,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.AuthResponse{}, FieldError{Message: "User not found", Field: "token"}
		}
		return models.AuthResponse{}, fmt.Errorf("update profile: %w", err)
	}
	applyNullableUserFields(&user, age, photo)

	token, err := s.jwt.Generate(user)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("generate jwt: %w", err)
	}

	return models.AuthResponse{Token: token, User: user}, nil
}

// ── Password Reset ─────────────────────────────────────────────────────────

func (s Service) ResetPassword(ctx context.Context, identifier, newPassword string) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	identifier = strings.TrimSpace(identifier)

	_, err = s.db.Exec(
		ctx,
		`UPDATE users
		 SET password_hash = $1,
		     updated_at = NOW()
		 WHERE LOWER(email) = LOWER($2)
		    OR phone = $2`,
		string(passwordHash),
		identifier,
	)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	return nil
}

// ── Validation ─────────────────────────────────────────────────────────────

func validateRegisterRequest(req models.RegisterRequest) error {
	if req.Name == "" {
		return FieldError{Message: "Name is required", Field: "name"}
	}
	if !usernameRegex.MatchString(req.Username) {
		return FieldError{
			Message: "Username must be 3-20 characters and use only letters, numbers, and underscores",
			Field:   "username",
		}
	}
	if !emailRegex.MatchString(req.Email) {
		return FieldError{Message: "Enter a valid email address", Field: "email"}
	}
	if req.Age == nil {
		return FieldError{Message: "Age is required", Field: "age"}
	}
	if len(req.Phone) != 10 || !regexp.MustCompile(`^[0-9]{10}$`).MatchString(req.Phone) {
		return FieldError{Message: "Phone number must be 10 digits", Field: "phone"}
	}
	if len(req.Password) < 6 {
		return FieldError{
			Message: "Password must be at least 6 characters",
			Field:   "password",
		}
	}
	return nil
}

func (s Service) exists(ctx context.Context, field, value string) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM users WHERE %s = $1)", field)
	var exists bool
	if err := s.db.QueryRow(ctx, query, value).Scan(&exists); err != nil {
		return false, fmt.Errorf("check %s exists: %w", field, err)
	}
	return exists, nil
}

func (s Service) existsForOtherUser(ctx context.Context, field, value string, userID int) (bool, error) {
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM users WHERE %s = $1 AND id <> $2)", field)
	var exists bool
	if err := s.db.QueryRow(ctx, query, value, userID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check %s exists for other user: %w", field, err)
	}
	return exists, nil
}

func (s Service) findByField(ctx context.Context, field, value string) (models.User, error) {
	query := fmt.Sprintf(`
		SELECT %s
		FROM users
		WHERE %s = $1
		LIMIT 1
	`, userSelectColumns, field)

	var user models.User
	var age sql.NullInt64
	var photo sql.NullString
	err := s.db.QueryRow(ctx, query, value).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Email,
		&user.Phone,
		&age,
		&photo,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return models.User{}, err
	}
	applyNullableUserFields(&user, age, photo)
	return user, nil
}

func applyNullableUserFields(user *models.User, age sql.NullInt64, photo sql.NullString) {
	if age.Valid {
		value := int(age.Int64)
		user.Age = &value
	}
	if photo.Valid {
		value := strings.TrimSpace(photo.String)
		if value != "" {
			user.ProfilePhotoPath = &value
		}
	}
}

func (s Service) usernameSuggestions(ctx context.Context, base string) []string {
	candidates := []string{
		fmt.Sprintf("%s_%d", base, 1),
		fmt.Sprintf("%s_%d", base, 24),
		fmt.Sprintf("%s_%d", base, 101),
		fmt.Sprintf("%s_app", base),
		fmt.Sprintf("the_%s", base),
	}

	suggestions := make([]string, 0, 3)
	for _, candidate := range candidates {
		if len(candidate) > 20 || !usernameRegex.MatchString(candidate) {
			continue
		}
		exists, err := s.exists(ctx, "username", candidate)
		if err == nil && !exists {
			suggestions = append(suggestions, candidate)
		}
		if len(suggestions) == 3 {
			return suggestions
		}
	}

	for i := 1000; len(suggestions) < 3 && i < 9999; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if len(candidate) > 20 {
			candidate = fmt.Sprintf("%s%d", base[:min(len(base), 12)], i)
		}
		exists, err := s.exists(ctx, "username", candidate)
		if err == nil && !exists {
			suggestions = append(suggestions, candidate)
		}
	}

	return suggestions
}

func (s Service) insertUser(ctx context.Context, req models.RegisterRequest, passwordHash string) (models.User, error) {
	const query = `
		INSERT INTO users (name, username, email, phone, age, password_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, username, email, phone, age, profile_photo_path, password_hash, created_at, updated_at
	`

	var user models.User
	var age sql.NullInt64
	var photo sql.NullString
	err := s.db.QueryRow(
		ctx,
		query,
		req.Name,
		req.Username,
		req.Email,
		req.Phone,
		req.Age,
		passwordHash,
	).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Email,
		&user.Phone,
		&age,
		&photo,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, errors.New("user was not created")
	}
	applyNullableUserFields(&user, age, photo)
	return user, err
}

type googleTokenInfo struct {
	Audience      string      `json:"aud"`
	Email         string      `json:"email"`
	EmailVerified interface{} `json:"email_verified"`
	Name          string      `json:"name"`
}

func verifyGoogleIDToken(ctx context.Context, idToken string) (googleTokenInfo, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://oauth2.googleapis.com/tokeninfo?id_token="+idToken,
		nil,
	)
	if err != nil {
		return googleTokenInfo{}, err
	}

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return googleTokenInfo{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return googleTokenInfo{}, fmt.Errorf("google tokeninfo status: %d", resp.StatusCode)
	}

	var info googleTokenInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return googleTokenInfo{}, err
	}

	expectedAudience := strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID"))
	if expectedAudience != "" && info.Audience != expectedAudience {
		return googleTokenInfo{}, fmt.Errorf("google token audience mismatch")
	}

	switch verified := info.EmailVerified.(type) {
	case bool:
		if !verified {
			return googleTokenInfo{}, fmt.Errorf("google email is not verified")
		}
	case string:
		if strings.ToLower(verified) != "true" {
			return googleTokenInfo{}, fmt.Errorf("google email is not verified")
		}
	default:
		return googleTokenInfo{}, fmt.Errorf("google email verification missing")
	}

	return info, nil
}

func (s Service) insertGoogleUser(ctx context.Context, googleUser googleTokenInfo) (models.User, error) {
	email := strings.ToLower(strings.TrimSpace(googleUser.Email))
	name := strings.TrimSpace(googleUser.Name)
	if name == "" {
		name = strings.Split(email, "@")[0]
	}

	usernameBase := sanitizeUsername(strings.Split(email, "@")[0])
	username := s.firstAvailableUsername(ctx, usernameBase)
	phone := s.googlePlaceholderPhone(ctx)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("google-auth-user"), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, err
	}

	req := models.RegisterRequest{
		Name:     name,
		Username: username,
		Email:    email,
		Phone:    phone,
		Age:      nil,
	}

	const query = `
		INSERT INTO users (name, username, email, phone, age, password_hash)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, name, username, email, phone, age, profile_photo_path, password_hash, created_at, updated_at
	`

	var user models.User
	var age sql.NullInt64
	var photo sql.NullString
	err = s.db.QueryRow(
		ctx,
		query,
		req.Name,
		req.Username,
		req.Email,
		req.Phone,
		req.Age,
		string(passwordHash),
	).Scan(
		&user.ID,
		&user.Name,
		&user.Username,
		&user.Email,
		&user.Phone,
		&age,
		&photo,
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	applyNullableUserFields(&user, age, photo)
	return user, err
}

func sanitizeUsername(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9_]`).ReplaceAllString(value, "_")
	value = strings.Trim(value, "_")
	if len(value) < 3 {
		value = "user_" + value
	}
	if len(value) > 14 {
		value = value[:14]
	}
	return value
}

func (s Service) firstAvailableUsername(ctx context.Context, base string) string {
	for i := 0; i < 10000; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s_%d", base, i)
		}
		if len(candidate) > 20 {
			candidate = candidate[:20]
		}
		exists, err := s.exists(ctx, "username", candidate)
		if err == nil && !exists {
			return candidate
		}
	}
	return fmt.Sprintf("user_%d", time.Now().Unix())
}

func (s Service) googlePlaceholderPhone(ctx context.Context) string {
	for i := 0; i < 100000; i++ {
		candidate := fmt.Sprintf("g%014d", time.Now().UnixNano()%100000000000000+int64(i))
		exists, err := s.exists(ctx, "phone", candidate)
		if err == nil && !exists {
			return candidate
		}
	}
	return fmt.Sprintf("g%d", time.Now().UnixNano()%100000000000000)
}
