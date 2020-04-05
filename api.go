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

	// UIPREFIX path
	UIPREFIX = "/ui"

	// SYSPREFIX path
	SYSPREFIX = "/system"
)

type jsonResponse struct {
	Status  int         `json:",omitempty"`
	Message string      `json:",omitempty"`
	Bytes   int         `json:",omitempty"`
	Error   string      `json:",omitempty"`
	Data    interface{} `json:",omitempty"`
}

// API Type
type API struct {
	app       *Bunker
	config    *Config
	log       *Log
	terminate chan bool
	kv        *KV
	http      *echo.Echo
}

//NewAPI function
func NewAPI(app *Bunker) (*API, error) {
	a := &API{
		app:    app,
		config: app.Config,
		log:    app.Logger,
		kv:     app.KV,
	}
	a.terminate = make(chan bool)
	a.http = echo.New()
	a.http.HideBanner = true
	//a.http.Use(middleware.Recover())
	a.http.Use(a.log.EchoLogger())
	a.http.Any(APIPREFIX+"kv/*", a.kvHandler)
	a.http.Static(UIPREFIX, "./ui/")
	a.http.Debug = true
	return a, nil
}

// Start starts a new server
func (a *API) Start() {
	go a.watch()
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

func (a *API) kvHandler(c echo.Context) error {
	switch c.Request().Method {
	case "GET":
		return a.kvGetHandler(c)
	case "POST":
		return a.kvPutHandler(c)
	case "DELETE":
		return a.kvPutHandler(c)
	default:
		return c.JSON(405, jsonResponse{
			Status: 405,
			Error:  "Method " + c.Request().Method + "not allowed.",
		})
	}
}

func (a *API) kvGetHandler(c echo.Context) error {
	path := trimPath(c.Request().URL.Path, APIPREFIX)
	if strings.HasSuffix(path, "/") {
		k, err := a.kv.GetKeys(path)
		if err != nil {
			a.log.Error(err)
			return c.JSON(500, jsonResponse{Error: err.Error(), Status: 500})
		}
		if len(k) == 0 {
			return c.JSON(404, jsonResponse{
				Error:  "Key " + path + " does not exist",
				Status: 404,
			})
		}
		return c.JSON(200, jsonResponse{Data: k, Status: 200})
	}
	b, err := a.kv.Get(path)
	if err != nil {
		a.log.Error(err)
		return c.JSON(500, jsonResponse{Error: err.Error(), Status: 500})
	}
	if len(b) == 0 {
		return c.JSON(404, jsonResponse{Error: "Key " + path + " does not exist", Status: 404})
	}
	return c.Blob(200, "application/json", b)
}

func (a *API) kvPutHandler(c echo.Context) error {
	path := trimPath(c.Request().URL.Path, APIPREFIX)
	buf, err := ioutil.ReadAll(c.Request().Body)
	if err != nil {
		a.log.Error(err)
		return c.JSON(400, jsonResponse{Error: err.Error(), Status: 400})
	}
	err = a.kv.Put(path, buf)
	if err != nil {
		a.log.Error(err)
		return c.JSON(500, jsonResponse{Error: err.Error(), Status: 500})
	}
	return c.JSON(200, jsonResponse{Status: 200, Message: "OK", Bytes: len(buf)})
}

func (a *API) kvDeleteHandler(c echo.Context) error {
	path := trimPath(c.Request().URL.Path, APIPREFIX)
	if strings.HasSuffix(path, "/") {
		err := a.kv.DeleteBucket(path)
		if err != nil {
			if err == bbolt.ErrBucketNotFound {
				return c.JSON(404, jsonResponse{Error: err.Error(), Status: 404})
			}
			return c.JSON(500, jsonResponse{Error: err.Error(), Status: 500})
		}
	}
	err := a.kv.DeleteKey(path)
	if err != nil {
		return c.JSON(500, jsonResponse{Status: 500, Error: err.Error()})
	}
	return c.JSON(200, jsonResponse{Status: 200, Message: "OK"})
}

func trimPath(path string, prefix string) string {
	return path[len(prefix):]
}
