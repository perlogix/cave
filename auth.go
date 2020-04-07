package main

import "github.com/labstack/echo"

//AuthService struct
type AuthService struct {
	terminate chan bool
	config    *Config
	log       *Log
	kv        *KV
	provider  Authenticator
}

//Authenticator interface
type Authenticator interface {
	Start()
	Login(username string, password string) bool
	Validate(creds map[string]string) bool
	Update(username string, password string) error
	Create(username string, password string) error
	Delete(username string) error
	Middleware(echo.HandlerFunc) echo.HandlerFunc
}

//NewAuth returns a new instance of the AuthService
func NewAuth(app *Bunker) (*AuthService, error) {
	var provider Authenticator
	var err error
	switch app.Config.Auth.Provider {
	case "token":
		provider, err = NewTokenAuth()
		if err != nil {
			return nil, err
		}
	case "basic":
		provider, err = NewBasicAuth()
		if err != nil {
			return nil, err
		}
	case "none":
		provider, err = NewNoAuth()
		if err != nil {
			return nil, err
		}
	default:
		provider, err = NewNoAuth()
		if err != nil {
			return nil, err
		}
	}
	auth := &AuthService{
		terminate: make(chan bool, 1),
		log:       app.Logger,
		kv:        app.KV,
		config:    app.Config,
		provider:  provider,
	}
	go auth.provider.Start()
	return auth, nil
}

// ValidateUser function
func (a *AuthService) ValidateUser(creds map[string]string) bool {
	b := a.provider.Validate(creds)
	return b
}

//CreateUser function
func (a *AuthService) CreateUser(username string, password string) error {
	err := a.provider.Create(username, password)
	if err != nil {
		return err
	}
	return nil
}

//DeleteUser function
func (a *AuthService) DeleteUser(username string) error {
	err := a.provider.Delete(username)
	if err != nil {
		return err
	}
	return nil
}

//ChangePassword function
func (a *AuthService) ChangePassword(username string, password string) error {
	err := a.provider.Update(username, password)
	if err != nil {
		return err
	}
	return nil
}

// Middleware function
func (a *AuthService) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return a.provider.Middleware(next)
}
