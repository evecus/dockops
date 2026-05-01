package auth

import (
	"errors"
	"time"

	"github.com/dockops/dockops/internal/db"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const jwtSecret = "dockops-secret-change-in-prod"

type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Admin struct {
	ID           int    `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
}

func CreateAdmin(database *db.DB, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = database.Exec(`INSERT INTO admin (username, password_hash) VALUES (?, ?)`, username, string(hash))
	return err
}

func UpdateAdmin(database *db.DB, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = database.Exec(`UPDATE admin SET username = ?, password_hash = ? WHERE id = 1`, username, string(hash))
	return err
}

func Login(database *db.DB, username, password string) (string, error) {
	var admin Admin
	err := database.QueryRow(`SELECT id, username, password_hash FROM admin WHERE username = ?`, username).
		Scan(&admin.ID, &admin.Username, &admin.PasswordHash)
	if err != nil {
		return "", errors.New("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid credentials")
	}

	claims := &Claims{
		Username: admin.Username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(jwtSecret))
}

func ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

func GetAdmin(database *db.DB) (*Admin, error) {
	var admin Admin
	err := database.QueryRow(`SELECT id, username, password_hash FROM admin WHERE id = 1`).
		Scan(&admin.ID, &admin.Username, &admin.PasswordHash)
	return &admin, err
}
