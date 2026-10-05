package publisher

import (
	"context"
	"errors"
	"net/http"
	"regexp"

	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/packageauth"
)

var packageReadPath = regexp.MustCompile(`^/user/packages/container/kensan-lab%2Fcanary(/versions\?per_page=100&page=([1-9]|10))?$`)

func (g GitHub) packageRequest(ctx context.Context, path string, out any) (int, error) {
	if !packageReadPath.MatchString(path) {
		return 0, errors.New("package credential outside fixed read-only target")
	}
	token, err := packageauth.OwnerToken(ctx, g.TokenFile, g.PackageTokenFile, g.Client, g.API)
	if err != nil {
		return 0, err
	}
	return g.requestToken(ctx, token, http.MethodGet, path, nil, out)
}
