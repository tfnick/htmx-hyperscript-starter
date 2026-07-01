package forum

import (
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrThreadNotFound = errors.New("thread not found")
	ErrInvalidInput   = errors.New("invalid input")
)

type Thread struct {
	ID        string
	Title     string
	Author    string
	Category  string
	Body      string
	Replies   []Reply
	CreatedAt time.Time
}

type Reply struct {
	ID        string
	Author    string
	Body      string
	CreatedAt time.Time
}

type Store struct {
	mu            sync.RWMutex
	threads       []Thread
	nextThreadID  int
	nextReplyID   int
	defaultAuthor string
}

func NewStore() *Store {
	now := time.Now().Add(-2 * time.Hour)
	return &Store{
		threads: []Thread{
			{
				ID:        "welcome",
				Title:     "Welcome to the HTMX Forum",
				Author:    "Zach",
				Category:  "Announcements",
				Body:      "This starter has been reshaped into a compact forum powered by Go, Echo, htmx, hyperscript, and Pico.css.",
				CreatedAt: now,
				Replies: []Reply{
					{
						ID:        "reply-1",
						Author:    "Mira",
						Body:      "Template overrides make this useful for experimenting with custom forum themes.",
						CreatedAt: now.Add(28 * time.Minute),
					},
				},
			},
			{
				ID:        "template-overrides",
				Title:     "How should template overrides be organized?",
				Author:    "Ari",
				Category:  "Development",
				Body:      "The executable ships with defaults, while a configured template path can replace individual HTML files.",
				CreatedAt: now.Add(35 * time.Minute),
			},
		},
		nextThreadID:  3,
		nextReplyID:   2,
		defaultAuthor: "Guest",
	}
}

func (s *Store) ListThreads() []Thread {
	s.mu.RLock()
	defer s.mu.RUnlock()

	threads := make([]Thread, len(s.threads))
	copy(threads, s.threads)
	sort.SliceStable(threads, func(i, j int) bool {
		return threads[i].CreatedAt.After(threads[j].CreatedAt)
	})

	return threads
}

func (s *Store) GetThread(id string) (Thread, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, thread := range s.threads {
		if thread.ID == id {
			return cloneThread(thread), nil
		}
	}

	return Thread{}, ErrThreadNotFound
}

func (s *Store) CreateThread(title, author, category, body string) (Thread, error) {
	title = strings.TrimSpace(title)
	author = cleanAuthor(author, s.defaultAuthor)
	category = strings.TrimSpace(category)
	body = strings.TrimSpace(body)
	if title == "" || body == "" {
		return Thread{}, ErrInvalidInput
	}
	if category == "" {
		category = "General"
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	thread := Thread{
		ID:        makeThreadID(title, s.nextThreadID),
		Title:     title,
		Author:    author,
		Category:  category,
		Body:      body,
		CreatedAt: time.Now(),
	}
	s.nextThreadID++
	s.threads = append(s.threads, thread)

	return cloneThread(thread), nil
}

func (s *Store) AddReply(threadID, author, body string) (Thread, error) {
	author = cleanAuthor(author, s.defaultAuthor)
	body = strings.TrimSpace(body)
	if body == "" {
		return Thread{}, ErrInvalidInput
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.threads {
		if s.threads[i].ID != threadID {
			continue
		}

		reply := Reply{
			ID:        makeReplyID(s.nextReplyID),
			Author:    author,
			Body:      body,
			CreatedAt: time.Now(),
		}
		s.nextReplyID++
		s.threads[i].Replies = append(s.threads[i].Replies, reply)

		return cloneThread(s.threads[i]), nil
	}

	return Thread{}, ErrThreadNotFound
}

func cloneThread(thread Thread) Thread {
	thread.Replies = append([]Reply(nil), thread.Replies...)
	return thread
}

func cleanAuthor(author, fallback string) string {
	author = strings.TrimSpace(author)
	if author == "" {
		return fallback
	}

	return author
}

func makeThreadID(title string, fallback int) string {
	var b strings.Builder
	lastWasDash := false
	for _, r := range strings.ToLower(title) {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastWasDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastWasDash = false
		case !lastWasDash:
			b.WriteRune('-')
			lastWasDash = true
		}
	}

	id := strings.Trim(b.String(), "-")
	if id == "" {
		id = "thread"
	}

	return id + "-" + strconv.Itoa(fallback)
}

func makeReplyID(id int) string {
	return "reply-" + strconv.Itoa(id)
}
