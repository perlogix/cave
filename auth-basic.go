package main

import "github.com/labstack/echo"

//BasicAuth type is no auth
type BasicAuth struct{}

// NewBasicAuth returns a new instance of the BasicAuth service
func NewBasicAuth() (*BasicAuth, error) {
	return &BasicAuth{}, nil
}

//Start function
func (a *BasicAuth) Start() {

}

//Login function
func (a *BasicAuth) Login(username string, password string) bool {
	return true
}

//Validate function
func (a *BasicAuth) Validate(creds map[string]string) bool {
	return true
}

//Update function
func (a *BasicAuth) Update(username string, password string) error {
	return nil
}

//Create function
func (a *BasicAuth) Create(username string, password string) error {
	return nil
}

//Delete function
func (a *BasicAuth) Delete(username string) error {
	return nil
}

//Middleware func
func (a *BasicAuth) Middleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		return next(c)
	}
}
