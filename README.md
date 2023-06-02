# test-task

## General information

Database used: PostgreSQL

Schema definitions are in schema.sql file.

## Requirements

- Docker
- GNU Make

## Endpoints

| Name                | Path            | Description                                                                | Parameters                                                                 |
| ------------------- | --------------- | -------------------------------------------------------------------------- | -------------------------------------------------------------------------- |
| HandleLogin         | /login          | Returns an acces token given the<br />parameters are found in the database | JSON:<br />{"username": "test",<br /> "password": 123456<br />}            |
| HandleUploadPicture | /upload-picture | Returns an URL for the uploaded image                                      | Bearer token, Expects a file from the an `<form>` tag. The key is "image". |
| HandleGetAllImages  | /images         | Returns all the images for the given AccessToken                           | Bearer token.                                                              |
| HandleGetImage      | /?id=           | Returns an image by ID                                                     | Bearer token, URL Query Parameter for image id ("?id=")"")                 |

## How to run tests

1. Start the docker image with the database: `make start_db`
2. Run the tests: `make test`
