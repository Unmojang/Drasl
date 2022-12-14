package main

import (
	"gorm.io/gorm"
	"errors"
	"encoding/json"
	"encoding/base64"
	"github.com/labstack/echo/v4"
	"net/http"
	"bytes"
	"crypto/x509"
)

func AuthGetServerInfo(app *App) func (c echo.Context) error {
	publicKeyDer, err := x509.MarshalPKIXPublicKey(&app.Key.PublicKey)
	Check(err)

	infoMap := make(map[string]string)
	infoMap["Status"] = "OK"
	infoMap["RuntimeMode"] = "productionMode"
	infoMap["ApplicationAuthor"] = "Unmojang"
	infoMap["ApplicationDescription"] = ""
	infoMap["SpecificationVersion"] = "2.13.34"
	infoMap["ImplementationVersion"] = "0.1.0"
	infoMap["ApplicationOwner"] = "TODO"
	// TODO multiple public keys
	infoMap["PublicKey"] = base64.StdEncoding.EncodeToString(publicKeyDer)

	infoBlob, err := json.Marshal(infoMap)

	if err != nil {
		panic(err)
	}

	return func (c echo.Context) error {
		return c.JSONBlob(http.StatusOK, infoBlob)
	}
}

func AuthAuthenticate(app *App) func (c echo.Context) error {
	type authenticateRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
		ClientToken *string `json:"clientToken"`
		Agent *Agent `json:"agent"`
		RequestUser bool `json:"requestUser"`
	}

	type authenticateResponse struct {
		AccessToken string `json:"accessToken"`
		ClientToken string `json:"clientToken"`
		SelectedProfile *Profile `json:"selectedProfile,omitempty"`
		AvailableProfiles *[]Profile `json:"availableProfiles,omitempty"`
		User *UserResponse `json:"user,omitempty"`
	}

	invalidCredentialsBlob, err := json.Marshal(ErrorResponse{
		Error: "ForbiddenOperationException",
		ErrorMessage: "Invalid credentials. Invalid username or password.",
	})
	if err != nil {
		panic(err)
	}

	return func(c echo.Context) (err error) {
		req := new(authenticateRequest)
		if err = c.Bind(req); err != nil {
			return err
		}

		var user User
		result := app.DB.Preload("TokenPairs").First(&user, "username = ?", req.Username)
		if result.Error != nil {
			return result.Error
		}

		passwordHash, err := HashPassword(req.Password, user.PasswordSalt)
		if err != nil {
			return err
		}

		if !bytes.Equal(passwordHash, user.PasswordHash) {
			return c.JSONBlob(http.StatusUnauthorized, invalidCredentialsBlob)
		}

		accessToken, err := RandomHex(16)
		if err != nil {
			return err
		}

		var clientToken string
		if req.ClientToken == nil {
			clientToken, err := RandomHex(16)
			if err != nil {
				return err
			}
			user.TokenPairs = append(user.TokenPairs, TokenPair{
				ClientToken: clientToken,
				AccessToken: accessToken,
				Valid: true,
			})
		} else {
			clientToken = *req.ClientToken
			clientTokenExists := false
			for i := range user.TokenPairs {
				if user.TokenPairs[i].ClientToken == clientToken {
					clientTokenExists = true
					user.TokenPairs[i].AccessToken = accessToken
					user.TokenPairs[i].Valid = true
					break;
				}
			}

			if !clientTokenExists {
				user.TokenPairs = append(user.TokenPairs, TokenPair{
					ClientToken: clientToken,
					AccessToken: accessToken,
					Valid: true,
				})
			}
		}

		result = app.DB.Save(&user)
		if result.Error != nil {
			return result.Error
		}

		var selectedProfile *Profile
		var availableProfiles *[]Profile
		if req.Agent != nil {
			id, err := UUIDToID(user.UUID)
			if err != nil {
				return err
			}
			selectedProfile = &Profile{
				ID: id,
				Name: user.PlayerName,
			}
			availableProfiles = &[]Profile{*selectedProfile}
		}

		var userResponse *UserResponse
		if req.RequestUser {
			id, err := UUIDToID(user.UUID)
			if err != nil {
				return err
			}
			userResponse = &UserResponse{
				ID: id,
				Properties: []UserProperty{UserProperty{
					Name: "preferredLanguage",
					Value: user.PreferredLanguage,
				}},
			}
		}

		res := authenticateResponse{
			ClientToken: clientToken,
			AccessToken: accessToken,
			SelectedProfile: selectedProfile,
			AvailableProfiles: availableProfiles,
			User: userResponse,
		}
		return c.JSON(http.StatusOK, &res)
	}
}

type UserProperty struct {
	Name string `json:"name"`
	Value string `json:"value"`
}
type UserResponse struct {
	ID string `json:"id"`
	Properties []UserProperty `json:"properties"`
}

func AuthRefresh(app *App) func (c echo.Context) error { 
	type refreshRequest struct {
		AccessToken string `json:"accessToken"`
		ClientToken string `json:"clientToken"`
		RequestUser bool `json:"requestUser"`
	}

	type refreshResponse struct {
		AccessToken string `json:"accessToken"`
		ClientToken string `json:"clientToken"`
		SelectedProfile Profile `json:"selectedProfile,omitempty"`
		AvailableProfiles []Profile `json:"availableProfiles,omitempty"`
		User *UserResponse `json:"user,omitempty"`
	}

	invalidAccessTokenBlob, err := json.Marshal(ErrorResponse{
		Error: "TODO",
		ErrorMessage: "TODO",
	})
	if err != nil {
		panic(err)
	}

	return func(c echo.Context) error {
		req := new(refreshRequest)
		if err := c.Bind(req); err != nil {
			return err
		}

		var tokenPair TokenPair
		result := app.DB.Preload("User").First(&tokenPair, "client_token = ?", req.ClientToken)
		if result.Error != nil {
			if errors.Is(result.Error, gorm.ErrRecordNotFound) {
				return c.NoContent(http.StatusUnauthorized)
			}
			return result.Error
		}
		user := tokenPair.User

		if req.AccessToken != tokenPair.AccessToken {
			return c.JSONBlob(http.StatusUnauthorized, invalidAccessTokenBlob)
		}

		accessToken, err := RandomHex(16)
		if err != nil {
			return err
		}
		tokenPair.AccessToken = accessToken
		tokenPair.Valid = true
		
		result = app.DB.Save(&tokenPair)
		if result.Error != nil {
			return result.Error
		}

		id, err := UUIDToID(user.UUID)
		if err != nil {
			return err
		}
		selectedProfile := Profile{
			ID: id,
			Name: user.PlayerName,
		}
		availableProfiles := []Profile{selectedProfile}

		var userResponse *UserResponse
		if req.RequestUser {
			id, err := UUIDToID(user.UUID)
			if err != nil {
				return err
			}
			userResponse = &UserResponse{
				ID: id,
				Properties: []UserProperty{{
					Name: "preferredLanguage",
					Value: user.PreferredLanguage,
				}},
			}
		}

		res := refreshResponse{
			AccessToken: tokenPair.AccessToken,
			ClientToken: tokenPair.ClientToken,
			SelectedProfile: selectedProfile,
			AvailableProfiles: availableProfiles,
			User: userResponse,
		}

		return c.JSON(http.StatusOK, &res)
	}
}


func AuthValidate(app *App) func (c echo.Context) error { 
	type validateRequest struct {
		AccessToken string `json:"accessToken"`
		ClientToken string `json:"clientToken"`
	}
	return func(c echo.Context) error {
		req := new(validateRequest)
		if err := c.Bind(req); err != nil {
			return err
		}

		var tokenPair TokenPair
		result := app.DB.First(&tokenPair, "client_token = ?", req.ClientToken)
		if result.Error != nil {
			return c.NoContent(http.StatusForbidden)
		}

		if !tokenPair.Valid || req.AccessToken != tokenPair.AccessToken {
			return c.NoContent(http.StatusForbidden)
		}

		return c.NoContent(http.StatusNoContent)
	}
}

func AuthSignout(app *App) func (c echo.Context) error { 
	type signoutRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	invalidCredentialsBlob, err := json.Marshal(ErrorResponse{
		Error: "ForbiddenOperationException",
		ErrorMessage: "Invalid credentials. Invalid username or password.",
	})
	if err != nil {
		panic(err)
	}

	return func(c echo.Context) error {
		req := new(signoutRequest)
		if err := c.Bind(req); err != nil {
			return err
		}

		var user User
		result := app.DB.First(&user, "username = ?", req.Username)
		if result.Error != nil {
			return result.Error
		}

		passwordHash, err := HashPassword(req.Password, user.PasswordSalt)
		if err != nil {
			return err
		}

		if !bytes.Equal(passwordHash, user.PasswordHash) {
			return c.JSONBlob(http.StatusUnauthorized, invalidCredentialsBlob)
		}

		app.DB.Model(TokenPair{}).Where("user_uuid = ?", user.UUID).Updates(TokenPair{Valid: false})

		if result.Error != nil {
			return result.Error
		}
		
		return c.NoContent(http.StatusNoContent)
	}
}

func AuthInvalidate(app *App) func (c echo.Context) error { 
	type invalidateRequest struct {
		AccessToken string `json:"accessToken"`
		ClientToken string `json:"clientToken"`
	}

	return func(c echo.Context) error {
		req := new(invalidateRequest)
		if err := c.Bind(req); err != nil {
			return err
		}

		var tokenPair TokenPair
		result := app.DB.First(&tokenPair, "client_token = ?", req.ClientToken)
		if result.Error != nil {
			// TODO handle not found?
			return result.Error
		}
		app.DB.Table("token_pairs").Where("user_uuid = ?", tokenPair.UserUUID).Updates(map[string]interface{}{"Valid": false})

		return c.NoContent(http.StatusNoContent)
	}
}
