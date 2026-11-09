package main

// Mocks are captured responses kept as files and served back by the proxy
// instead of the network: a Map Local rule made from what was on screen.

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/CarlOpenLab/omniflow/internal/model"
	"github.com/CarlOpenLab/omniflow/internal/proxy"
	"github.com/CarlOpenLab/omniflow/locales"
)

// applyMocks hands the enabled rules to the proxy.
func (a *app) applyMocks() {
	if a.prx == nil {
		return
	}
	mocks := make([]proxy.Mock, 0, len(a.mocks))
	for _, r := range a.mocks {
		if !r.Enabled || r.Method == "" || r.Host == "" || r.Path == "" || r.File == "" {
			continue
		}
		mocks = append(mocks, proxy.Mock{
			Method:      r.Method,
			Host:        r.Host,
			Path:        r.Path,
			Status:      r.Status,
			ContentType: r.ContentType,
			File:        r.File,
		})
	}
	a.prx.SetMocks(mocks)
}

func (a *app) persistMocks() {
	if a.store != nil {
		_ = a.store.SetMocks(a.mocks)
	}
}

// mockFlow turns a captured response into a Map Local rule: the body is
// written into a file the person can edit, and the proxy answers the same
// method, host and path from it.
func (a *app) mockFlow(f proxy.Flow) {
	if f.Status == 0 {
		a.setNotice(locales.NoticeMockNeedsResponse())
		return
	}
	if a.store == nil {
		a.setNotice(locales.CommonUnavailable())
		return
	}
	path := f.Path
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if path == "" {
		path = "/"
	}
	name := f.Method + " " + f.Host + path

	for i := range a.mocks {
		r := &a.mocks[i]
		if r.Method != f.Method || !strings.EqualFold(r.Host, f.Host) || r.Path != path {
			continue
		}
		if err := os.WriteFile(r.File, f.RespBody, 0o644); err != nil {
			a.setNotice(locales.NoticeMockWriteFailed(err.Error()))
			return
		}
		r.Enabled, r.Name = true, name
		r.Status, r.ContentType = f.Status, f.ContentType
		a.persistMocks()
		a.applyMocks()
		a.setNotice(mockNotice(name, f.RespTruncated, true))
		return
	}

	dir := filepath.Join(a.dataDir(), "mocks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		a.setNotice(locales.NoticeMockWriteFailed(err.Error()))
		return
	}
	id := model.NewID("mock")
	file := filepath.Join(dir, id+mockFileExt(f.ContentType))
	if err := os.WriteFile(file, f.RespBody, 0o644); err != nil {
		a.setNotice(locales.NoticeMockWriteFailed(err.Error()))
		return
	}
	a.mocks = append([]model.MockRule{{
		ID:          id,
		Enabled:     true,
		Name:        name,
		Method:      f.Method,
		Host:        f.Host,
		Path:        path,
		Status:      f.Status,
		ContentType: f.ContentType,
		File:        file,
		CreatedAt:   time.Now(),
	}}, a.mocks...)
	a.limitMocks()
	a.persistMocks()
	a.applyMocks()
	a.setNotice(mockNotice(name, f.RespTruncated, false))
}

// mockNotice says what the new or updated mock answered, and warns when the
// captured body was cut before the rule was made.
func mockNotice(name string, truncated, updated bool) string {
	if truncated {
		return locales.NoticeMockTruncated()
	}
	if updated {
		return locales.NoticeMockUpdated(name)
	}
	return locales.NoticeMockCreated(name)
}

// limitMocks keeps the list bounded, and takes the oldest rule's file with
// it so the mock folder does not collect orphans.
func (a *app) limitMocks() {
	const maxMocks = 200
	for len(a.mocks) > maxMocks {
		last := a.mocks[len(a.mocks)-1]
		a.mocks = a.mocks[:len(a.mocks)-1]
		if last.File != "" {
			_ = os.Remove(last.File)
		}
	}
}

func (a *app) deleteMock(id string) {
	for i := range a.mocks {
		if a.mocks[i].ID != id {
			continue
		}
		file, name := a.mocks[i].File, a.mocks[i].Name
		a.mocks = append(a.mocks[:i], a.mocks[i+1:]...)
		a.persistMocks()
		a.applyMocks()
		if file != "" {
			_ = os.Remove(file)
		}
		a.setNotice(locales.NoticeMockDeleted(name))
		return
	}
}

func (a *app) setMockEnabled(id string, enabled bool) {
	for i := range a.mocks {
		if a.mocks[i].ID != id {
			continue
		}
		a.mocks[i].Enabled = enabled
		a.persistMocks()
		a.applyMocks()
		return
	}
}

func (a *app) revealMock(r model.MockRule) {
	if err := revealPath(r.File); err != nil {
		a.setNotice(locales.NoticeMockRevealFailed(err.Error()))
	}
}

func (a *app) revealMocksDir() {
	if a.store == nil {
		return
	}
	if err := revealPath(filepath.Join(a.dataDir(), "mocks")); err != nil {
		a.setNotice(locales.NoticeMockRevealFailed(err.Error()))
	}
}

// mockFileExt gives the captured body an extension an editor understands.
func mockFileExt(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "json"):
		return ".json"
	case strings.Contains(ct, "html"):
		return ".html"
	case strings.Contains(ct, "xml"):
		return ".xml"
	case strings.Contains(ct, "javascript"):
		return ".js"
	case strings.Contains(ct, "css"):
		return ".css"
	case strings.Contains(ct, "svg"):
		return ".svg"
	case strings.Contains(ct, "png"):
		return ".png"
	case strings.Contains(ct, "jpeg"), strings.Contains(ct, "jpg"):
		return ".jpg"
	case strings.Contains(ct, "gif"):
		return ".gif"
	case strings.HasPrefix(ct, "text/"):
		return ".txt"
	}
	return ".bin"
}
