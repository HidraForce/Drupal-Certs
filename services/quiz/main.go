package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type Question struct {
	ID          string   `json:"id" firestore:"id"`
	Domain      string   `json:"domain" firestore:"domain"`
	Prompt      string   `json:"prompt" firestore:"prompt"`
	Options     []string `json:"options" firestore:"options"`
	Answer      int      `json:"answer,omitempty" firestore:"answer"`
	Explanation string   `json:"explanation,omitempty" firestore:"explanation"`
	Source      string   `json:"source,omitempty" firestore:"source"`
}

var demo = []Question{
	{ID: "demo-1", Domain: "Drupal fundamentals", Prompt: "Which layer is responsible for Drupal's rendered page structure?", Options: []string{"Theme layer", "Database driver", "Queue worker", "Cache backend"}, Answer: 0, Explanation: "Themes and render arrays control presentation."},
	{ID: "demo-2", Domain: "Twig", Prompt: "What is the safest default for values printed by Twig?", Options: []string{"They are auto-escaped", "They run as PHP", "They bypass caching", "They become JSON"}, Answer: 0, Explanation: "Drupal's Twig environment auto-escapes output by default."},
}

type server struct {
	store  *firestore.Client
	auth   *auth.Client
	admins map[string]bool
}

func main() {
	ctx := context.Background()
	var store *firestore.Client
	var authClient *auth.Client
	if project := strings.TrimSpace(os.Getenv("FIREBASE_PROJECT_ID")); project != "" {
		firebaseOptions := []option.ClientOption{}
		if credentials := strings.TrimSpace(os.Getenv("FIREBASE_CREDENTIALS_JSON")); credentials != "" {
			firebaseOptions = append(firebaseOptions, option.WithCredentialsJSON([]byte(credentials)))
		}
		app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: project}, firebaseOptions...)
		if err != nil {
			log.Fatal(err)
		}
		authClient, err = app.Auth(ctx)
		if err != nil {
			log.Fatal(err)
		}
		store, err = app.Firestore(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
	} else {
		log.Print("FIREBASE_PROJECT_ID is empty; using demo questions and disabling admin imports")
	}
	s := &server{store: store, auth: authClient, admins: adminSet(os.Getenv("ADMIN_EMAILS"))}
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, middleware.Recoverer, middleware.Compress(5), cors)
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok", "service": "quiz"})
	})
	r.Get("/v1/questions", s.listQuestions)
	r.Post("/v1/questions/{id}/check", s.checkAnswer)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/v1/admin/status", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]bool{"admin": true}) })
		r.Post("/v1/admin/questions/import", s.importQuestions)
	})
	port := env("PORT", "8081")
	log.Printf("quiz service listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func (s *server) questions(ctx context.Context) ([]Question, error) {
	if s.store == nil {
		return demo, nil
	}
	iter := s.store.Collection("questions").Documents(ctx)
	defer iter.Stop()
	result := []Question{}
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		var q Question
		if err := doc.DataTo(&q); err != nil {
			return nil, err
		}
		result = append(result, q)
	}
	if len(result) == 0 {
		return demo, nil
	}
	return result, nil
}

func (s *server) listQuestions(w http.ResponseWriter, r *http.Request) {
	type publicQuestion struct {
		ID      string   `json:"id"`
		Domain  string   `json:"domain"`
		Prompt  string   `json:"prompt"`
		Options []string `json:"options"`
	}
	all, err := s.questions(r.Context())
	if err != nil {
		http.Error(w, "could not load questions", 500)
		return
	}
	out := make([]publicQuestion, 0, len(all))
	for _, q := range all {
		out = append(out, publicQuestion{q.ID, q.Domain, q.Prompt, q.Options})
	}
	writeJSON(w, 200, out)
}

func (s *server) checkAnswer(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Answer int `json:"answer"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "invalid body", 400)
		return
	}
	all, err := s.questions(r.Context())
	if err != nil {
		http.Error(w, "could not load question", 500)
		return
	}
	for _, q := range all {
		if q.ID == chi.URLParam(r, "id") {
			writeJSON(w, 200, map[string]any{"correct": body.Answer == q.Answer, "answer": q.Answer, "explanation": q.Explanation})
			return
		}
	}
	http.Error(w, "question not found", 404)
}

func (s *server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil || s.store == nil {
			http.Error(w, "admin storage is not configured", 503)
			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		token, err := s.auth.VerifyIDToken(r.Context(), raw)
		if err != nil || !verifiedEmail(token) || !s.admins[strings.ToLower(fmt.Sprint(token.Claims["email"]))] {
			http.Error(w, "admin access required", 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *server) importQuestions(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "attach a CSV file under field 'file'", 400)
		return
	}
	defer file.Close()
	questions, err := parseCSV(file)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	batch := s.store.Batch()
	now := time.Now().UTC()
	for _, q := range questions {
		ref := s.store.Collection("questions").Doc(q.ID)
		batch.Set(ref, map[string]any{"id": q.ID, "domain": q.Domain, "prompt": q.Prompt, "options": q.Options, "answer": q.Answer, "explanation": q.Explanation, "source": q.Source, "updatedAt": now})
	}
	if _, err := batch.Commit(r.Context()); err != nil {
		http.Error(w, "could not save questions", 500)
		return
	}
	writeJSON(w, 200, map[string]any{"imported": len(questions)})
}

func parseCSV(input io.Reader) ([]Question, error) {
	reader := csv.NewReader(input)
	reader.TrimLeadingSpace = true
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}
	if len(rows) < 2 {
		return nil, errors.New("CSV must contain a header and at least one question")
	}
	want := []string{"id", "domain", "prompt", "option_a", "option_b", "option_c", "option_d", "answer", "explanation", "source"}
	if len(rows[0]) != len(want) {
		return nil, fmt.Errorf("expected %d columns", len(want))
	}
	for i := range want {
		if strings.ToLower(strings.TrimSpace(rows[0][i])) != want[i] {
			return nil, fmt.Errorf("column %d must be %q", i+1, want[i])
		}
	}
	if len(rows)-1 > 400 {
		return nil, errors.New("one import may contain at most 400 questions")
	}
	result := make([]Question, 0, len(rows)-1)
	for index, row := range rows[1:] {
		if len(row) != len(want) {
			return nil, fmt.Errorf("row %d has the wrong number of columns", index+2)
		}
		answer, err := strconv.Atoi(strings.TrimSpace(row[7]))
		if err != nil || answer < 0 || answer > 3 {
			return nil, fmt.Errorf("row %d answer must be 0, 1, 2, or 3", index+2)
		}
		q := Question{strings.TrimSpace(row[0]), strings.TrimSpace(row[1]), strings.TrimSpace(row[2]), []string{strings.TrimSpace(row[3]), strings.TrimSpace(row[4]), strings.TrimSpace(row[5]), strings.TrimSpace(row[6])}, answer, strings.TrimSpace(row[8]), strings.TrimSpace(row[9])}
		if q.ID == "" || q.Domain == "" || q.Prompt == "" {
			return nil, fmt.Errorf("row %d is missing id, domain, or prompt", index+2)
		}
		result = append(result, q)
	}
	return result, nil
}

func verifiedEmail(token *auth.Token) bool {
	value, ok := token.Claims["email_verified"].(bool)
	return ok && value
}
func adminSet(value string) map[string]bool {
	result := map[string]bool{}
	for _, email := range strings.Split(value, ",") {
		if email = strings.ToLower(strings.TrimSpace(email)); email != "" {
			result[email] = true
		}
	}
	return result
}
func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", env("WEB_ORIGIN", "http://localhost:4200"))
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
