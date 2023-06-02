package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

var (
	testUser     User
	testServer   *httptest.Server
	testToken    *Token
	testPassword = "test-1"
)

func init() {
	if err := godotenv.Load(); err != nil {
		panic(err)
	}

	tmp, err := os.CreateTemp(os.TempDir(), "")
	if err != nil {
		panic(err)
	}
	testUser = newUser(tmp.Name(), "test-1")
	testServer = setupHttptestServer()
	testToken, err = NewJWTAccessToken(testUser)
	if err != nil {
		panic(err)
	}
}

func TestAPIServer_HandleLogin(t *testing.T) {
	b, err := json.Marshal(HandleLoginRequest{Username: testUser.Username, Password: testPassword})
	assert.NoError(t, err)

	resp, err := http.Post(testServer.URL+"/login", "application/json", bytes.NewBuffer(b))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var v HandleLoginResponse
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NoError(t, err)

	_, ok := VerifyJWTToken(v.Token)
	assert.Equal(t, true, ok)
}

func TestAPIServer_HandleUploadPicture(t *testing.T) {
	bf, mw := loadFileForm(t)
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/upload-picture", bf)
	assert.NoError(t, err)

	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+testToken.Access)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var imgResponse HandleUploadPictureResponse
	err = json.NewDecoder(resp.Body).Decode(&imgResponse)
	assert.NoError(t, err)
}

func TestAPIServer_HandleGetAllImagesAndHandleGetImageByID(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/images", http.NoBody)
	assert.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+testToken.Access)

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)

	var images HandleGetAllImagesResponse
	err = json.NewDecoder(resp.Body).Decode(&images)
	assert.NoError(t, err)

	assert.NotEqual(t, 0, len(images.Images))

	// HandleGetImageByID

	id := images.Images[0].ImageURL
	req, err = http.NewRequest(http.MethodGet, testServer.URL+"/?id="+id, http.NoBody)
	assert.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+testToken.Access)

	resp, err = http.DefaultClient.Do(req)
	assert.NoError(t, err)

	b1, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	b2, err := os.ReadFile("./testdata/test.png")
	assert.NoError(t, err)
	assert.Equal(t, b1, b2)
}

func TestAPIServer_HandleLoginUnauthorized(t *testing.T) {
	b, err := json.Marshal(HandleLoginRequest{Username: testUser.Username, Password: "invalid-password"})
	assert.NoError(t, err)

	resp, err := http.Post(testServer.URL+"/login", "application/json", bytes.NewBuffer(b))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var v HandleLoginResponse
	err = json.NewDecoder(resp.Body).Decode(&v)
	assert.NoError(t, err)

	_, ok := VerifyJWTToken(v.Token)
	assert.NotEqual(t, true, ok)
}

func TestAPIServer_HandleUploadPictureUnauthorized(t *testing.T) {
	bf, mw := loadFileForm(t)
	mw.Close()

	req, err := http.NewRequest(http.MethodPost, testServer.URL+"/upload-picture", bf)
	assert.NoError(t, err)

	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+"invalid-token")

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
}

func TestAPIServer_HandleGetAllImagesUnauthorized(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, testServer.URL+"/images", http.NoBody)
	assert.NoError(t, err)

	req.Header.Set("Authorization", "Bearer "+"invalid-token")

	resp, err := http.DefaultClient.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

	var images HandleGetAllImagesResponse
	err = json.NewDecoder(resp.Body).Decode(&images)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(images.Images))
}

func loadFileForm(t *testing.T) (*bytes.Buffer, *multipart.Writer) {
	t.Helper()

	b, err := os.ReadFile("./testdata/test.png")
	assert.NoError(t, err)

	var bf bytes.Buffer
	w := multipart.NewWriter(&bf)

	fw, err := w.CreateFormFile("image", "test.png")
	assert.NoError(t, err)

	_, err = io.Copy(fw, bytes.NewBuffer(b))
	assert.NoError(t, err)

	return &bf, w
}

func newUser(name, password string) User {
	db, err := NewPostgreSQLDatabase()
	if err != nil {
		panic(err)
	}

	h, err := hashPassword(password)
	if err != nil {
		panic(err)
	}
	id, err := db.CreateUser(context.Background(), name, string(h))
	if err != nil {
		panic(err)
	}
	user, err := db.GetUserByID(context.Background(), id)
	if err != nil {
		panic(err)
	}
	return user
}

func setupHttptestServer() *httptest.Server {
	db, err := NewPostgreSQLDatabase()
	if err != nil {
		panic(err)
	}

	s := NewAPIServer(db, "localhost:3000")
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

	return httptest.NewServer(r)
}

func hashPassword(pwd string) ([]byte, error) {
	return bcrypt.GenerateFromPassword([]byte(pwd), 14)
}
