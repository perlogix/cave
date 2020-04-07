package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"

	"github.com/labstack/echo"
	"go.etcd.io/bbolt"
)

// TODO: API stuff here

const (
	// APIPREFIX path
	APIPREFIX = "/api/v1/"

	// KVPREFIX path
	KVPREFIX = "/api/v1/kv/"

	// UIPREFIX path
	UIPREFIX = "/ui"

	// SYSPREFIX path
	SYSPREFIX = "/system"
)

type jsonError struct {
	Message string `json:"message,omitempty"`
}

// API Type
type API struct {
	app       *Bunker
	config    *Config
	log       *Log
	terminate chan bool
	kv        *KV
	http      *echo.Echo
	auth      *AuthService
}

//NewAPI function
func NewAPI(app *Bunker) (*API, error) {
	a := &API{
		app:    app,
		config: app.Config,
		log:    app.Logger,
		kv:     app.KV,
		auth:   app.Auth,
	}
	a.terminate = make(chan bool)
	a.http = echo.New()
	a.http.HideBanner = true
	//a.http.Use(middleware.Recover())
	a.http.Use(a.log.EchoLogger())

	a.http.Any(KVPREFIX+"*", a.kvHandler, a.auth.Middleware)
	a.http.POST(APIPREFIX+"login", a.routeLogin)
	a.http.Static(UIPREFIX+"*", "./ui/")

	a.http.HidePort = true
	a.http.Debug = true
	return a, nil
}

// Start starts a new server
func (a *API) Start() {
	go a.watch()
	scheme := "http://"
	if a.config.SSL.Enable {
		scheme = "https://"
	}
	a.log.InfoF("API listening on %s0.0.0.0:%v", scheme, a.config.API.Port)
	a.http.Start(fmt.Sprintf("0.0.0.0:%v", a.config.API.Port))
}

func (a *API) watch() {
	for {
		select {
		case <-a.terminate:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			err := a.http.Shutdown(ctx)
			if err != nil {
				a.log.Error(err)
			}

			return
		default:
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (a *API) routeLogin(c echo.Context) error {
	return c.JSON(200, map[string]string{"message": "ok"})
}

func (a *API) kvHandler(c echo.Context) error {
	switch c.Request().Method {
	case "GET":
		return a.kvGetHandler(c)
	case "POST":
		return a.kvPutHandler(c)
	case "DELETE":
		return a.kvPutHandler(c)
	default:
		return c.JSON(405, jsonError{Message: "Method " + c.Request().Method + " is not allowed"})
	}
}

func (a *API) treeHandler(c echo.Context, path string) error {
	tree, err := a.kv.GetTree(strings.TrimSuffix(path, "_tree"))
	if err != nil {
		return c.JSON(500, jsonError{Message: err.Error()})
	}
	return c.JSON(200, tree)
}

func (a *API) kvGetHandler(c echo.Context) error {
	path := trimPath(c.Request().URL.Path, KVPREFIX)
	if strings.HasSuffix(c.Request().URL.Path, "/_tree") {
		return a.treeHandler(c, path)
	}
	if strings.HasSuffix(path, "/") || path == "" {
		k, err := a.kv.GetKeys(path, "kv")
		if err != nil {
			a.log.Error(err)
			return c.JSON(500, jsonError{Message: err.Error()})
		}
		if len(k) == 0 {
			return c.JSON(404, jsonError{
				Message: "Key " + path + " does not exist",
			})
		}
		return c.JSON(200, k)
	}
	b, err := a.kv.Get(path, "kv")
	if err != nil {
		a.log.Error(err)
		return c.JSON(500, jsonError{Message: err.Error()})
	}

	if len(b) == 0 {
		return c.JSON(404, jsonError{Message: "Key " + path + " does not exist"})
	}
	return c.Blob(200, "application/json", b)

}

func (a *API) kvPutHandler(c echo.Context) error {
	path := trimPath(c.Request().URL.Path, APIPREFIX)
	buf, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		a.log.Error(err)
		return c.JSON(400, jsonError{Message: err.Error()})
	}
	err = a.kv.Put(path, buf, "kv")
	if err != nil {
		a.log.Error(err)
		return c.JSON(500, jsonError{Message: err.Error()})
	}
	return c.JSON(200, jsonError{Message: "ok"})
}

func (a *API) kvDeleteHandler(c echo.Context) error {
	path := trimPath(c.Request().URL.Path, APIPREFIX)
	if strings.HasSuffix(path, "/") {
		err := a.kv.DeleteBucket(path, "kv")
		if err != nil {
			if err == bbolt.ErrBucketNotFound {
				return c.JSON(404, jsonError{Message: err.Error()})
			}
			return c.JSON(500, jsonError{Message: err.Error()})
		}
	}
	err := a.kv.DeleteKey(path, "kv")
	if err != nil {
		return c.JSON(500, jsonError{Message: err.Error()})
	}
	return c.JSON(200, jsonError{Message: "ok"})
}

func trimPath(path string, prefix string) string {
	return path[len(prefix):]
}
