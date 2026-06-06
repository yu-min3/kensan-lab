package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	note "github.com/kensan/backend/services/note/internal"
	"github.com/kensan/backend/services/note/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ========== Mock Repository ==========

// Compile-time check that MockRepository implements repository.Repository
var _ repository.Repository = (*MockRepository)(nil)

// MockRepository is a mock implementation of the repository.Repository interface
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) GetByIDAndUserID(ctx context.Context, id, userID string) (*note.Note, error) {
	args := m.Called(ctx, id, userID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*note.Note), args.Error(1)
}

func (m *MockRepository) List(ctx context.Context, userID string, filter *note.NoteFilter) ([]*note.NoteListItem, error) {
	args := m.Called(ctx, userID, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*note.NoteListItem), args.Error(1)
}

func (m *MockRepository) Create(ctx context.Context, n *note.Note) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockRepository) Update(ctx context.Context, n *note.Note) error {
	args := m.Called(ctx, n)
	return args.Error(0)
}

func (m *MockRepository) DeleteByIDAndUserID(ctx context.Context, id, userID string) error {
	args := m.Called(ctx, id, userID)
	return args.Error(0)
}

func (m *MockRepository) Search(ctx context.Context, userID, query string, filter *note.NoteFilter, limit int) ([]*note.SearchResult, error) {
	args := m.Called(ctx, userID, query, filter, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*note.SearchResult), args.Error(1)
}

func (m *MockRepository) ListNoteTypes(ctx context.Context, activeOnly bool) ([]*note.NoteTypeConfig, error) {
	args := m.Called(ctx, activeOnly)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*note.NoteTypeConfig), args.Error(1)
}

func (m *MockRepository) GetNoteTypeBySlug(ctx context.Context, slug string) (*note.NoteTypeConfig, error) {
	args := m.Called(ctx, slug)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*note.NoteTypeConfig), args.Error(1)
}

func (m *MockRepository) ListContents(ctx context.Context, noteID string) ([]*note.NoteContent, error) {
	args := m.Called(ctx, noteID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*note.NoteContent), args.Error(1)
}

func (m *MockRepository) GetContent(ctx context.Context, contentID string) (*note.NoteContent, error) {
	args := m.Called(ctx, contentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*note.NoteContent), args.Error(1)
}

func (m *MockRepository) CreateContent(ctx context.Context, content *note.NoteContent) error {
	args := m.Called(ctx, content)
	return args.Error(0)
}

func (m *MockRepository) UpdateContent(ctx context.Context, content *note.NoteContent) error {
	args := m.Called(ctx, content)
	return args.Error(0)
}

func (m *MockRepository) DeleteContent(ctx context.Context, contentID string) error {
	args := m.Called(ctx, contentID)
	return args.Error(0)
}

func (m *MockRepository) ReorderContents(ctx context.Context, noteID string, contentIDs []string) error {
	args := m.Called(ctx, noteID, contentIDs)
	return args.Error(0)
}

func (m *MockRepository) UpdateIndexStatus(ctx context.Context, noteID string, status note.IndexStatus) error {
	args := m.Called(ctx, noteID, status)
	return args.Error(0)
}

func (m *MockRepository) ListMetadata(ctx context.Context, noteID string) ([]*note.NoteMetadataItem, error) {
	args := m.Called(ctx, noteID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*note.NoteMetadataItem), args.Error(1)
}

func (m *MockRepository) SetMetadata(ctx context.Context, noteID, key string, value *string) error {
	args := m.Called(ctx, noteID, key, value)
	return args.Error(0)
}

func (m *MockRepository) DeleteMetadata(ctx context.Context, noteID, key string) error {
	args := m.Called(ctx, noteID, key)
	return args.Error(0)
}

func (m *MockRepository) BulkSetMetadata(ctx context.Context, noteID string, metadata []note.SetNoteMetadataInput) error {
	args := m.Called(ctx, noteID, metadata)
	return args.Error(0)
}

// ========== Mock Storage Client ==========

// Compile-time check that MockStorageClient implements StorageClient
var _ StorageClient = (*MockStorageClient)(nil)

// MockStorageClient is a mock implementation of the StorageClient interface
type MockStorageClient struct {
	mock.Mock
}

func (m *MockStorageClient) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) error {
	args := m.Called(ctx, key, reader, size, contentType)
	return args.Error(0)
}

func (m *MockStorageClient) GetPresignedUploadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	args := m.Called(ctx, key, expiry)
	return args.String(0), args.Error(1)
}

func (m *MockStorageClient) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	args := m.Called(ctx, key, expiry)
	return args.String(0), args.Error(1)
}

func (m *MockStorageClient) Delete(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

// ========== Helper Functions ==========

// newTestService creates a service with a mock repo and pre-loaded type cache
func newTestService(mockRepo *MockRepository) *Service {
	svc := &Service{
		repo:      mockRepo,
		storage:   nil,
		typeCache: make(map[string]*note.NoteTypeConfig),
	}
	// Pre-load the type cache with common test types
	svc.typeCache["diary"] = &note.NoteTypeConfig{
		ID:       "type-diary",
		Slug:     "diary",
		IsActive: true,
		Constraints: note.TypeConstraints{
			TitleRequired:   true,
			DateRequired:    true,
			ContentRequired: false,
		},
	}
	svc.typeCache["learning"] = &note.NoteTypeConfig{
		ID:       "type-learning",
		Slug:     "learning",
		IsActive: true,
		Constraints: note.TypeConstraints{
			TitleRequired:   true,
			DateRequired:    false,
			ContentRequired: false,
		},
	}
	return svc
}

func strPtr(s string) *string {
	return &s
}

// ========== CreateNote Tests ==========

func TestCreateNote_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary,
		Title:   strPtr("My Diary Entry"),
		Content: "Today was a good day.",
		Format:  note.NoteFormatMarkdown,
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*note.Note")).Return(nil)

	result, err := svc.Create(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, note.NoteTypeDiary, result.Type)
	assert.Equal(t, "My Diary Entry", *result.Title)
	assert.Equal(t, "Today was a good day.", result.Content)
	assert.Equal(t, note.NoteFormatMarkdown, result.Format)
	assert.False(t, result.Archived)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_EmptyTitle(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary,
		Title:   strPtr(""),
		Content: "Some content",
		Format:  note.NoteFormatMarkdown,
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTitleRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_NilTitleWhenRequired(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary, // diary requires title
		Title:   nil,
		Content: "Some content",
		Format:  note.NoteFormatMarkdown,
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTitleRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_InvalidType(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteType("invalid_type"),
		Title:   strPtr("Test"),
		Content: "Some content",
		Format:  note.NoteFormatMarkdown,
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidType, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_EmptyType(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    "",
		Title:   strPtr("Test"),
		Content: "Some content",
		Format:  note.NoteFormatMarkdown,
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTypeRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_EmptyFormat(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Test"),
		Content: "Some content",
		Format:  "",
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrFormatRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_InvalidFormat(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Test"),
		Content: "Some content",
		Format:  note.NoteFormat("invalid"),
	}

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFormat, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_TitleTrimmed(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary,
		Title:   strPtr("  My Title  "),
		Content: "Some content",
		Format:  note.NoteFormatMarkdown,
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*note.Note")).Return(nil)

	result, err := svc.Create(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "My Title", *result.Title)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_WithMetadata(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	value := "5"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeLearning,
		Title:   strPtr("Learning Entry"),
		Content: "Learned Go testing",
		Format:  note.NoteFormatMarkdown,
		Metadata: []note.SetNoteMetadataInput{
			{Key: "difficulty", Value: &value},
		},
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*note.Note")).Return(nil)

	result, err := svc.Create(ctx, userID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Metadata, 1)
	assert.Equal(t, "difficulty", result.Metadata[0].Key)
	assert.Equal(t, "5", *result.Metadata[0].Value)
	mockRepo.AssertExpectations(t)
}

func TestCreateNote_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	input := &note.CreateNoteInput{
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Test"),
		Content: "Some content",
		Format:  note.NoteFormatMarkdown,
	}

	mockRepo.On("Create", ctx, mock.AnythingOfType("*note.Note")).Return(errors.New("database error"))

	result, err := svc.Create(ctx, userID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== GetByID Tests ==========

func TestGetByID_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	expectedNote := &note.Note{
		ID:      noteID,
		UserID:  userID,
		Type:    note.NoteTypeDiary,
		Title:   strPtr("My Note"),
		Content: "Note content",
		Format:  note.NoteFormatMarkdown,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(expectedNote, nil)

	result, err := svc.GetByID(ctx, userID, noteID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, noteID, result.ID)
	assert.Equal(t, userID, result.UserID)
	assert.Equal(t, "My Note", *result.Title)
	mockRepo.AssertExpectations(t)
}

func TestGetByID_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.GetByID(ctx, userID, noteID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetByID_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, errors.New("database error"))

	result, err := svc.GetByID(ctx, userID, noteID)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== UpdateNote Tests ==========

func TestUpdateNote_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	newTitle := "Updated Title"
	newContent := "Updated content"

	existingNote := &note.Note{
		ID:      noteID,
		UserID:  userID,
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Old Title"),
		Content: "Old content",
		Format:  note.NoteFormatMarkdown,
	}

	input := &note.UpdateNoteInput{
		Title:   &newTitle,
		Content: &newContent,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*note.Note")).Return(nil)

	result, err := svc.Update(ctx, userID, noteID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "Updated Title", *result.Title)
	assert.Equal(t, "Updated content", result.Content)
	mockRepo.AssertExpectations(t)
}

func TestUpdateNote_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"
	newTitle := "Updated Title"

	input := &note.UpdateNoteInput{
		Title: &newTitle,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.Update(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdateNote_EmptyTitle(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	emptyTitle := ""

	existingNote := &note.Note{
		ID:      noteID,
		UserID:  userID,
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Old Title"),
		Content: "Old content",
		Format:  note.NoteFormatMarkdown,
	}

	input := &note.UpdateNoteInput{
		Title: &emptyTitle,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)

	result, err := svc.Update(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTitleRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdateNote_WhitespaceOnlyTitle(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	wsTitle := "   "

	existingNote := &note.Note{
		ID:      noteID,
		UserID:  userID,
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Old Title"),
		Content: "Old content",
		Format:  note.NoteFormatMarkdown,
	}

	input := &note.UpdateNoteInput{
		Title: &wsTitle,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)

	result, err := svc.Update(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrTitleRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdateNote_InvalidFormat(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	invalidFormat := note.NoteFormat("invalid")

	existingNote := &note.Note{
		ID:      noteID,
		UserID:  userID,
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Title"),
		Content: "content",
		Format:  note.NoteFormatMarkdown,
	}

	input := &note.UpdateNoteInput{
		Format: &invalidFormat,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)

	result, err := svc.Update(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidFormat, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestUpdateNote_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	newTitle := "Updated Title"

	existingNote := &note.Note{
		ID:      noteID,
		UserID:  userID,
		Type:    note.NoteTypeDiary,
		Title:   strPtr("Old Title"),
		Content: "Old content",
		Format:  note.NoteFormatMarkdown,
	}

	input := &note.UpdateNoteInput{
		Title: &newTitle,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*note.Note")).Return(errors.New("database error"))

	result, err := svc.Update(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== DeleteNote Tests ==========

func TestDeleteNote_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	mockRepo.On("DeleteByIDAndUserID", ctx, noteID, userID).Return(nil)

	err := svc.Delete(ctx, userID, noteID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteNote_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"

	mockRepo.On("DeleteByIDAndUserID", ctx, noteID, userID).Return(repository.ErrNoteNotFound)

	err := svc.Delete(ctx, userID, noteID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	mockRepo.AssertExpectations(t)
}

func TestDeleteNote_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	mockRepo.On("DeleteByIDAndUserID", ctx, noteID, userID).Return(errors.New("database error"))

	err := svc.Delete(ctx, userID, noteID)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// ========== ListNotes Tests ==========

func TestListNotes_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := &note.NoteFilter{}

	expectedNotes := []*note.NoteListItem{
		{
			ID:     "note-1",
			UserID: userID,
			Type:   note.NoteTypeDiary,
			Title:  strPtr("Note 1"),
			Format: note.NoteFormatMarkdown,
		},
		{
			ID:     "note-2",
			UserID: userID,
			Type:   note.NoteTypeLearning,
			Title:  strPtr("Note 2"),
			Format: note.NoteFormatMarkdown,
		},
	}

	mockRepo.On("List", ctx, userID, filter).Return(expectedNotes, nil)

	result, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "Note 1", *result[0].Title)
	assert.Equal(t, "Note 2", *result[1].Title)
	mockRepo.AssertExpectations(t)
}

func TestListNotes_WithTypeFilter(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := &note.NoteFilter{
		Types: []note.NoteType{note.NoteTypeDiary},
	}

	expectedNotes := []*note.NoteListItem{
		{
			ID:     "note-1",
			UserID: userID,
			Type:   note.NoteTypeDiary,
			Title:  strPtr("Diary Entry"),
			Format: note.NoteFormatMarkdown,
		},
	}

	mockRepo.On("List", ctx, userID, filter).Return(expectedNotes, nil)

	result, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, note.NoteTypeDiary, result[0].Type)
	mockRepo.AssertExpectations(t)
}

func TestListNotes_EmptyResult(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := &note.NoteFilter{}

	mockRepo.On("List", ctx, userID, filter).Return([]*note.NoteListItem{}, nil)

	result, err := svc.List(ctx, userID, filter)

	assert.NoError(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestListNotes_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	filter := &note.NoteFilter{}

	mockRepo.On("List", ctx, userID, filter).Return(nil, errors.New("database error"))

	result, err := svc.List(ctx, userID, filter)

	assert.Error(t, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== Search Tests ==========

func TestSearch_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	query := "Go testing"
	filter := &note.NoteFilter{}
	limit := 20

	expectedResults := []*note.SearchResult{
		{
			Note: &note.NoteListItem{
				ID:     "note-1",
				UserID: userID,
				Title:  strPtr("Go Testing Guide"),
			},
			Score: 0.95,
		},
	}

	mockRepo.On("Search", ctx, userID, query, filter, limit).Return(expectedResults, nil)

	result, err := svc.Search(ctx, userID, query, filter, limit)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Go Testing Guide", *result[0].Note.Title)
	mockRepo.AssertExpectations(t)
}

func TestSearch_EmptyQuery(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"

	result, err := svc.Search(ctx, userID, "", nil, 20)

	assert.Error(t, err)
	assert.Equal(t, ErrQueryRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestSearch_WhitespaceQuery(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"

	result, err := svc.Search(ctx, userID, "   ", nil, 20)

	assert.Error(t, err)
	assert.Equal(t, ErrQueryRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestSearch_DefaultLimit(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	query := "test"

	// When limit <= 0, the service sets it to 20
	mockRepo.On("Search", ctx, userID, query, (*note.NoteFilter)(nil), 20).Return([]*note.SearchResult{}, nil)

	result, err := svc.Search(ctx, userID, query, nil, 0)

	assert.NoError(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestSearch_MaxLimit(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	query := "test"

	// When limit > 100, the service caps it to 100
	mockRepo.On("Search", ctx, userID, query, (*note.NoteFilter)(nil), 100).Return([]*note.SearchResult{}, nil)

	result, err := svc.Search(ctx, userID, query, nil, 200)

	assert.NoError(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== Archive Tests ==========

func TestArchive_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	existingNote := &note.Note{
		ID:       noteID,
		UserID:   userID,
		Type:     note.NoteTypeDiary,
		Title:    strPtr("My Note"),
		Content:  "content",
		Format:   note.NoteFormatMarkdown,
		Archived: false,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("Update", ctx, mock.AnythingOfType("*note.Note")).Return(nil)

	result, err := svc.Archive(ctx, userID, noteID, true)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Archived)
	mockRepo.AssertExpectations(t)
}

func TestArchive_NotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.Archive(ctx, userID, noteID, true)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== LoadNoteTypes Tests ==========

func TestLoadNoteTypes_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &Service{
		repo:      mockRepo,
		storage:   nil,
		typeCache: make(map[string]*note.NoteTypeConfig),
	}

	ctx := context.Background()

	noteTypes := []*note.NoteTypeConfig{
		{ID: "type-1", Slug: "diary", DisplayName: "Diary", IsActive: true},
		{ID: "type-2", Slug: "learning", DisplayName: "Learning", IsActive: true},
		{ID: "type-3", Slug: "deprecated", DisplayName: "Deprecated", IsActive: false},
	}

	mockRepo.On("ListNoteTypes", ctx, false).Return(noteTypes, nil)

	err := svc.LoadNoteTypes(ctx)

	assert.NoError(t, err)
	assert.True(t, svc.IsValidNoteType("diary"))
	assert.True(t, svc.IsValidNoteType("learning"))
	assert.False(t, svc.IsValidNoteType("deprecated")) // inactive
	assert.False(t, svc.IsValidNoteType("nonexistent"))
	mockRepo.AssertExpectations(t)
}

func TestLoadNoteTypes_RepositoryError(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &Service{
		repo:      mockRepo,
		storage:   nil,
		typeCache: make(map[string]*note.NoteTypeConfig),
	}

	ctx := context.Background()

	mockRepo.On("ListNoteTypes", ctx, false).Return(nil, errors.New("database error"))

	err := svc.LoadNoteTypes(ctx)

	assert.Error(t, err)
	mockRepo.AssertExpectations(t)
}

// ========== GetNoteTypes Tests ==========

func TestGetNoteTypes_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()

	types, err := svc.GetNoteTypes(ctx)

	assert.NoError(t, err)
	assert.Len(t, types, 2) // diary + learning from newTestService
	mockRepo.AssertExpectations(t)
}

func TestGetNoteTypes_FilterInactive(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := &Service{
		repo:      mockRepo,
		storage:   nil,
		typeCache: make(map[string]*note.NoteTypeConfig),
	}
	svc.typeCache["active"] = &note.NoteTypeConfig{Slug: "active", IsActive: true}
	svc.typeCache["inactive"] = &note.NoteTypeConfig{Slug: "inactive", IsActive: false}

	ctx := context.Background()

	types, err := svc.GetNoteTypes(ctx)

	assert.NoError(t, err)
	assert.Len(t, types, 1)
	assert.Equal(t, "active", types[0].Slug)
	mockRepo.AssertExpectations(t)
}

// ========== IsValidNoteType Tests ==========

func TestIsValidNoteType(t *testing.T) {
	tests := []struct {
		name     string
		slug     string
		expected bool
	}{
		{"valid diary type", "diary", true},
		{"valid learning type", "learning", true},
		{"invalid type", "nonexistent", false},
		{"empty type", "", false},
	}

	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.IsValidNoteType(tt.slug)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== ListContents Tests ==========

func TestListContents_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	existingNote := &note.Note{
		ID:     noteID,
		UserID: userID,
	}
	expectedContents := []*note.NoteContent{
		{ID: "content-1", NoteID: noteID, ContentType: note.ContentTypeMarkdown, SortOrder: 0},
		{ID: "content-2", NoteID: noteID, ContentType: note.ContentTypeImage, SortOrder: 1},
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("ListContents", ctx, noteID).Return(expectedContents, nil)

	result, err := svc.ListContents(ctx, userID, noteID)

	assert.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, note.ContentTypeMarkdown, result[0].ContentType)
	assert.Equal(t, note.ContentTypeImage, result[1].ContentType)
	mockRepo.AssertExpectations(t)
}

func TestListContents_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.ListContents(ctx, userID, noteID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== GetContent Tests ==========

func TestGetContent_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	expectedContent := &note.NoteContent{
		ID:          contentID,
		NoteID:      noteID,
		ContentType: note.ContentTypeMarkdown,
		Content:     strPtr("# Hello"),
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(expectedContent, nil)

	result, err := svc.GetContent(ctx, userID, noteID, contentID)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, contentID, result.ID)
	assert.Equal(t, noteID, result.NoteID)
	mockRepo.AssertExpectations(t)
}

func TestGetContent_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"
	contentID := "content-123"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.GetContent(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetContent_ContentNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-nonexistent"

	existingNote := &note.Note{ID: noteID, UserID: userID}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(nil, nil)

	result, err := svc.GetContent(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.Equal(t, ErrContentNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetContent_ContentBelongsToDifferentNote(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	// Content belongs to a different note
	wrongContent := &note.NoteContent{
		ID:     contentID,
		NoteID: "note-other",
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(wrongContent, nil)

	result, err := svc.GetContent(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.Equal(t, ErrContentNotFound, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== CreateContent Tests ==========

func TestCreateContent_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	input := &note.CreateNoteContentInput{
		ContentType: note.ContentTypeMarkdown,
		Content:     strPtr("# Hello World"),
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("ListContents", ctx, noteID).Return([]*note.NoteContent{}, nil)
	mockRepo.On("CreateContent", ctx, mock.AnythingOfType("*note.NoteContent")).Return(nil)
	mockRepo.On("UpdateIndexStatus", ctx, noteID, note.IndexStatusPending).Return(nil)

	result, err := svc.CreateContent(ctx, userID, noteID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, noteID, result.NoteID)
	assert.Equal(t, note.ContentTypeMarkdown, result.ContentType)
	assert.Equal(t, 0, result.SortOrder)
	mockRepo.AssertExpectations(t)
}

func TestCreateContent_EmptyContentType(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	input := &note.CreateNoteContentInput{
		ContentType: "",
		Content:     strPtr("content"),
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)

	result, err := svc.CreateContent(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrContentTypeRequired, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateContent_InvalidContentType(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	input := &note.CreateNoteContentInput{
		ContentType: note.ContentType("invalid"),
		Content:     strPtr("content"),
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)

	result, err := svc.CreateContent(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.Equal(t, ErrInvalidContentType, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateContent_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"

	input := &note.CreateNoteContentInput{
		ContentType: note.ContentTypeMarkdown,
		Content:     strPtr("content"),
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.CreateContent(ctx, userID, noteID, input)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestCreateContent_WithSortOrder(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	sortOrder := 5

	existingNote := &note.Note{ID: noteID, UserID: userID}
	input := &note.CreateNoteContentInput{
		ContentType: note.ContentTypeMarkdown,
		Content:     strPtr("content"),
		SortOrder:   &sortOrder,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("CreateContent", ctx, mock.AnythingOfType("*note.NoteContent")).Return(nil)
	mockRepo.On("UpdateIndexStatus", ctx, noteID, note.IndexStatusPending).Return(nil)

	result, err := svc.CreateContent(ctx, userID, noteID, input)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 5, result.SortOrder)
	mockRepo.AssertExpectations(t)
}

// ========== DeleteContent Tests ==========

func TestDeleteContent_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	existingContent := &note.NoteContent{
		ID:          contentID,
		NoteID:      noteID,
		ContentType: note.ContentTypeMarkdown,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(existingContent, nil)
	mockRepo.On("DeleteContent", ctx, contentID).Return(nil)
	mockRepo.On("UpdateIndexStatus", ctx, noteID, note.IndexStatusPending).Return(nil)

	err := svc.DeleteContent(ctx, userID, noteID, contentID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteContent_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"
	contentID := "content-123"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	err := svc.DeleteContent(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	mockRepo.AssertExpectations(t)
}

func TestDeleteContent_ContentNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-nonexistent"

	existingNote := &note.Note{ID: noteID, UserID: userID}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(nil, nil)

	err := svc.DeleteContent(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.Equal(t, ErrContentNotFound, err)
	mockRepo.AssertExpectations(t)
}

func TestDeleteContent_WithExternalStorage(t *testing.T) {
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageClient)
	svc := newTestService(mockRepo)
	svc.storage = mockStorage

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"
	storageKey := "notes/note-123/content-123.png"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	existingContent := &note.NoteContent{
		ID:          contentID,
		NoteID:      noteID,
		ContentType: note.ContentTypeImage,
		StorageKey:  &storageKey,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(existingContent, nil)
	mockStorage.On("Delete", ctx, storageKey).Return(nil)
	mockRepo.On("DeleteContent", ctx, contentID).Return(nil)
	mockRepo.On("UpdateIndexStatus", ctx, noteID, note.IndexStatusPending).Return(nil)

	err := svc.DeleteContent(ctx, userID, noteID, contentID)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

// ========== ReorderContents Tests ==========

func TestReorderContents_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentIDs := []string{"content-2", "content-1", "content-3"}

	existingNote := &note.Note{ID: noteID, UserID: userID}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("ReorderContents", ctx, noteID, contentIDs).Return(nil)

	err := svc.ReorderContents(ctx, userID, noteID, contentIDs)

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestReorderContents_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo)

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"
	contentIDs := []string{"content-1"}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	err := svc.ReorderContents(ctx, userID, noteID, contentIDs)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	mockRepo.AssertExpectations(t)
}

// ========== GetUploadURL Tests ==========

func TestGetUploadURL_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageClient)
	svc := newTestService(mockRepo)
	svc.storage = mockStorage

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	req := &note.UploadURLRequest{
		FileName: "diagram.png",
		MimeType: "image/png",
		FileSize: 1024,
	}

	existingNote := &note.Note{ID: noteID, UserID: userID}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockStorage.On("GetPresignedUploadURL", ctx, mock.AnythingOfType("string"), 15*time.Minute).Return("https://storage.example.com/upload?token=abc", nil)

	result, err := svc.GetUploadURL(ctx, userID, noteID, req)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.UploadURL)
	assert.NotEmpty(t, result.ContentID)
	assert.NotEmpty(t, result.StorageKey)
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestGetUploadURL_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageClient)
	svc := newTestService(mockRepo)
	svc.storage = mockStorage

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"
	req := &note.UploadURLRequest{
		FileName: "diagram.png",
		MimeType: "image/png",
		FileSize: 1024,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.GetUploadURL(ctx, userID, noteID, req)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetUploadURL_StorageUnavailable(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo) // no storage configured

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	req := &note.UploadURLRequest{
		FileName: "diagram.png",
		MimeType: "image/png",
		FileSize: 1024,
	}

	existingNote := &note.Note{ID: noteID, UserID: userID}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)

	result, err := svc.GetUploadURL(ctx, userID, noteID, req)

	assert.Error(t, err)
	assert.Equal(t, ErrStorageUnavailable, err)
	assert.Nil(t, result)
	mockRepo.AssertExpectations(t)
}

// ========== GetDownloadURL Tests ==========

func TestGetDownloadURL_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageClient)
	svc := newTestService(mockRepo)
	svc.storage = mockStorage

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"
	storageKey := "notes/note-123/content-123.png"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	existingContent := &note.NoteContent{
		ID:          contentID,
		NoteID:      noteID,
		ContentType: note.ContentTypeImage,
		StorageKey:  &storageKey,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(existingContent, nil)
	mockStorage.On("GetPresignedDownloadURL", ctx, storageKey, time.Hour).Return("https://storage.example.com/download?token=abc", nil)

	result, err := svc.GetDownloadURL(ctx, userID, noteID, contentID)

	assert.NoError(t, err)
	assert.Equal(t, "https://storage.example.com/download?token=abc", result)
	mockRepo.AssertExpectations(t)
	mockStorage.AssertExpectations(t)
}

func TestGetDownloadURL_NoteNotFound(t *testing.T) {
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageClient)
	svc := newTestService(mockRepo)
	svc.storage = mockStorage

	ctx := context.Background()
	userID := "user-123"
	noteID := "nonexistent"
	contentID := "content-123"

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(nil, repository.ErrNoteNotFound)

	result, err := svc.GetDownloadURL(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrNoteNotFound))
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetDownloadURL_ContentNotInStorage(t *testing.T) {
	mockRepo := new(MockRepository)
	mockStorage := new(MockStorageClient)
	svc := newTestService(mockRepo)
	svc.storage = mockStorage

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	// Content with no storage key (inline only)
	existingContent := &note.NoteContent{
		ID:          contentID,
		NoteID:      noteID,
		ContentType: note.ContentTypeMarkdown,
		Content:     strPtr("inline content"),
		StorageKey:  nil,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(existingContent, nil)

	result, err := svc.GetDownloadURL(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}

func TestGetDownloadURL_StorageUnavailable(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := newTestService(mockRepo) // no storage configured

	ctx := context.Background()
	userID := "user-123"
	noteID := "note-123"
	contentID := "content-123"
	storageKey := "notes/note-123/content-123.png"

	existingNote := &note.Note{ID: noteID, UserID: userID}
	existingContent := &note.NoteContent{
		ID:          contentID,
		NoteID:      noteID,
		ContentType: note.ContentTypeImage,
		StorageKey:  &storageKey,
	}

	mockRepo.On("GetByIDAndUserID", ctx, noteID, userID).Return(existingNote, nil)
	mockRepo.On("GetContent", ctx, contentID).Return(existingContent, nil)

	result, err := svc.GetDownloadURL(ctx, userID, noteID, contentID)

	assert.Error(t, err)
	assert.Equal(t, ErrStorageUnavailable, err)
	assert.Empty(t, result)
	mockRepo.AssertExpectations(t)
}
