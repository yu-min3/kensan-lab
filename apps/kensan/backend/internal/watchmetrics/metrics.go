// Package watchmetrics records public publication/development observations.
// Serving a dashboard only reads the snapshot; it never calls a provider.
package watchmetrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"syscall"
	"time"
)

const Path = "feeds/watch-metrics/latest.json"

var JST = time.FixedZone("JST", 9*60*60)

type Article struct {
	Title string    `json:"title"`
	URL   string    `json:"url"`
	At    time.Time `json:"at"`
}
type Pull struct {
	Title     string    `json:"title"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
	MergedAt  time.Time `json:"mergedAt"`
}
type Publications struct {
	Month string    `json:"month"`
	Items []Article `json:"items"`
}
type Development struct {
	Week        string   `json:"week"`
	Items       []Pull   `json:"items"`
	Days        [7]int   `json:"days"`
	MedianHours *float64 `json:"medianHours"`
}
type Section[T any] struct {
	Data          *T        `json:"data"`
	UpdatedAt     time.Time `json:"updatedAt"`
	CheckedAt     time.Time `json:"checkedAt"`
	Error         string    `json:"error,omitempty"`
	Stale         bool      `json:"stale"`
	CurrentPeriod bool      `json:"currentPeriod"`
}
type View struct {
	Version      int                   `json:"version"`
	Repo         string                `json:"repo"`
	Zenn         string                `json:"zenn"`
	Publications Section[Publications] `json:"publications"`
	Development  Section[Development]  `json:"development"`
}
type Collector struct {
	Client               *http.Client
	GitHubBase, ZennBase string
}

func NewCollector() Collector {
	return Collector{Client: &http.Client{Timeout: 20 * time.Second}, GitHubBase: "https://api.github.com", ZennBase: "https://zenn.dev"}
}
func Periods(now time.Time) (time.Time, time.Time) {
	n := now.In(JST)
	day := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, JST)
	return time.Date(n.Year(), n.Month(), 1, 0, 0, 0, 0, JST), day.AddDate(0, 0, -(int(n.Weekday())+6)%7)
}
func (c Collector) get(ctx context.Context, url string, v any) error {
	req, e := http.NewRequestWithContext(ctx, "GET", url, nil)
	if e != nil {
		return e
	}
	req.Header.Set("User-Agent", "kensan-watch-metrics")
	req.Header.Set("Accept", "application/json")
	resp, e := c.Client.Do(req)
	if e != nil {
		return fmt.Errorf("provider unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("provider HTTP %d", resp.StatusCode)
	}
	b, e := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024+1))
	if e != nil || len(b) > 4*1024*1024 {
		return fmt.Errorf("invalid provider response")
	}
	if json.Unmarshal(b, v) != nil {
		return fmt.Errorf("invalid provider JSON")
	}
	return nil
}
func (c Collector) publications(ctx context.Context, user string, now time.Time) (*Publications, error) {
	month, _ := Periods(now)
	out := &Publications{Month: month.Format("2006-01"), Items: []Article{}}
	seen := map[int]bool{}
	for page := 1; page <= 20; page++ {
		var data struct {
			Articles []struct {
				ID    int       `json:"id"`
				Title string    `json:"title"`
				At    time.Time `json:"published_at"`
				Path  string    `json:"path"`
			} `json:"articles"`
			Next  json.RawMessage `json:"next_page"`
			Total *int            `json:"total_count"`
		}
		e := c.get(ctx, fmt.Sprintf("%s/api/articles?username=%s&order=latest&page=%d&count=100", c.ZennBase, user, page), &data)
		if e != nil {
			return nil, e
		}
		if data.Articles == nil || len(data.Next) == 0 {
			return nil, fmt.Errorf("incomplete article response")
		}
		for _, a := range data.Articles {
			if a.ID <= 0 || a.At.IsZero() || a.Title == "" || !strings.HasPrefix(a.Path, "/"+user+"/articles/") {
				return nil, fmt.Errorf("invalid article")
			}
			if seen[a.ID] {
				continue
			}
			seen[a.ID] = true
			if !a.At.Before(month) && !a.At.After(now) {
				out.Items = append(out.Items, Article{a.Title, "https://zenn.dev" + a.Path, a.At})
			}
		}
		if string(data.Next) == "null" {
			if data.Total != nil && len(seen) != *data.Total {
				return nil, fmt.Errorf("incomplete article pagination")
			}
			return out, nil
		}
		var next int
		if json.Unmarshal(data.Next, &next) != nil || next != page+1 {
			return nil, fmt.Errorf("invalid article pagination")
		}
	}
	return nil, fmt.Errorf("article pagination limit")
}
func (c Collector) development(ctx context.Context, repo string, now time.Time) (*Development, error) {
	_, week := Periods(now)
	out := &Development{Week: week.Format("2006-01-02"), Items: []Pull{}}
	seen := map[int]bool{}
	hours := []float64{}
	for page := 1; page <= 20; page++ {
		var data []struct {
			Number  int        `json:"number"`
			Title   string     `json:"title"`
			URL     string     `json:"html_url"`
			Created time.Time  `json:"created_at"`
			Updated time.Time  `json:"updated_at"`
			Merged  *time.Time `json:"merged_at"`
		}
		e := c.get(ctx, fmt.Sprintf("%s/repos/%s/pulls?state=closed&sort=updated&direction=desc&per_page=100&page=%d", c.GitHubBase, repo, page), &data)
		if e != nil {
			return nil, e
		}
		if data == nil {
			return nil, fmt.Errorf("invalid pull response")
		}
		old := false
		for _, p := range data {
			if p.Number <= 0 || p.Updated.IsZero() || p.Created.IsZero() {
				return nil, fmt.Errorf("invalid pull dates")
			}
			if p.Updated.Before(week) {
				old = true
			}
			if p.Merged == nil || p.Merged.Before(week) || p.Merged.After(now) || seen[p.Number] {
				continue
			}
			if p.Merged.Before(p.Created) || !strings.HasPrefix(p.URL, "https://github.com/"+repo+"/pull/") {
				return nil, fmt.Errorf("invalid merged pull")
			}
			seen[p.Number] = true
			out.Items = append(out.Items, Pull{p.Title, p.URL, p.Created, *p.Merged})
			i := int(p.Merged.In(JST).Sub(week).Hours() / 24)
			if i >= 0 && i < 7 {
				out.Days[i]++
			}
			hours = append(hours, p.Merged.Sub(p.Created).Hours())
		}
		if len(data) < 100 || old {
			sort.Float64s(hours)
			if n := len(hours); n > 0 {
				m := hours[n/2]
				if n%2 == 0 {
					m = (hours[n/2-1] + m) / 2
				}
				out.MedianHours = &m
			}
			sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].MergedAt.After(out.Items[j].MergedAt) })
			return out, nil
		}
	}
	return nil, fmt.Errorf("pull pagination limit")
}
func read(r *os.Root) (View, error) {
	var v View
	f, e := r.Open(Path)
	if e != nil {
		return v, e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 4*1024*1024+1))
	if e != nil || len(b) > 4*1024*1024 {
		return v, fmt.Errorf("invalid snapshot")
	}
	if e = json.Unmarshal(b, &v); e != nil || v.Version != 1 {
		return v, fmt.Errorf("invalid snapshot")
	}
	return v, nil
}
func Read(root string, now time.Time) (View, error) {
	r, e := os.OpenRoot(root)
	if e != nil {
		return View{}, e
	}
	defer r.Close()
	v, e := read(r)
	if os.IsNotExist(e) {
		return View{}, nil
	}
	if e != nil {
		return View{}, e
	}
	month, week := Periods(now)
	v.Publications.CurrentPeriod = v.Publications.Data != nil && v.Publications.Data.Month == month.Format("2006-01")
	v.Development.CurrentPeriod = v.Development.Data != nil && v.Development.Data.Week == week.Format("2006-01-02")
	v.Publications.Stale = v.Publications.Error != "" || now.Sub(v.Publications.UpdatedAt) > 3*time.Hour || v.Publications.UpdatedAt.After(now.Add(time.Minute))
	v.Development.Stale = v.Development.Error != "" || now.Sub(v.Development.UpdatedAt) > 3*time.Hour || v.Development.UpdatedAt.After(now.Add(time.Minute))
	return v, nil
}

// Refresh preserves each last successful section independently on failures.
func (c Collector) Refresh(ctx context.Context, root, repo, user string, now time.Time) error {
	valid := func(s string) bool {
		return s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_") == ""
	}
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || !valid(parts[0]) || !valid(parts[1]) || !valid(user) {
		return fmt.Errorf("invalid public source")
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return e
	}
	defer r.Close()
	if e = r.MkdirAll("feeds/state", 0700); e != nil {
		return e
	}
	if e = r.MkdirAll("feeds/watch-metrics", 0700); e != nil {
		return e
	}
	lock, e := r.OpenFile("feeds/state/watch-metrics.lock", os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fmt.Errorf("metrics collector already running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	v, e := read(r)
	if e != nil && !os.IsNotExist(e) {
		return e
	}
	if v.Repo != repo || v.Zenn != user {
		v = View{Version: 1, Repo: repo, Zenn: user}
	}
	v.Publications.CheckedAt = now
	v.Development.CheckedAt = now
	failed := false
	if data, e := c.publications(ctx, user, now); e != nil {
		v.Publications.Error = "公開記事の取得に失敗"
		failed = true
	} else {
		v.Publications.Data = data
		v.Publications.UpdatedAt = now
		v.Publications.Error = ""
	}
	if data, e := c.development(ctx, repo, now); e != nil {
		v.Development.Error = "開発指標の取得に失敗"
		failed = true
	} else {
		v.Development.Data = data
		v.Development.UpdatedAt = now
		v.Development.Error = ""
	}
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		return e
	}
	tmp := Path + fmt.Sprintf(".tmp-%d", now.UnixNano())
	f, e := r.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return e
	}
	defer r.Remove(tmp)
	_, e = f.Write(append(b, '\n'))
	if e == nil {
		e = f.Sync()
	}
	ce := f.Close()
	if e != nil {
		return e
	}
	if ce != nil {
		return ce
	}
	if e = r.Rename(tmp, Path); e != nil {
		return e
	}
	if failed {
		return fmt.Errorf("metrics partially unavailable; previous observations preserved")
	}
	return nil
}
