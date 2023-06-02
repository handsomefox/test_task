package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	_ "embed"

	"golang.org/x/exp/slog"
)

//go:embed schema.sql
var schema string

type PostgreSQLDatabase struct {
	db *sql.DB
}

func NewPostgreSQLDatabase() (*PostgreSQLDatabase, error) {
	var (
		user     = os.Getenv("POSTGRES_USER")
		password = os.Getenv("POSTGRES_PASSWORD")
		port     = os.Getenv("DB_PORT")
		connStr  = fmt.Sprintf("user=%s password=%s port=%s dbname=db sslmode=disable", user, password, port)
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	pg := &PostgreSQLDatabase{db: db}
	if err := pg.db.Ping(); err != nil {
		return nil, err
	}

	slog.Debug("Database pinged")

	if _, err := pg.db.ExecContext(context.Background(), schema); err != nil {
		slog.Debug("Failed to create database schema", "error", err)
	} else {
		slog.Info("Successfully created the database schema")
	}

	return pg, nil
}

func (pq *PostgreSQLDatabase) CreateImage(ctx context.Context, userID int32, imagePath, imageURL string) (Image, error) {
	const createImage = `
	INSERT INTO images(user_id, image_path, image_url)
	VALUES($1, $2, $3)
	RETURNING id, user_id, image_path, image_url
	`

	row := pq.db.QueryRowContext(ctx, createImage, userID, imagePath, imageURL)
	var i Image
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.ImagePath,
		&i.ImageURL,
	)

	return i, err
}

func (pq *PostgreSQLDatabase) CreateUser(ctx context.Context, username, passwordHash string) (int64, error) {
	const createUser = `
	INSERT INTO users (username, password_hash)
	VALUES($1, $2)
	RETURNING id
	`

	row := pq.db.QueryRowContext(ctx, createUser, username, passwordHash)
	var id int64
	err := row.Scan(&id)

	return id, err
}

func (pq *PostgreSQLDatabase) DeleteImageByID(ctx context.Context, id int64) error {
	const deleteImageByID = `
	DELETE from images
	WHERE id = $1
	`

	_, err := pq.db.ExecContext(ctx, deleteImageByID, id)

	return err
}

func (pq *PostgreSQLDatabase) GetAllUserImages(ctx context.Context, userID int32) ([]Image, error) {
	const getAllUserImages = `
	SELECT
		id,
    	user_id,
    	image_path,
    	image_url
	FROM images
	WHERE user_id = $1
	`

	rows, err := pq.db.QueryContext(ctx, getAllUserImages, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Image

	for rows.Next() {
		var i Image
		if err := rows.Scan(
			&i.ID,
			&i.UserID,
			&i.ImagePath,
			&i.ImageURL,
		); err != nil {
			return nil, err
		}

		items = append(items, i)
	}

	if err := rows.Close(); err != nil {
		return nil, err
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (pq *PostgreSQLDatabase) GetImageByURL(ctx context.Context, url any) (Image, error) {
	const getImageByID = `
	SELECT
		id,
    	user_id,
    	image_path,
    	image_url
	FROM images
	WHERE image_url = $1
	`

	row := pq.db.QueryRowContext(ctx, getImageByID, url)
	var i Image
	err := row.Scan(
		&i.ID,
		&i.UserID,
		&i.ImagePath,
		&i.ImageURL,
	)

	return i, err
}

func (pq *PostgreSQLDatabase) GetUserByID(ctx context.Context, id any) (User, error) {
	const getUserByID = `
	SELECT
		id,
    	username,
    	password_hash
	FROM users
	WHERE id = $1
	`

	row := pq.db.QueryRowContext(ctx, getUserByID, id)
	var i User
	err := row.Scan(&i.ID, &i.Username, &i.PasswordHash)

	return i, err
}

func (pq *PostgreSQLDatabase) GetUserByNamePassword(ctx context.Context, username string) (User, error) {
	const getUserByID = `
	SELECT
		id,
    	username,
    	password_hash
	FROM users
	WHERE username = $1
	`

	row := pq.db.QueryRowContext(ctx, getUserByID, username)
	var i User
	err := row.Scan(&i.ID, &i.Username, &i.PasswordHash)

	return i, err
}
