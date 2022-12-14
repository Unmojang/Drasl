package main

import (
	"fmt"
	"database/sql"
	"errors"
	"golang.org/x/crypto/scrypt"
	"strings"
)

const (
	SkinModelSlim    string = "slim"
	SkinModelClassic string = "classic"
)

func MakeNullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{Valid: false}
	}
	new_string := *s
	return sql.NullString{
		String: new_string,
		Valid:  true,
	}
}

func UnmakeNullString(ns *sql.NullString) *string {
	if ns.Valid {
		new_string := ns.String
		return &new_string
	}
	return nil
}

func IsValidSkinModel(model string) bool {
	switch model {
	case SkinModelSlim, SkinModelClassic:
		return true
	default:
		return false
	}
}

func UUIDToID(uuid string) (string, error) {
	if len(uuid) != 36 {
		return "", errors.New("Invalid UUID")
	}
	return strings.ReplaceAll(uuid, "-", ""), nil
}

func IDToUUID(id string) (string, error) {
	if len(id) != 32 {
		return "", errors.New("Invalid ID")
	}
	return id[0:8] + "-" + id[8:12] + "-" + id[12:16] + "-" + id[16:20] + "-" + id[20:], nil
}

func ValidatePlayerName(playerName string) error {
	maxLength := 16
	if playerName == "" {
		return errors.New("can't be blank")
	}
	if len(playerName) > maxLength {
		return errors.New(fmt.Sprintf("can't be longer than %d characters", maxLength))
	}
	return nil
}

func ValidateUsername(username string) error {
	return ValidatePlayerName(username)
}

func ValidatePassword(password string) error {
	if password == "" {
		return errors.New("can't be blank")
	}
	return nil
}

func IsValidPreferredLanguage(preferredLanguage string) bool {
	switch preferredLanguage {
	case "sq",
		"ar",
		"be",
		"bg",
		"ca",
		"zh",
		"hr",
		"cs",
		"da",
		"nl",
		"en",
		"et",
		"fi",
		"fr",
		"de",
		"el",
		"iw",
		"hi",
		"hu",
		"is",
		"in",
		"ga",
		"it",
		"ja",
		"ko",
		"lv",
		"lt",
		"mk",
		"ms",
		"mt",
		"no",
		"nb",
		"nn",
		"pl",
		"pt",
		"ro",
		"ru",
		"sr",
		"sk",
		"sl",
		"es",
		"sv",
		"th",
		"tr",
		"uk",
		"vi":
		return true
	default:
		return false
	}
}

const SCRYPT_N = 32768
const SCRYPT_r = 8
const SCRYPT_p = 1
const SCRYPT_BYTES = 32

func HashPassword(password string, salt []byte) ([]byte, error) {
	return scrypt.Key(
		[]byte(password),
		salt,
		SCRYPT_N,
		SCRYPT_r,
		SCRYPT_p,
		SCRYPT_BYTES,
	)
}

func SkinURL(app *App, hash string) string {
	return app.Config.FrontEndServer.URL + "/texture/skin/" + hash + ".png"
}

func CapeURL(app *App, hash string) string {
	return app.Config.FrontEndServer.URL + "/texture/cape/" + hash + ".png"
}

type TokenPair struct {
	ClientToken string `gorm:"primaryKey"`
	AccessToken string `gorm:"index"`
	Valid       bool   `gorm:"not null"`
	UserUUID    string
	User        User
}

type User struct {
	UUID              string      `gorm:"primaryKey"`
	Username          string      `gorm:"unique;not null"`
	PasswordSalt      []byte      `gorm:"not null"`
	PasswordHash      []byte      `gorm:"not null"`
	TokenPairs        []TokenPair `gorm:"foreignKey:UserUUID"`
	ServerID          sql.NullString
	PlayerName        string `gorm:"unique"`
	PreferredLanguage string
	BrowserToken      sql.NullString `gorm:"index"`
	SkinHash          sql.NullString `gorm:"index"`
	SkinModel         string
	CapeHash          sql.NullString `gorm:"index"`
}
