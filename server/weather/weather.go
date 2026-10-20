// Package weather gets forecasts from the National Weather Service API and
// caches them.
//
// Getting a forecast from NWS takes two requests: /points/{lat},{lng} says
// which forecast grid cell the location is in, then the grid's forecast URL
// has the actual forecast. Grid cells are about 2.5 km and never really
// change, so that lookup is cached for a week. Forecasts update about once an
// hour, so they're cached for 30 minutes.
package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	pointTTL    = 7 * 24 * time.Hour
	forecastTTL = 30 * time.Minute
	// the app shows the next 4 forecast periods (today, tonight, ...)
	periodsShown = 4
)

// ErrNoForecast means NWS didn't have a forecast for that spot.
var ErrNoForecast = errors.New("no forecast available for this location")

// Cache is the key/value store the client uses. Redis in the app, a map in tests.
type Cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
}

// Period is one block of the NWS forecast, like "Tonight" or "Tuesday".
type Period struct {
	Name             string `json:"name"`
	StartTime        string `json:"startTime"`
	IsDaytime        bool   `json:"isDaytime"`
	Temperature      int    `json:"temperature"`
	TemperatureUnit  string `json:"temperatureUnit"`
	WindSpeed        string `json:"windSpeed"`
	WindDirection    string `json:"windDirection"`
	ShortForecast    string `json:"shortForecast"`
	DetailedForecast string `json:"detailedForecast"`
}

type Client struct {
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
	Cache     Cache
	Log       *slog.Logger

	// A route asks for a lot of checkpoints at once and several of them can
	// land in the same grid cell, so identical requests that are already in
	// flight get shared instead of all hitting NWS.
	group singleflight.Group
}

func New(baseURL, userAgent string, cache Cache, log *slog.Logger) *Client {
	return &Client{
		BaseURL:   strings.TrimRight(baseURL, "/"),
		UserAgent: userAgent,
		HTTP:      &http.Client{Timeout: 10 * time.Second},
		Cache:     cache,
		Log:       log,
	}
}

// Forecast returns the next few forecast periods for a location, and whether
// the forecast came out of the cache.
func (c *Client) Forecast(ctx context.Context, lat, lng float64) ([]Period, bool, error) {
	forecastURL, err := c.forecastURL(ctx, lat, lng)
	if err != nil {
		return nil, false, err
	}

	key := "nws:forecast:" + forecastURL
	if v, ok := c.cacheGet(ctx, key); ok {
		var periods []Period
		if err := json.Unmarshal([]byte(v), &periods); err == nil {
			return periods, true, nil
		}
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		var body struct {
			Properties struct {
				Periods []Period `json:"periods"`
			} `json:"properties"`
		}
		if err := c.getJSON(ctx, forecastURL, &body); err != nil {
			return nil, err
		}
		periods := body.Properties.Periods
		if len(periods) == 0 {
			return nil, ErrNoForecast
		}
		if len(periods) > periodsShown {
			periods = periods[:periodsShown]
		}
		if b, err := json.Marshal(periods); err == nil {
			c.cacheSet(ctx, key, string(b), forecastTTL)
		}
		return periods, nil
	})
	if err != nil {
		return nil, false, err
	}
	return v.([]Period), false, nil
}

// forecastURL looks up which NWS grid cell a location is in. Coordinates are
// rounded to 2 decimals (about 1 km) so nearby clicks share a cache entry.
func (c *Client) forecastURL(ctx context.Context, lat, lng float64) (string, error) {
	coords := fmt.Sprintf("%.2f,%.2f", lat, lng)
	key := "nws:point:" + coords
	if v, ok := c.cacheGet(ctx, key); ok {
		return v, nil
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		var body struct {
			Properties struct {
				Forecast string `json:"forecast"`
			} `json:"properties"`
		}
		if err := c.getJSON(ctx, c.BaseURL+"/points/"+coords, &body); err != nil {
			return "", err
		}
		u := body.Properties.Forecast
		// only ever follow links back to the NWS API
		if u == "" || !strings.HasPrefix(u, c.BaseURL+"/") {
			return "", ErrNoForecast
		}
		c.cacheSet(ctx, key, u, pointTTL)
		return u, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

// getJSON does a GET against NWS. Their API throws the occasional 500, so
// server errors get one retry.
func (c *Client) getJSON(ctx context.Context, url string, out any) error {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(300 * time.Millisecond):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}
		// NWS asks every app to identify itself with contact info
		req.Header.Set("User-Agent", c.UserAgent)
		req.Header.Set("Accept", "application/geo+json")

		resp, err := c.HTTP.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		err = func() error {
			defer resp.Body.Close()
			switch {
			case resp.StatusCode == http.StatusNotFound:
				return ErrNoForecast
			case resp.StatusCode >= 500:
				return fmt.Errorf("nws returned %d", resp.StatusCode)
			case resp.StatusCode != http.StatusOK:
				return fmt.Errorf("nws returned %d: %w", resp.StatusCode, ErrNoForecast)
			}
			return json.NewDecoder(resp.Body).Decode(out)
		}()
		if err == nil || errors.Is(err, ErrNoForecast) {
			return err
		}
		lastErr = err
	}
	return lastErr
}

// If the cache is down the app still works, it just goes straight to NWS.
func (c *Client) cacheGet(ctx context.Context, key string) (string, bool) {
	v, ok, err := c.Cache.Get(ctx, key)
	if err != nil {
		c.Log.Warn("cache get failed", "key", key, "err", err)
		return "", false
	}
	return v, ok
}

func (c *Client) cacheSet(ctx context.Context, key, value string, ttl time.Duration) {
	if err := c.Cache.Set(ctx, key, value, ttl); err != nil {
		c.Log.Warn("cache set failed", "key", key, "err", err)
	}
}
