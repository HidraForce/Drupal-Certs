package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type ctxKey string
const userKey ctxKey = "user"

type Progress struct { Correct int `json:"correct"`; Total int `json:"total"`; Streak int `json:"streak"`; UpdatedAt time.Time `json:"updatedAt"` }

func main() {
	ctx := context.Background()
	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: env("FIREBASE_PROJECT_ID", "drupal-study-lab")})
	if err != nil { log.Fatal(err) }
	authClient, err := app.Auth(ctx); if err != nil { log.Fatal(err) }
	store, err := app.Firestore(ctx); if err != nil { log.Fatal(err) }; defer store.Close()

	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, cors)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]string{"status":"ok", "service":"progress"}) })
	r.Group(func(r chi.Router) {
		r.Use(requireUser(authClient))
		r.Get("/v1/progress", func(w http.ResponseWriter, r *http.Request) {
			uid := r.Context().Value(userKey).(string); var p Progress
			doc, err := store.Collection("progress").Doc(uid).Get(r.Context())
			if err != nil { writeJSON(w, 200, Progress{}); return }
			if doc.DataTo(&p) != nil { http.Error(w, "could not read progress", 500); return }; writeJSON(w, 200, p)
		})
		r.Post("/v1/progress", func(w http.ResponseWriter, r *http.Request) {
			uid := r.Context().Value(userKey).(string); var p Progress
			if json.NewDecoder(r.Body).Decode(&p) != nil { http.Error(w, "invalid body", 400); return }
			p.UpdatedAt = time.Now().UTC()
			if _, err := store.Collection("progress").Doc(uid).Set(r.Context(), p); err != nil { http.Error(w, "could not save progress", 500); return }
			writeJSON(w, 200, p)
		})
	})
	port := env("PORT", "8082"); log.Printf("progress service listening on :%s", port); log.Fatal(http.ListenAndServe(":"+port, r))
}

func requireUser(client *auth.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" { http.Error(w, "authentication required", 401); return }
		verified, err := client.VerifyIDToken(r.Context(), token); if err != nil { http.Error(w, "invalid token", 401); return }
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userKey, verified.UID)))
	}) }
}

func cors(next http.Handler) http.Handler { return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Header().Set("Access-Control-Allow-Origin", env("WEB_ORIGIN", "http://localhost:4200")); w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type"); w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS"); if r.Method == "OPTIONS" { w.WriteHeader(204); return }; next.ServeHTTP(w,r) }) }
func writeJSON(w http.ResponseWriter, status int, value any) { w.Header().Set("Content-Type", "application/json"); w.WriteHeader(status); _ = json.NewEncoder(w).Encode(value) }
func env(key, fallback string) string { if value := strings.TrimSpace(os.Getenv(key)); value != "" { return value }; return fallback }
