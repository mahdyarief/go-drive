package handler

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-drive/server/internal/storage"
)

// ServeFile handles GET /api/files/serve/*path?exp=&sig= — an HMAC-signed
// local-file stream with range-request support (http.ServeContent).
func ServeFile() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Param("path")
		exp := c.Query("exp")
		sig := c.Query("sig")

		if path == "" || exp == "" || sig == "" {
			Err(c, http.StatusForbidden, "missing signature")
			return
		}
		expInt, err := strconv.ParseInt(exp, 10, 64)
		if err != nil {
			Err(c, http.StatusForbidden, "bad expiry")
			return
		}
		if time.Now().Unix() > expInt {
			Err(c, http.StatusGone, "link expired")
			return
		}

		key := storage.SignKey()
		mac := hmac.New(sha256.New, key)
		mac.Write([]byte(path + ":" + exp))
		expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(expected), []byte(sig)) {
			Err(c, http.StatusForbidden, "invalid signature")
			return
		}

		baseDir := localBaseDir()
		full := filepath.Join(baseDir, filepath.Clean("/"+path))
		if full != baseDir && !strings.HasPrefix(full, baseDir+string(os.PathSeparator)) {
			Err(c, http.StatusForbidden, "invalid path")
			return
		}
		f, err := os.Open(full)
		if err != nil {
			if os.IsNotExist(err) {
				Err(c, http.StatusNotFound, "file not found")
				return
			}
			Err(c, http.StatusInternalServerError, "opening file")
			return
		}
		defer f.Close()

		st, err := f.Stat()
		if err != nil {
			Err(c, http.StatusInternalServerError, "stat file")
			return
		}

		c.Header("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
		c.Header("Cache-Control", "private, max-age=3600")
		http.ServeContent(c.Writer, c.Request, filepath.Base(path), st.ModTime(), f)
	}
}

// localBaseDir mirrors the factory's local blob root resolution so the serve
// handler can resolve object paths without building a provider.
func localBaseDir() string {
	if d := os.Getenv("LOCAL_BLOB_DIR"); d != "" {
		abs, err := filepath.Abs(d)
		if err == nil {
			return abs
		}
		return d
	}
	abs, _ := filepath.Abs("./data/blobs")
	return abs
}
