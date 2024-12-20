package fiberfluentdlogger

/*
Copyright 2024 Rodolfo González González

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

import (
	"fmt"
	"time"

	"runtime/debug"

	"github.com/fluent/fluent-logger-golang/fluent"
	fiber "github.com/gofiber/fiber/v2"
	"github.com/ztrue/tracerr"
)

//*****************************************************************************

type LoggerConfig struct {
	Enabled bool // whether the middleware is enabled
	Host string // the fluentd server address
	Port int    // the fluentd server port
	Tag  string // the tag to be used for the messages
}

//-----------------------------------------------------------------------------

// Logger is a struct that holds the Fluentd logger instance and configuration
type Logger struct {
	client *fluent.Fluent
	tag    string
}

//-----------------------------------------------------------------------------

// New initializes a Fluentd logger and returns a middleware
func New(config LoggerConfig) (*Logger, error) {
	if !config.Enabled {
		return nil, fmt.Errorf("middleware disabled")
	}

	// Initialize Fluentd logger
	fluentLogger, err := fluent.New(fluent.Config{
		FluentHost: config.Host,
		FluentPort: config.Port,
	})
	if err != nil {
		return nil, err
	}

	return &Logger{
		client: fluentLogger,
		tag:    config.Tag,
	}, nil
}

//-----------------------------------------------------------------------------

// Logger logs each request to Fluentd
func (l *Logger) Logger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next() // Process the request
		latency := time.Since(start)

		// Log data to Fluentd
		logData := map[string]interface{}{
			"method":        c.Method(),
			"path":          c.Path(),
			"status":        c.Response().StatusCode(),
			"latency_ms":    latency.Milliseconds(),
			"client_ip":     c.IP(),
			"user_agent":    c.Get("User-Agent"),
			"response_size": len(c.Response().Body()),
		}
		if err != nil {
			logData["error"] = tracerr.SprintSource(err)
		}

		// Send to Fluentd
		if err := l.client.Post(l.tag, logData); err != nil {
			tracerr.PrintSource(err)
		}

		return err
	}
}

//-----------------------------------------------------------------------------

// PanicLogger logs details on panic to Fluentd
func (l *Logger) PanicLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		err := c.Next() // Process the request

		// Check if there was a panic (status code 500 indicates a server error)
		if c.Response().StatusCode() == fiber.StatusInternalServerError {
			// Log data to Fluentd
			logData := map[string]interface{}{
				"method":     c.Method(),
				"path":       c.Path(),
				"client_ip":  c.IP(),
				"user_agent": c.Get("User-Agent"),
			}

			// Optionally, include the details of the err
			if err != nil {
				logData["error"] = tracerr.SprintSource(err)
			}

			// Optionally, include stack trace if err is a panic
			if err != nil {
				logData["stacktrace"] = string(debug.Stack())
			}

			// Send to Fluentd
			if err := l.client.Post(l.tag+".panic", logData); err != nil {
				tracerr.PrintSource(err)
			}
		}

		return err
	}
}
