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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type contextKey string

const (
	userIDKey    contextKey = "userID"
	userEmailKey contextKey = "userEmail"
)

type Question struct {
	ID            string   `json:"id" firestore:"id"`
	Certification string   `json:"certification" firestore:"certification"`
	Domain        string   `json:"domain" firestore:"domain"`
	Prompt        string   `json:"prompt" firestore:"prompt"`
	Options       []string `json:"options" firestore:"options"`
	Answer        int      `json:"answer,omitempty" firestore:"answer"`
	Explanation   string   `json:"explanation,omitempty" firestore:"explanation"`
	Source        string   `json:"source,omitempty" firestore:"source"`
}

var demo = []Question{
	{ID: "frontend-1", Certification: "frontend", Domain: "Twig", Prompt: "What is the safest default for values printed by Twig?", Options: []string{"They are auto-escaped", "They run as PHP", "They bypass caching", "They become JSON"}, Answer: 0, Explanation: "Drupal's Twig environment auto-escapes output by default."},
	{ID: "frontend-2", Certification: "frontend", Domain: "Theme system", Prompt: "Which file declares a Drupal theme?", Options: []string{"THEME.info.yml", "settings.php", "composer.lock", "services.xml"}, Answer: 0, Explanation: "A theme is declared with its .info.yml file."},
	{ID: "backend-1", Certification: "backend", Domain: "Services", Prompt: "Which container holds registered Drupal services?", Options: []string{"Service container", "Render cache", "Theme registry", "State API"}, Answer: 0, Explanation: "Drupal services are registered and resolved through the service container."},
	{ID: "backend-2", Certification: "backend", Domain: "Plugins", Prompt: "What does a plugin manager primarily handle?", Options: []string{"Plugin discovery and instantiation", "CSS compilation", "DNS routing", "Browser storage"}, Answer: 0, Explanation: "Plugin managers discover definitions and create plugin instances."},
	{ID: "devops-1", Certification: "devops", Domain: "Configuration", Prompt: "Which command exports active Drupal configuration?", Options: []string{"drush config:export", "drush cache:rebuild", "composer audit", "phpunit --export"}, Answer: 0, Explanation: "Drush config:export writes active configuration to the sync directory."},
	{ID: "devops-2", Certification: "devops", Domain: "Deployment", Prompt: "Where should production secrets be stored?", Options: []string{"A managed secret store", "Committed settings.php", "Public files", "Theme templates"}, Answer: 0, Explanation: "Secrets belong in protected environment or secret-management systems."},
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
	r.With(s.requireUser).Get("/v1/questions", s.listQuestions)
	r.With(s.requireUser).Post("/v1/questions/{id}/check", s.checkAnswer)
	r.With(s.requireUser).Post("/v1/questions/{id}/report", s.reportQuestion)
	r.Group(func(r chi.Router) {
		r.Use(s.requireAdmin)
		r.Get("/v1/admin/status", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, 200, map[string]bool{"admin": true}) })
		r.Post("/v1/admin/questions/import", s.importQuestions)
		r.Get("/v1/admin/review", s.reviewQueue)
		r.Post("/v1/admin/questions/{id}/review", s.resolveReview)
		r.Get("/v1/admin/exhaustion-alerts", s.exhaustionAlerts)
		r.Post("/v1/admin/exhaustion-alerts/{id}/resolve", s.resolveExhaustionAlert)
		r.Post("/v1/admin/exhaustion-alerts/{id}/reset", s.resetExhaustedUser)
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
		if q.Certification == "" {
			q.Certification = "frontend"
		}
		q.Certification = strings.ToLower(strings.TrimSpace(q.Certification))
		result = append(result, q)
	}
	if len(result) == 0 {
		return demo, nil
	}
	return result, nil
}

func (s *server) listQuestions(w http.ResponseWriter, r *http.Request) {
	type publicQuestion struct {
		ID            string   `json:"id"`
		Certification string   `json:"certification"`
		Domain        string   `json:"domain"`
		Prompt        string   `json:"prompt"`
		Options       []string `json:"options"`
	}
	all, err := s.questions(r.Context())
	if err != nil {
		http.Error(w, "could not load questions", 500)
		return
	}
	certification := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("certification")))
	if certification != "" && !validCertification(certification) {
		http.Error(w, "invalid certification", 400)
		return
	}
	answered := map[string]bool{}
	if s.store != nil {
		iter := s.store.Collection("users").Doc(userID(r)).Collection("answered").Documents(r.Context())
		defer iter.Stop()
		for {
			doc, nextErr := iter.Next()
			if errors.Is(nextErr, iterator.Done) {
				break
			}
			if nextErr != nil {
				http.Error(w, "could not load answer history", 500)
				return
			}
			answered[doc.Ref.ID] = true
		}
	}
	out := make([]publicQuestion, 0, len(all))
	totalInTrack := 0
	for _, q := range all {
		if certification != "" && q.Certification != certification {
			continue
		}
		totalInTrack++
		if answered[q.ID] {
			continue
		}
		out = append(out, publicQuestion{q.ID, q.Certification, q.Domain, q.Prompt, q.Options})
	}
	if len(out) == 0 && totalInTrack > 0 && s.store != nil {
		s.recordExhaustion(r.Context(), userID(r), userEmail(r), certification, totalInTrack, totalInTrack)
		w.Header().Set("X-Question-Status", "exhausted")
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
			if s.store != nil {
				ref := s.store.Collection("users").Doc(userID(r)).Collection("answered").Doc(q.ID)
				err := s.store.RunTransaction(r.Context(), func(ctx context.Context, tx *firestore.Transaction) error {
					if _, getErr := tx.Get(ref); getErr == nil {
						return status.Error(codes.AlreadyExists, "question already answered")
					} else if status.Code(getErr) != codes.NotFound {
						return getErr
					}
					return tx.Create(ref, map[string]any{"questionId": q.ID, "certification": q.Certification, "correct": body.Answer == q.Answer, "answeredAt": time.Now().UTC()})
				})
				if status.Code(err) == codes.AlreadyExists {
					http.Error(w, "question already answered", http.StatusConflict)
					return
				}
				if err != nil {
					http.Error(w, "could not save answer", 500)
					return
				}
			}
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

func (s *server) requireUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			http.Error(w, "authentication is not configured", 503)
			return
		}
		raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if raw == "" {
			http.Error(w, "authentication required", 401)
			return
		}
		token, err := s.auth.VerifyIDToken(r.Context(), raw)
		if err != nil {
			http.Error(w, "invalid token", 401)
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, token.UID)
		ctx = context.WithValue(ctx, userEmailKey, fmt.Sprint(token.Claims["email"]))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userID(r *http.Request) string { value, _ := r.Context().Value(userIDKey).(string); return value }
func userEmail(r *http.Request) string {
	value, _ := r.Context().Value(userEmailKey).(string)
	return value
}

func (s *server) reportQuestion(w http.ResponseWriter, r *http.Request) {
	if s.store == nil {
		http.Error(w, "report storage is not configured", 503)
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		http.Error(w, "invalid report", 400)
		return
	}
	body.Reason = strings.TrimSpace(body.Reason)
	if len(body.Reason) > 500 {
		http.Error(w, "reason is too long", 400)
		return
	}
	id := chi.URLParam(r, "id")
	all, err := s.questions(r.Context())
	if err != nil {
		http.Error(w, "could not load question", 500)
		return
	}
	var question *Question
	for i := range all {
		if all[i].ID == id {
			question = &all[i]
			break
		}
	}
	if question == nil {
		http.Error(w, "question not found", 404)
		return
	}
	questionRef := s.store.Collection("questions").Doc(id)
	reportRef := questionRef.Collection("reports").Doc(userID(r))
	reportCount := int64(0)
	err = s.store.RunTransaction(r.Context(), func(ctx context.Context, tx *firestore.Transaction) error {
		if _, getErr := tx.Get(reportRef); getErr == nil {
			return status.Error(codes.AlreadyExists, "already reported")
		} else if status.Code(getErr) != codes.NotFound {
			return getErr
		}
		if doc, getErr := tx.Get(questionRef); getErr == nil {
			if value, ok := doc.Data()["reportCount"].(int64); ok {
				reportCount = value
			}
		} else if status.Code(getErr) != codes.NotFound {
			return getErr
		}
		reportCount++
		if err := tx.Create(reportRef, map[string]any{"uid": userID(r), "email": userEmail(r), "reason": body.Reason, "reportedAt": time.Now().UTC()}); err != nil {
			return err
		}
		return tx.Set(questionRef, map[string]any{"id": question.ID, "certification": question.Certification, "domain": question.Domain, "prompt": question.Prompt, "options": question.Options, "answer": question.Answer, "explanation": question.Explanation, "source": question.Source, "reportCount": reportCount, "needsReview": reportCount >= 3, "updatedAt": time.Now().UTC()}, firestore.MergeAll)
	})
	if status.Code(err) == codes.AlreadyExists {
		http.Error(w, "you already reported this question", http.StatusConflict)
		return
	}
	if err != nil {
		http.Error(w, "could not report question", 500)
		return
	}
	writeJSON(w, 200, map[string]any{"reported": true, "reportCount": reportCount, "needsReview": reportCount >= 3})
}

func (s *server) recordExhaustion(ctx context.Context, uid, email, certification string, answeredCount, availableCount int) {
	ref := s.store.Collection("exhaustionAlerts").Doc(uid + "-" + certification)
	_, err := ref.Create(ctx, map[string]any{"uid": uid, "email": email, "certification": certification, "answeredCount": answeredCount, "availableCount": availableCount, "createdAt": time.Now().UTC(), "resolved": false})
	if err != nil && status.Code(err) != codes.AlreadyExists {
		log.Printf("record exhaustion alert: %v", err)
	}
}

func (s *server) reviewQueue(w http.ResponseWriter, r *http.Request) {
	iter := s.store.Collection("questions").Where("needsReview", "==", true).Documents(r.Context())
	defer iter.Stop()
	items := []map[string]any{}
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			http.Error(w, "could not load review queue", 500)
			return
		}
		data := doc.Data()
		data["id"] = doc.Ref.ID
		items = append(items, data)
	}
	writeJSON(w, 200, items)
}

func (s *server) resolveReview(w http.ResponseWriter, r *http.Request) {
	_, err := s.store.Collection("questions").Doc(chi.URLParam(r, "id")).Set(r.Context(), map[string]any{"needsReview": false, "reportCount": 0, "reviewedAt": time.Now().UTC()}, firestore.MergeAll)
	if err != nil {
		http.Error(w, "could not resolve review", 500)
		return
	}
	writeJSON(w, 200, map[string]bool{"resolved": true})
}

func (s *server) exhaustionAlerts(w http.ResponseWriter, r *http.Request) {
	iter := s.store.Collection("exhaustionAlerts").Where("resolved", "==", false).Documents(r.Context())
	defer iter.Stop()
	items := []map[string]any{}
	for {
		doc, err := iter.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			http.Error(w, "could not load alerts", 500)
			return
		}
		data := doc.Data()
		data["id"] = doc.Ref.ID
		items = append(items, data)
	}
	writeJSON(w, 200, items)
}

func (s *server) resolveExhaustionAlert(w http.ResponseWriter, r *http.Request) {
	_, err := s.store.Collection("exhaustionAlerts").Doc(chi.URLParam(r, "id")).Set(r.Context(), map[string]any{"resolved": true, "resolvedAt": time.Now().UTC()}, firestore.MergeAll)
	if err != nil {
		http.Error(w, "could not resolve alert", 500)
		return
	}
	writeJSON(w, 200, map[string]bool{"resolved": true})
}

func (s *server) resetExhaustedUser(w http.ResponseWriter, r *http.Request) {
	alertRef := s.store.Collection("exhaustionAlerts").Doc(chi.URLParam(r, "id"))
	alert, err := alertRef.Get(r.Context())
	if err != nil {
		if status.Code(err) == codes.NotFound {
			http.Error(w, "exhaustion alert not found", 404)
			return
		}
		http.Error(w, "could not load exhaustion alert", 500)
		return
	}
	uid, _ := alert.Data()["uid"].(string)
	certification, _ := alert.Data()["certification"].(string)
	if uid == "" || !validCertification(certification) {
		http.Error(w, "exhaustion alert is invalid", 500)
		return
	}

	iter := s.store.Collection("users").Doc(uid).Collection("answered").Where("certification", "==", certification).Documents(r.Context())
	defer iter.Stop()
	deleted := 0
	batch := s.store.Batch()
	pending := 0
	for {
		doc, nextErr := iter.Next()
		if errors.Is(nextErr, iterator.Done) {
			break
		}
		if nextErr != nil {
			http.Error(w, "could not load user answer history", 500)
			return
		}
		batch.Delete(doc.Ref)
		deleted++
		pending++
		if pending == 400 {
			if _, err := batch.Commit(r.Context()); err != nil {
				http.Error(w, "could not reset user answer history", 500)
				return
			}
			batch = s.store.Batch()
			pending = 0
		}
	}
	if pending > 0 {
		if _, err := batch.Commit(r.Context()); err != nil {
			http.Error(w, "could not reset user answer history", 500)
			return
		}
	}
	if _, err := alertRef.Delete(r.Context()); err != nil {
		http.Error(w, "answer history reset, but alert could not be removed", 500)
		return
	}
	writeJSON(w, 200, map[string]any{"reset": true, "deletedAnswers": deleted, "certification": certification})
}

func (s *server) importQuestions(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	certification := strings.ToLower(strings.TrimSpace(r.FormValue("certification")))
	if !validCertification(certification) {
		http.Error(w, "certification must be frontend, backend, or devops", 400)
		return
	}
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
		q.Certification = certification
		q.ID = certification + "--" + q.ID
		ref := s.store.Collection("questions").Doc(q.ID)
		batch.Set(ref, map[string]any{"id": q.ID, "certification": q.Certification, "domain": q.Domain, "prompt": q.Prompt, "options": q.Options, "answer": q.Answer, "explanation": q.Explanation, "source": q.Source, "updatedAt": now})
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
		q := Question{ID: strings.TrimSpace(row[0]), Domain: strings.TrimSpace(row[1]), Prompt: strings.TrimSpace(row[2]), Options: []string{strings.TrimSpace(row[3]), strings.TrimSpace(row[4]), strings.TrimSpace(row[5]), strings.TrimSpace(row[6])}, Answer: answer, Explanation: strings.TrimSpace(row[8]), Source: strings.TrimSpace(row[9])}
		if q.ID == "" || q.Domain == "" || q.Prompt == "" {
			return nil, fmt.Errorf("row %d is missing id, domain, or prompt", index+2)
		}
		result = append(result, q)
	}
	return result, nil
}

func validCertification(value string) bool {
	return value == "frontend" || value == "backend" || value == "devops"
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
		origin, allowed := r.Header.Get("Origin"), false
		for _, value := range strings.Split(env("WEB_ORIGIN", "http://localhost:4200"), ",") {
			if strings.TrimRight(strings.TrimSpace(value), "/") == origin {
				allowed = true
				break
			}
		}
		if origin != "" && !allowed {
			http.Error(w, "origin not allowed", 403)
			return
		}
		if allowed {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
		}
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Expose-Headers", "X-Question-Status")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
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
