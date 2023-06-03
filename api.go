package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/slog"
)

var ErrUnauthorized = errors.New("api: user unauthorized")

const MaxImageSize = 50 << 20 // 50MB

type APIServer struct {
	db         *PostgreSQLDatabase
	listenAddr string
}

func NewAPIServer(db *PostgreSQLDatabase, listenAddr string) *APIServer {
	return &APIServer{
		db:         db,
		listenAddr: listenAddr,
	}
}

type APIFunc func(w http.ResponseWriter, r *http.Request) error

func makeHandler(f APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := f(w, r)

		var statusError *StatusError
		if errors.As(err, &statusError) {
			slog.Error("Writing API Status Error to response", "status_error", statusError)

			if statusError.Err != nil {
				w.WriteHeader(statusError.Status)
				writeJSON(w, statusError)
			} else {
				http.Error(w, http.StatusText(statusError.Status), statusError.Status)
			}

			return
		}

		if err != nil {
			slog.Error("Writing an error to response", "error", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)

			return
		}
	}
}

type StatusError struct {
	Err    error `json:"error,omitempty"`
	Status int   `json:"status,omitempty"`
}

func (a *StatusError) Error() string {
	if a.Err != nil {
		return a.Err.Error()
	}

	return ""
}

func (s *APIServer) Run() error {
	r := http.NewServeMux()

	r.HandleFunc("/login", makeHandler(s.HandleLogin))
	r.HandleFunc("/upload-picture", makeHandler(
		s.authMiddleware(s.HandleUploadPicture),
	))
	r.HandleFunc("/images", makeHandler(
		s.authMiddleware(s.HandleGetAllImages),
	))
	r.Handle("/", makeHandler(
		s.authMiddleware(s.HandleGetImage),
	))

	srv := http.Server{
		Addr:              s.listenAddr,
		Handler:           r,
		ReadTimeout:       10,
		ReadHeaderTimeout: 10,
		WriteTimeout:      20,
		IdleTimeout:       10,
	}

	slog.Info("Starting the server", "listen_addr", s.listenAddr)

	return srv.ListenAndServe()
}

type HandleLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type HandleLoginResponse struct {
	Token string `json:"token"`
}

func (s *APIServer) HandleLogin(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return &StatusError{Err: nil, Status: http.StatusMethodNotAllowed}
	}

	var resp HandleLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return &StatusError{Err: err, Status: http.StatusBadRequest}
	}

	user, err := s.db.GetUserByNamePassword(r.Context(), resp.Username)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusNotFound}
	}

	ok := verifyPassword(resp.Password, user.PasswordHash)
	if !ok {
		return &StatusError{Err: ErrUnauthorized, Status: http.StatusUnauthorized}
	}

	token, err := NewJWTAccessToken(user)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	return writeJSON(w, HandleLoginResponse{Token: token.Access})
}

type HandleUploadPictureResponse struct {
	URL string `json:"url"`
}

func (s *APIServer) HandleUploadPicture(claims *jwt.RegisteredClaims, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		return &StatusError{Err: nil, Status: http.StatusMethodNotAllowed}
	}

	if err := r.ParseMultipartForm(MaxImageSize); err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	formFile, handler, err := r.FormFile("image")
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusBadRequest}
	}
	defer formFile.Close()

	slog.Debug("Received an image",
		"filename", handler.Filename,
		"size", handler.Size,
		"header", handler.Header,
	)

	id, err := uuid.NewRandom()
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	filename := id.String() + handler.Filename

	fpath, err := filepath.Abs("./saved_images/" + filename)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	f, err := createFile(fpath)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	defer f.Close()

	_, err = io.Copy(f, formFile)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	userID, err := strconv.ParseInt(claims.ID, 10, 32)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	_, err = s.db.CreateImage(r.Context(), int32(userID), filename, id.String())
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	slog.Debug("Saved a file", "filename", filename)

	return writeJSON(w, HandleUploadPictureResponse{URL: s.listenAddr + "/?id=" + id.String()})
}

type HandleGetAllImagesResponse struct {
	Images []Image `json:"images"`
}

func (s *APIServer) HandleGetAllImages(claims *jwt.RegisteredClaims, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return &StatusError{Err: nil, Status: http.StatusMethodNotAllowed}
	}

	userID, err := strconv.ParseInt(claims.ID, 10, 32)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	images, err := s.db.GetAllUserImages(r.Context(), int32(userID))
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	for i := 0; i < len(images); i++ {
		url := s.listenAddr + "/?id=" + images[i].ImageURL
		if !strings.HasPrefix(url, "http://") || !strings.HasPrefix(url, "https://") {
			url = "http://" + url
		}

		images[i].ImageURL = url
	}

	return writeJSON(w, HandleGetAllImagesResponse{Images: images})
}

func (s *APIServer) HandleGetImage(claims *jwt.RegisteredClaims, w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodGet {
		return &StatusError{Err: nil, Status: http.StatusMethodNotAllowed}
	}

	userID, err := strconv.ParseInt(claims.ID, 10, 32)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	id := r.URL.Query().Get("id")

	img, err := s.db.GetImageByURL(r.Context(), id)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	if img.UserID != int32(userID) {
		return &StatusError{Err: ErrUnauthorized, Status: http.StatusUnauthorized}
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	f, err := os.Open("./saved_images/" + img.ImagePath)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}
	defer f.Close()

	_, err = io.Copy(w, f)
	if err != nil {
		return &StatusError{Err: err, Status: http.StatusInternalServerError}
	}

	return nil
}

func writeJSON(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")

	return json.NewEncoder(w).Encode(v)
}

type APIAuthFunc func(claims *jwt.RegisteredClaims, w http.ResponseWriter, r *http.Request) error

func (s *APIServer) authMiddleware(f APIAuthFunc) APIFunc {
	return func(w http.ResponseWriter, r *http.Request) error {
		auth := r.Header.Get("Authorization")

		header := strings.Split(auth, " ")
		if len(header) != 2 {
			return ErrUnauthorized
		}

		claims, ok := VerifyJWTToken(header[1])
		if !ok {
			return &StatusError{Err: ErrUnauthorized, Status: http.StatusUnauthorized}
		}

		return f(claims, w, r)
	}
}

func verifyPassword(pwd, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pwd))
	return err == nil
}

func createFile(p string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(p), 0o770); err != nil {
		return nil, err
	}

	return os.Create(p)
}
