package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
)

type Counter struct {
	value int
	mutex sync.RWMutex
}

func (c *Counter) Increment() int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value++
	return c.value
}

func (c *Counter) Get() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.value
}

func (c *Counter) Reset() {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.value = 0
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	counter := &Counter{}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		count := counter.Get()
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Counter App Demo</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: linear-gradient(135deg, #ff9a9e 0%%, #fecfef 50%%, #fecfef 100%%); }
        .container { background: white; padding: 40px; border-radius: 15px; display: inline-block; box-shadow: 0 10px 25px rgba(0,0,0,0.2); }
        h1 { color: #333; font-size: 3em; margin-bottom: 30px; }
        .counter { font-size: 5em; color: #e74c3c; font-weight: bold; margin: 30px 0; }
        .buttons { margin: 30px 0; }
        button { font-size: 1.2em; padding: 15px 25px; margin: 10px; border: none; border-radius: 8px; cursor: pointer; transition: all 0.3s; }
        .btn-primary { background: #3498db; color: white; }
        .btn-success { background: #27ae60; color: white; }
        .btn-danger { background: #e74c3c; color: white; }
        button:hover { transform: translateY(-2px); box-shadow: 0 5px 15px rgba(0,0,0,0.2); }
        .info { background: #f8f9fa; padding: 20px; border-radius: 8px; margin: 20px 0; color: #333; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🔢 Counter Demo App</h1>
        <div class="counter">%d</div>
        <div class="buttons">
            <button class="btn-success" onclick="increment()">+ Increment</button>
            <button class="btn-danger" onclick="reset()">↻ Reset</button>
            <button class="btn-primary" onclick="location.reload()">🔄 Refresh</button>
        </div>
        <div class="info">
            <p><strong>Port:</strong> %s</p>
            <p>This counter app demonstrates stateful applications deployed via Draheim</p>
        </div>
    </div>
    <script>
        function increment() {
            fetch('/increment', {method: 'POST'})
                .then(() => location.reload());
        }
        function reset() {
            if(confirm('Reset counter to 0?')) {
                fetch('/reset', {method: 'POST'})
                    .then(() => location.reload());
            }
        }
    </script>
</body>
</html>
`, count, port)
	})

	http.HandleFunc("/increment", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			newValue := counter.Increment()
			log.Printf("Counter incremented to: %d", newValue)
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			counter.Reset()
			log.Printf("Counter reset to 0")
			w.WriteHeader(http.StatusOK)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	http.HandleFunc("/api/count", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"count": %d, "port": "%s"}`, counter.Get(), port)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "count": %d, "port": "%s"}`, counter.Get(), port)
	})

	log.Printf("Counter demo app starting on port %s", port)
	log.Printf("Initial counter value: %d", counter.Get())
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
