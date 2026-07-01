package internal

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"my-api-mcp/gen"
)

type BookService struct {
	mu     sync.RWMutex
	books  map[int64]gen.Book
	nextID int64
}

func NewBookService() *BookService {
	price1 := 49.99
	price2 := 39.99
	return &BookService{
		books: map[int64]gen.Book{
			1: {
				Id:     1,
				Title:  "The Go Programming Language",
				Author: "Alan A. A. Donovan",
				Genre:  stringPtr("Programming"),
				Isbn:   stringPtr("978-0134190440"),
				Price:  &price1,
			},
			2: {
				Id:     2,
				Title:  "Clean Code",
				Author: "Robert C. Martin",
				Genre:  stringPtr("Software Engineering"),
				Isbn:   stringPtr("978-0132350884"),
				Price:  &price2,
			},
		},
		nextID: 3,
	}
}

func stringPtr(s string) *string { return &s }

func (s *BookService) ListBooks(ctx context.Context, params gen.ListBooksParams) ([]gen.Book, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []gen.Book
	for _, book := range s.books {
		if params.Author != nil && book.Author != *params.Author {
			continue
		}
		if params.Genre != nil && book.Genre != nil && *book.Genre != *params.Genre {
			continue
		}
		result = append(result, book)
	}

	return result, nil
}

func (s *BookService) CreateBook(ctx context.Context, req gen.CreateBookRequest) (*gen.Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	book := gen.Book{
		Id:     s.nextID,
		Title:  req.Title,
		Author: req.Author,
		Genre:  req.Genre,
		Isbn:   req.Isbn,
	}

	// Price 类型转换：*float32 -> *float64
	if req.Price != nil {
		p := float64(*req.Price)
		book.Price = &p
	}

	s.books[s.nextID] = book
	s.nextID++

	return &book, nil
}

func (s *BookService) GetBook(ctx context.Context, bookId int64) (*gen.Book, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	book, ok := s.books[bookId]
	if !ok {
		return nil, fmt.Errorf("book %d not found", bookId)
	}

	return &book, nil
}

func (s *BookService) UpdateBook(ctx context.Context, bookId int64, req gen.UpdateBookRequest) (*gen.Book, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	book, ok := s.books[bookId]
	if !ok {
		return nil, fmt.Errorf("book %d not found", bookId)
	}

	if req.Title != nil {
		book.Title = *req.Title
	}
	if req.Author != nil {
		book.Author = *req.Author
	}
	if req.Genre != nil {
		book.Genre = req.Genre
	}
	if req.Price != nil {
		p := float64(*req.Price)
		book.Price = &p
	}

	s.books[bookId] = book
	return &book, nil
}

func (s *BookService) DeleteBook(ctx context.Context, bookId int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.books[bookId]; !ok {
		return fmt.Errorf("book %d not found", bookId)
	}

	delete(s.books, bookId)
	return nil
}

func (s *BookService) SearchBooks(ctx context.Context, params gen.SearchBooksParams) ([]gen.Book, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []gen.Book
	limit := 10
	if params.Limit != nil {
		limit = *params.Limit
	}

	for _, book := range s.books {
		if strings.Contains(strings.ToLower(book.Title), strings.ToLower(params.Q)) ||
			strings.Contains(strings.ToLower(book.Author), strings.ToLower(params.Q)) {
			results = append(results, book)
			if len(results) >= limit {
				break
			}
		}
	}

	return results, len(results), nil
}
