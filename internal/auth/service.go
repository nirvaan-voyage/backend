package auth

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
	"nirvaan-backend/internal/models"
)

var (
	emailRegex    = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)
	usernameRegex = regexp.MustCompile(`^[A-Za-z0-9_]{3,20}$`)
	uppercaseRe   = regexp.MustCompile(`[A-Z]`)
	lowercaseRe   = regexp.MustCompile(`[a-z]`)
	numberRe      = regexp.MustCompile(`[0-9]`)
)

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
		return models.AuthResponse{}, err
	}

	token, err := s.jwt.Generate(user)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("generate jwt: %w", err)
	}

	return models.AuthResponse{Token: token, User: user}, nil
}

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
	if len(req.Password) < 8 || !uppercaseRe.MatchString(req.Password) ||
		!lowercaseRe.MatchString(req.Password) || !numberRe.MatchString(req.Password) {
		return FieldError{
			Message: "Password must be at least 8 characters and include uppercase, lowercase, and number",
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
		RETURNING id, name, username, email, phone, age, password_hash, created_at, updated_at
	`

	var user models.User
	var age int
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
		&user.PasswordHash,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return models.User{}, errors.New("user was not created")
	}
	user.Age = &age
	return user, err
}
