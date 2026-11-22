package main

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	PublicKey string
	PrivateKey string
	APIURL string
	Port string
	DataFile string
}

type LibraryItem struct {
	ComicID     string `json:"comicId"`
	Title       string `json:"title"`
	Series      string `json:"series"`
	IssueNumber string `json:"issueNumber"`
	ReleaseDate string `json:"releaseDate"`
	CoverURL    string `json:"coverUrl"`
	Description string `json:"description"`
	Read        bool   `json:"read"`
	Downloaded  bool   `json:"downloaded"`
	FilePath    string `json:"filePath"`
	AddedAt     string `json:"addedAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type LibraryStore struct {
	mu     sync.RWMutex
	path   string
	items  []LibraryItem
}

type ComicResponse struct {
	Data struct {
		Results []struct {
			ID          int `json:"id"`
			Title       string `json:"title"`
			IssueNumber string `json:"issueNumber"`
			Description string `json:"description"`
			Dates       []struct {
				Type string `json:"type"`
				Date string `json:"date"`
			} `json:"dates"`
			Thumbnail struct {
				Path      string `json:"path"`
				Extension string `json:"extension"`
			} `json:"thumbnail"`
			Series struct {
				Name string `json:"name"`
			} `json:"series"`
		} `json:"results"`
		Total int `json:"total"`
	} `json:"data"`
}

type Comic struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	IssueNumber string `json:"issueNumber"`
	Description string `json:"description"`
	ReleaseDate string `json:"releaseDate"`
	CoverURL    string `json:"coverUrl"`
	Series      string `json:"series"`
}

var cfg Config
var store *LibraryStore

func main() {
	loadDotEnv(".env")

	cfg = Config{
		PublicKey:  os.Getenv("COMIC_ASSISTANT_PUBLIC_KEY"),
		PrivateKey: os.Getenv("COMIC_ASSISTANT_PRIVATE_KEY"),
		APIURL:     os.Getenv("COMIC_ASSISTANT_API_URL"),
		Port:       getenv("PORT", "8080"),
		DataFile:   getenv("DATA_FILE", "./data/library.json"),
	}

	store = &LibraryStore{path: cfg.DataFile}
	if err := store.load(); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", healthHandler)
	mux.HandleFunc("/api/search", searchHandler)
	mux.HandleFunc("/api/library", libraryHandler)
	mux.HandleFunc("/api/library/", libraryItemHandler)

	handler := cors(mux)

	log.Printf("Comic Assistant API listening on http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, handler))
}

func loadDotEnv(path string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}

	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
		if os.Getenv(key) == "" {
			_ = os.Setenv(key, value)
		}
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"configured": cfg.PublicKey != "" && cfg.PrivateKey != "" && cfg.APIURL != "",
	})
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query parameter q is required"})
		return
	}

	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	comics, total, err := searchComics(query, limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": comics,
		"total": total,
	})
}

func libraryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, store.list())
	case http.MethodPost:
		var item LibraryItem
		if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}
		if item.ComicID == "" || item.Title == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "comicId and title are required"})
			return
		}

		now := time.Now().UTC().Format(time.RFC3339)
		item.AddedAt = now
		item.UpdatedAt = now

		if err := store.add(item); err != nil {
			writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusCreated, item)
	default:
		methodNotAllowed(w)
	}
}

func libraryItemHandler(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/library/")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "comic id is required"})
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var patch struct {
			Read       *bool   `json:"read"`
			Downloaded *bool   `json:"downloaded"`
			FilePath   *string `json:"filePath"`
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
			return
		}

		item, err := store.update(id, patch.Read, patch.Downloaded, patch.FilePath)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, item)

	case http.MethodDelete:
		if err := store.remove(id); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		methodNotAllowed(w)
	}
}

func searchComics(query string, limit int) ([]Comic, int, error) {
	if cfg.PublicKey == "" || cfg.PrivateKey == "" || cfg.APIURL == "" {
		return nil, 0, fmt.Errorf("comic API is not configured; set COMIC_ASSISTANT_API_URL, COMIC_ASSISTANT_PUBLIC_KEY and COMIC_ASSISTANT_PRIVATE_KEY")
	}

	ts := strconv.FormatInt(time.Now().UnixNano(), 10)
	hashInput := ts + cfg.PrivateKey + cfg.PublicKey
	sum := md5.Sum([]byte(hashInput))
	hash := hex.EncodeToString(sum[:])

	url := fmt.Sprintf(
		"%s?apikey=%s&ts=%s&hash=%s&titleStartsWith=%s&limit=%d&orderBy=-onsaleDate",
		strings.TrimRight(cfg.APIURL, "?&"), cfg.PublicKey, ts, hash, urlQuery(query), limit,
	)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("comic API returned HTTP %d: %s", resp.StatusCode, truncate(string(body), 300))
	}

	var decoded ComicResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, 0, fmt.Errorf("invalid comic API response: %w", err)
	}

	results := make([]Comic, 0, len(decoded.Data.Results))
	for _, c := range decoded.Data.Results {
		cover := ""
		if c.Thumbnail.Path != "" && c.Thumbnail.Extension != "" {
			cover = c.Thumbnail.Path + "." + c.Thumbnail.Extension
			cover = strings.Replace(cover, "http://", "https://", 1)
		}

		release := ""
		for _, d := range c.Dates {
			if d.Type == "onsaleDate" {
				release = d.Date
				break
			}
		}

		results = append(results, Comic{
			ID:          strconv.Itoa(c.ID),
			Title:       c.Title,
			IssueNumber: c.IssueNumber,
			Description: stripHTML(c.Description),
			ReleaseDate: release,
			CoverURL:    cover,
			Series:      c.Series.Name,
		})
	}

	return results, decoded.Data.Total, nil
}

func (s *LibraryStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	b, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		s.items = []LibraryItem{}
		return s.persistLocked()
	}
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		s.items = []LibraryItem{}
		return nil
	}
	return json.Unmarshal(b, &s.items)
}

func (s *LibraryStore) persistLocked() error {
	b, err := json.MarshalIndent(s.items, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *LibraryStore) list() []LibraryItem {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]LibraryItem, len(s.items))
	copy(out, s.items)
	return out
}

func (s *LibraryStore) add(item LibraryItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, existing := range s.items {
		if existing.ComicID == item.ComicID {
			return fmt.Errorf("comic already exists in library")
		}
	}

	s.items = append(s.items, item)
	return s.persistLocked()
}

func (s *LibraryStore) update(id string, read, downloaded *bool, filePath *string) (LibraryItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.items {
		if s.items[i].ComicID != id {
			continue
		}
		if read != nil {
			s.items[i].Read = *read
		}
		if downloaded != nil {
			s.items[i].Downloaded = *downloaded
		}
		if filePath != nil {
			s.items[i].FilePath = *filePath
		}
		s.items[i].UpdatedAt = time.Now().UTC().Format(time.RFC3339)

		if err := s.persistLocked(); err != nil {
			return LibraryItem{}, err
		}
		return s.items[i], nil
	}

	return LibraryItem{}, fmt.Errorf("comic %s not found", id)
}

func (s *LibraryStore) remove(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.items {
		if s.items[i].ComicID == id {
			s.items = append(s.items[:i], s.items[i+1:]...)
			return s.persistLocked()
		}
	}
	return fmt.Errorf("comic %s not found", id)
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PATCH,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func urlQuery(s string) string {
	r := strings.NewReplacer(
		" ", "%20",
		"+", "%2B",
		"&", "%26",
		"?", "%3F",
		"#", "%23",
	)
	return r.Replace(s)
}

func stripHTML(s string) string {
	for {
		start := strings.Index(s, "<")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], ">")
		if end < 0 {
			return s[:start]
		}
		s = s[:start] + " " + s[start+end+1:]
	}
	return strings.Join(strings.Fields(s), " ")
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
