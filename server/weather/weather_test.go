package weather

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memCache struct {
	mu   sync.Mutex
	data map[string]string
	fail bool
}

func newMemCache() *memCache { return &memCache{data: map[string]string{}} }

func (m *memCache) Get(_ context.Context, k string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return "", false, errors.New("cache down")
	}
	v, ok := m.data[k]
	return v, ok, nil
}

func (m *memCache) Set(_ context.Context, k, v string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		return errors.New("cache down")
	}
	m.data[k] = v
	return nil
}

// fakeNWS acts like api.weather.gov and counts the requests it gets.
type fakeNWS struct {
	*httptest.Server
	points, forecasts atomic.Int32
	userAgent         atomic.Value
	failNext          atomic.Int32 // how many requests should 500 first
}

func newFakeNWS(t *testing.T) *fakeNWS {
	f := &fakeNWS{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.userAgent.Store(r.Header.Get("User-Agent"))
		if f.failNext.Load() > 0 {
			f.failNext.Add(-1)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		switch {
		case strings.HasPrefix(r.URL.Path, "/points/0.00"):
			// pretend 0,0 is in the ocean
			w.WriteHeader(http.StatusNotFound)
		case strings.HasPrefix(r.URL.Path, "/points/"):
			f.points.Add(1)
			fmt.Fprintf(w, `{"properties":{"forecast":"%s/gridpoints/MSO/1,2/forecast"}}`, f.URL)
		case r.URL.Path == "/gridpoints/MSO/1,2/forecast":
			f.forecasts.Add(1)
			io.WriteString(w, `{"properties":{"periods":[
				{"name":"Today","temperature":41,"temperatureUnit":"F","shortForecast":"Snow"},
				{"name":"Tonight","temperature":20,"temperatureUnit":"F","shortForecast":"Clear"},
				{"name":"Tuesday","temperature":45,"temperatureUnit":"F","shortForecast":"Sunny"},
				{"name":"Tuesday Night","temperature":22,"temperatureUnit":"F","shortForecast":"Clear"},
				{"name":"Wednesday","temperature":50,"temperatureUnit":"F","shortForecast":"Sunny"}
			]}}`)
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func newClient(nws *fakeNWS, cache Cache) *Client {
	return New(nws.URL, "test-agent (test@example.com)", cache, slog.New(slog.NewTextHandler(io.Discard, nil)))
}

func TestForecastUsesCache(t *testing.T) {
	nws := newFakeNWS(t)
	c := newClient(nws, newMemCache())
	ctx := context.Background()

	periods, cached, err := c.Forecast(ctx, 46.8721, -113.9940)
	if err != nil {
		t.Fatal(err)
	}
	if cached {
		t.Fatal("first request should not be cached")
	}
	if len(periods) != 4 || periods[0].Name != "Today" || periods[0].Temperature != 41 {
		t.Fatalf("unexpected periods: %+v", periods)
	}

	// same spot, and one a few hundred meters away that rounds to the same key
	for _, p := range [][2]float64{{46.8721, -113.9940}, {46.8705, -113.9920}} {
		if _, cached, err := c.Forecast(ctx, p[0], p[1]); err != nil || !cached {
			t.Fatalf("Forecast(%v) cached=%v err=%v, want a cache hit", p, cached, err)
		}
	}
	if nws.points.Load() != 1 || nws.forecasts.Load() != 1 {
		t.Fatalf("NWS got %d points and %d forecast requests, want 1 and 1", nws.points.Load(), nws.forecasts.Load())
	}
	if ua := nws.userAgent.Load(); ua != "test-agent (test@example.com)" {
		t.Fatalf("User-Agent = %v", ua)
	}
}

func TestForecastSharesInFlightRequests(t *testing.T) {
	nws := newFakeNWS(t)
	c := newClient(nws, newMemCache())

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, _, err := c.Forecast(context.Background(), 46.87, -113.99); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if n := nws.forecasts.Load(); n > 2 {
		t.Fatalf("20 parallel requests caused %d forecast calls to NWS", n)
	}
}

func TestForecastRetriesServerErrors(t *testing.T) {
	nws := newFakeNWS(t)
	nws.failNext.Store(1)
	c := newClient(nws, newMemCache())
	if _, _, err := c.Forecast(context.Background(), 46.87, -113.99); err != nil {
		t.Fatalf("one 500 should be retried, got %v", err)
	}
}

func TestForecastNotFound(t *testing.T) {
	nws := newFakeNWS(t)
	c := newClient(nws, newMemCache())
	if _, _, err := c.Forecast(context.Background(), 0, 0); !errors.Is(err, ErrNoForecast) {
		t.Fatalf("err = %v, want ErrNoForecast", err)
	}
}

func TestForecastWorksWhenCacheIsDown(t *testing.T) {
	nws := newFakeNWS(t)
	cache := newMemCache()
	cache.fail = true
	c := newClient(nws, cache)
	if _, _, err := c.Forecast(context.Background(), 46.87, -113.99); err != nil {
		t.Fatalf("should fall back to NWS when the cache errors, got %v", err)
	}
}
