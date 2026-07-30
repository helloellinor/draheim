package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Hello World Demo App</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); color: white; }
        .container { background: rgba(255,255,255,0.1); padding: 40px; border-radius: 10px; display: inline-block; }
        h1 { font-size: 3em; margin-bottom: 20px; }
        p { font-size: 1.2em; }
        .info { background: rgba(255,255,255,0.2); padding: 20px; border-radius: 8px; margin: 20px 0; }
    </style>
</head>
<body>
    <div class="container">
        <h1>🎉 Hello World!</h1>
        <p>This is a demo application deployed via Draheim</p>
        <div class="info">
            <p><strong>Port:</strong> %s</p>
            <p><strong>Status:</strong> Running successfully!</p>
            <p><strong>Time:</strong> <span id="time"></span></p>
        </div>
        <p>This demo shows how Draheim can deploy Go applications from Git repositories</p>
    </div>
    <script>
        setInterval(() => {
            document.getElementById('time').textContent = new Date().toLocaleString();
        }, 1000);
    </script>
</body>
</html>
`, port)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"status": "healthy", "port": "%s"}`, port)
	})

	log.Printf("Hello World demo app starting on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}