package main

import "github.com/labstack/echo"

//TokenAuth type
type TokenAuth struct{}

// NewTokenAuth function
func NewTokenAuth() (*TokenAuth, error) {
	return &TokenAuth{}, nil
}

//Start function
func (a *TokenAuth) Start() {

}

//Login function
func (a *TokenAuth) Login(username string, password string) bool {
	return true
}

//Validate function
func (a *TokenAuth) Validate(creds map[string]string) bool {
	return true
}

//Update function
func (a *TokenAuth) Update(username string, password string) error {
	return nil
}

//Create function
func (a *TokenAuth) Create(username string, password string) error {
	return nil
}

//Delete function
func (a *TokenAuth) Delete(username string) error {
	return nil
}

//Middleware func
func (a *TokenAuth) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}
