package drive

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/yu-min3/kensan-lab/apps/feed/internal/briefing"
	gdrive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

const driveReadOnlyScope = "https://www.googleapis.com/auth/drive.readonly"

type Inbox struct {
	service  *gdrive.Service
	folderID string
}

func New(ctx context.Context, folderID, credentialsFile string) (*Inbox, error) {
	if err := validateFolderID(folderID); err != nil {
		return nil, err
	}
	service, err := gdrive.NewService(
		ctx,
		option.WithCredentialsFile(credentialsFile),
		option.WithScopes(driveReadOnlyScope),
	)
	if err != nil {
		return nil, fmt.Errorf("create Drive service: %w", err)
	}
	return NewWithService(service, folderID)
}

func NewWithService(service *gdrive.Service, folderID string) (*Inbox, error) {
	if service == nil {
		return nil, fmt.Errorf("Drive service is required")
	}
	if err := validateFolderID(folderID); err != nil {
		return nil, err
	}
	return &Inbox{service: service, folderID: folderID}, nil
}

func validateFolderID(folderID string) error {
	if strings.TrimSpace(folderID) == "" {
		return fmt.Errorf("Drive output folder ID is required")
	}
	if strings.ContainsAny(folderID, "'\\") {
		return fmt.Errorf("invalid Drive output folder ID")
	}
	return nil
}

func (i *Inbox) Fetch(ctx context.Context, date time.Time) (briefing.Report, error) {
	name := date.Format("2006-01-02") + ".md"
	query := fmt.Sprintf(
		"'%s' in parents and name = '%s' and trashed = false",
		i.folderID,
		name,
	)
	var candidates []*gdrive.File
	pageToken := ""
	for {
		call := i.service.Files.List().
			Context(ctx).
			Q(query).
			Spaces("drive").
			PageSize(100).
			Fields("nextPageToken,files(id,name,mimeType,modifiedTime,size)")
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		result, err := call.Do()
		if err != nil {
			return briefing.Report{}, fmt.Errorf("search Drive report: %w", err)
		}
		for _, file := range result.Files {
			if file.Name == name && allowedMIMEType(file.MimeType) {
				candidates = append(candidates, file)
			}
		}
		if result.NextPageToken == "" {
			break
		}
		pageToken = result.NextPageToken
	}
	if len(candidates) == 0 {
		return briefing.Report{}, briefing.ErrReportNotFound
	}

	latest, latestAt, err := selectLatest(candidates)
	if err != nil {
		return briefing.Report{}, err
	}
	response, err := i.service.Files.Get(latest.Id).Context(ctx).Download()
	if err != nil {
		return briefing.Report{}, fmt.Errorf("download Drive report: %w", err)
	}
	defer response.Body.Close()
	content, err := io.ReadAll(io.LimitReader(response.Body, briefing.MaxReportSize+1))
	if err != nil {
		return briefing.Report{}, fmt.Errorf("read Drive report: %w", err)
	}
	if len(content) > briefing.MaxReportSize {
		return briefing.Report{}, fmt.Errorf("Drive report exceeds %d bytes", briefing.MaxReportSize)
	}
	return briefing.Report{
		ExternalID: latest.Id,
		Name:       latest.Name,
		MIMEType:   latest.MimeType,
		ModifiedAt: latestAt,
		Content:    content,
	}, nil
}

func selectLatest(files []*gdrive.File) (*gdrive.File, time.Time, error) {
	var latest *gdrive.File
	var latestAt time.Time
	tied := false
	for _, file := range files {
		modifiedAt, err := time.Parse(time.RFC3339, file.ModifiedTime)
		if err != nil {
			return nil, time.Time{}, fmt.Errorf("invalid Drive modifiedTime for %s: %w", file.Id, err)
		}
		switch {
		case latest == nil || modifiedAt.After(latestAt):
			latest, latestAt, tied = file, modifiedAt, false
		case modifiedAt.Equal(latestAt):
			tied = true
		}
	}
	if tied {
		return nil, time.Time{}, briefing.ErrReportAmbiguous
	}
	return latest, latestAt, nil
}

func allowedMIMEType(value string) bool {
	return value == "text/markdown" || value == "text/plain"
}
