package main

import "github.com/labstack/echo"

//NoAuth type is no auth
type NoAuth struct{}

// NewNoAuth returns a new instance of the NoAuth service
func NewNoAuth() (*NoAuth, error) {
	return &NoAuth{}, nil
}

//Start function
func (a *NoAuth) Start() {
	return
}

//Login function
func (a *NoAuth) Login(username string, password string) bool {
	return true
}

//Validate function
func (a *NoAuth) Validate(map[string]string) bool {
	return true
}

//Update function
func (a *NoAuth) Update(username string, password string) error {
	return nil
}

//Create function
func (a *NoAuth) Create(username string, password string) error {
	return nil
}

//Delete function
func (a *NoAuth) Delete(username string) error {
	return nil
}

//Middleware func
func (a *NoAuth) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {

		return next(c)
	}
}
