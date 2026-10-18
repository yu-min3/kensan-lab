package discovery

import (
	"context"
	"errors"
	"github.com/PuerkitoBio/goquery"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/creatorprefs"
	"github.com/yu-min3/kensan-lab/apps/konro/backend/internal/webpage"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var handlePath = regexp.MustCompile(`^/@[^/?#\s]{1,100}/?$`)
var channelPath = regexp.MustCompile(`^/channel/UC[A-Za-z0-9_-]{22}/?$`)
var chefPath = regexp.MustCompile(`^/chefs/[1-9][0-9]{0,9}/?$`)

func profileTarget(raw string) (string, error) {
	u, err := webpage.NormalizeURL(raw)
	if err != nil {
		return "", err
	}
	if u.Port() != "" {
		return "", creatorprefs.ErrInvalid
	}
	u.Scheme = "https"
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = strings.TrimRight(u.Path, "/")
	switch strings.ToLower(u.Hostname()) {
	case "www.youtube.com", "youtube.com", "m.youtube.com":
		if !handlePath.MatchString(u.Path) && !channelPath.MatchString(u.Path) {
			return "", errors.New("YouTubeのチャンネルURLを入力してください。動画URLでは登録できません")
		}
		u.Host = "www.youtube.com"
	case "www.kurashiru.com", "kurashiru.com":
		if !chefPath.MatchString(u.Path) {
			return "", errors.New("クラシルの料理人プロフィールURLを入力してください")
		}
		u.Host = "www.kurashiru.com"
	default:
		return "", errors.New("現在はYouTubeチャンネルとクラシルの料理人プロフィールに対応しています")
	}
	return u.String(), nil
}
func (s *Service) ResolveCreator(ctx context.Context, raw string) (creatorprefs.Creator, error) {
	target, err := profileTarget(raw)
	if err != nil {
		return creatorprefs.Creator{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	page, err := s.fetchPage(ctx, target)
	if err != nil {
		return creatorprefs.Creator{}, errors.New("作り手を確認できませんでした。公式URLを確認して、もう一度試してください")
	}
	final, err := profileTarget(page.URL)
	if err != nil {
		return creatorprefs.Creator{}, errors.New("公式プロフィールへ接続できませんでした")
	}
	requested, _ := url.Parse(target)
	actual, _ := url.Parse(final)
	if actual.Host != requested.Host {
		return creatorprefs.Creator{}, creatorprefs.ErrInvalid
	}
	if requested.Host == "www.kurashiru.com" {
		if actual.Path != requested.Path {
			return creatorprefs.Creator{}, creatorprefs.ErrInvalid
		}
		doc, err := goquery.NewDocumentFromReader(strings.NewReader(page.HTML))
		if err != nil {
			return creatorprefs.Creator{}, err
		}
		name := strings.Join(strings.Fields(doc.Find(".ChefsShow-name").First().Text()), " ")
		id := "kurashiru:chef:" + strings.TrimPrefix(actual.Path, "/chefs/")
		c := creatorprefs.Creator{ID: id, Name: name, URL: creatorprefs.ProfileURL(id)}
		if !creatorprefs.Valid(c) {
			return c, errors.New("料理人のプロフィールを確認できませんでした")
		}
		return c, nil
	}
	v, ok := webpage.ScriptObject(page.HTML, "ytInitialData")
	if !ok {
		return creatorprefs.Creator{}, errors.New("チャンネル情報を読み取れませんでした")
	}
	metadata, _ := v["metadata"].(map[string]any)
	channel, _ := metadata["channelMetadataRenderer"].(map[string]any)
	id, _ := channel["externalId"].(string)
	name, _ := channel["title"].(string)
	c := creatorprefs.Creator{ID: "youtube:" + id, Name: strings.Join(strings.Fields(name), " "), URL: creatorprefs.ProfileURL("youtube:" + id)}
	if !creatorprefs.Valid(c) {
		return c, errors.New("YouTubeチャンネルを確認できませんでした")
	}
	if strings.HasPrefix(requested.Path, "/channel/") && strings.TrimPrefix(requested.Path, "/channel/") != id {
		return c, creatorprefs.ErrInvalid
	}
	// A channel-ID canonical link must agree with the metadata, if supplied.
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(page.HTML))
	if doc != nil {
		canonical := doc.Find(`link[rel="canonical"]`).AttrOr("href", "")
		if u, e := url.Parse(canonical); e == nil && strings.HasPrefix(u.Path, "/channel/") && strings.TrimPrefix(u.Path, "/channel/") != id {
			return c, creatorprefs.ErrInvalid
		}
	}
	return c, nil
}
